package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

var (
	ErrGlobalCampaignLookupEvidenceRequired = errors.New("global campaign lookup requires an evidence reference")
	ErrGlobalCampaignAmbiguousCandidates    = errors.New("global campaign lookup matched multiple campaigns")
	ErrGlobalCampaignRulesetConflict        = errors.New("global campaign ruleset versions conflict")
	ErrGlobalCampaignManagedIdentityInput   = errors.New("global campaign store owns existing campaign identity fields")
)

type globalCampaignAnchor struct {
	Kind string
	Ref  string
}

type GlobalCampaignCandidate struct {
	Campaign          GlobalCampaign
	SharedAnchorCount int64
}

// FindGlobalCampaignCandidates resolves current campaigns that share at least
// one canonical evidence reference with the input. Entity-only hints and
// verdict refs deliberately do not participate in automatic candidate lookup.
func FindGlobalCampaignCandidates(ctx context.Context, db *sql.DB, in GlobalCampaignMaterializerInput) ([]GlobalCampaignCandidate, error) {
	if db == nil {
		return nil, ErrGlobalCampaignStoreUnavailable
	}
	anchors := globalCampaignLookupAnchorsFromInput(in)
	if len(anchors) == 0 {
		return nil, nil
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, fmt.Errorf("begin global campaign candidate lookup: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	candidates, err := findGlobalCampaignCandidatesTx(ctx, tx, anchors)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit global campaign candidate lookup: %w", err)
	}
	return candidates, nil
}

// MaterializeAndPersistGlobalCampaign performs evidence-anchor locking,
// candidate lookup, deterministic merge and immutable persistence in one
// PostgreSQL transaction. Multiple candidate identities fail closed rather
// than being silently collapsed into one campaign.
func MaterializeAndPersistGlobalCampaign(ctx context.Context, db *sql.DB, in GlobalCampaignMaterializerInput) (GlobalCampaignPersistResult, error) {
	if db == nil {
		return GlobalCampaignPersistResult{}, ErrGlobalCampaignStoreUnavailable
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return GlobalCampaignPersistResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := materializeAndPersistGlobalCampaignTx(ctx, tx, in)
	if err != nil {
		return GlobalCampaignPersistResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return GlobalCampaignPersistResult{}, err
	}
	return result, nil
}

// Shared transaction core lets the runtime pin its lease, queue acknowledgement
// and campaign revision to a single commit boundary.
func materializeAndPersistGlobalCampaignTx(ctx context.Context, tx *sql.Tx, in GlobalCampaignMaterializerInput) (GlobalCampaignPersistResult, error) {
	if err := validateGlobalCampaignManagedIdentityInput(in); err != nil {
		return GlobalCampaignPersistResult{}, err
	}
	anchors := globalCampaignLookupAnchorsFromInput(in)
	if len(anchors) == 0 {
		return GlobalCampaignPersistResult{}, ErrGlobalCampaignLookupEvidenceRequired
	}

	if err := lockGlobalCampaignAnchorsTx(ctx, tx, anchors); err != nil {
		return GlobalCampaignPersistResult{}, err
	}
	candidates, err := findGlobalCampaignCandidatesTx(ctx, tx, anchors)
	if err != nil {
		return GlobalCampaignPersistResult{}, err
	}

	var result GlobalCampaignPersistResult
	switch len(candidates) {
	case 0:
		campaign := MaterializeGlobalCampaign(in)
		result, err = persistGlobalCampaignRevisionTx(ctx, tx, campaign)
	case 1:
		campaignRef := candidates[0].Campaign.CampaignRef
		if err := lockGlobalCampaignRefTx(ctx, tx, campaignRef); err != nil {
			return GlobalCampaignPersistResult{}, err
		}
		stored, found, loadErr := loadGlobalCampaignRowTx(ctx, tx, campaignRef, true)
		if loadErr != nil {
			return GlobalCampaignPersistResult{}, loadErr
		}
		if !found {
			return GlobalCampaignPersistResult{}, fmt.Errorf("%w: candidate %s disappeared before merge", ErrGlobalCampaignStoredCorrupt, campaignRef)
		}
		current, decodeErr := decodeStoredGlobalCampaign(campaignRef, stored)
		if decodeErr != nil {
			return GlobalCampaignPersistResult{}, decodeErr
		}
		if !globalCampaignAnchorsOverlap(anchors, globalCampaignLookupAnchorsFromCampaign(current)) {
			return GlobalCampaignPersistResult{}, fmt.Errorf("%w: candidate %s no longer shares incoming evidence", ErrGlobalCampaignRevisionConflict, campaignRef)
		}
		merged, changed, mergeErr := MergeGlobalCampaignMaterial(current, in)
		if mergeErr != nil {
			return GlobalCampaignPersistResult{}, mergeErr
		}
		if !changed {
			result, err = persistGlobalCampaignRevisionTx(ctx, tx, current)
			break
		}
		result, err = persistGlobalCampaignRevisionTx(ctx, tx, merged)
	default:
		refs := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			refs = append(refs, candidate.Campaign.CampaignRef)
		}
		sort.Strings(refs)
		return GlobalCampaignPersistResult{}, fmt.Errorf("%w: %s", ErrGlobalCampaignAmbiguousCandidates, strings.Join(refs, ","))
	}
	if err != nil {
		return GlobalCampaignPersistResult{}, err
	}
	return result, nil
}

// MergeGlobalCampaignMaterial unions a new partial evidence window into one
// already-validated campaign while preserving its stable identity and state.
// The merge is replay-stable: unchanged material does not mint a new revision.
func MergeGlobalCampaignMaterial(existing GlobalCampaign, in GlobalCampaignMaterializerInput) (GlobalCampaign, bool, error) {
	if err := ValidateGlobalCampaign(existing); err != nil {
		return GlobalCampaign{}, false, fmt.Errorf("existing global campaign is invalid: %w", err)
	}
	if err := validateGlobalCampaignManagedIdentityInput(in); err != nil {
		return GlobalCampaign{}, false, err
	}

	merged := existing
	observedAt := in.ObservedAt.UTC()
	if !in.ObservedAt.IsZero() {
		if merged.FirstObservedAt.IsZero() || observedAt.Before(merged.FirstObservedAt) {
			merged.FirstObservedAt = observedAt
		}
		if merged.LastObservedAt.IsZero() || observedAt.After(merged.LastObservedAt) {
			merged.LastObservedAt = observedAt
		}
	}

	merged.Networks = mergeGlobalCampaignStrings(merged.Networks, in.Networks)
	merged.Subjects = mergeGlobalCampaignStrings(merged.Subjects, in.Subjects)
	merged.Actors = mergeGlobalCampaignStrings(merged.Actors, in.Actors)
	merged.Assets = mergeGlobalCampaignStrings(merged.Assets, in.Assets)
	merged.Contracts = mergeGlobalCampaignStrings(merged.Contracts, in.Contracts)
	merged.Pools = mergeGlobalCampaignStrings(merged.Pools, in.Pools)
	merged.Bridges = mergeGlobalCampaignStrings(merged.Bridges, in.Bridges)
	merged.ObservationRefs = mergeGlobalCampaignStrings(merged.ObservationRefs, in.ObservationRefs)
	merged.RelationRefs = mergeGlobalCampaignStrings(merged.RelationRefs, in.RelationRefs)
	merged.BridgeLinkRefs = mergeGlobalCampaignStrings(merged.BridgeLinkRefs, in.BridgeLinkRefs)
	merged.CampaignGenomeRefs = mergeGlobalCampaignStrings(merged.CampaignGenomeRefs, in.CampaignGenomeRefs)
	merged.CampaignTempoRefs = mergeGlobalCampaignStrings(merged.CampaignTempoRefs, in.CampaignTempoRefs)
	merged.BehaviorSignatureRefs = mergeGlobalCampaignStrings(merged.BehaviorSignatureRefs, in.BehaviorSignatureRefs)
	merged.IncidentRefs = mergeGlobalCampaignStrings(merged.IncidentRefs, in.IncidentRefs)
	merged.VerdictRefs = mergeGlobalCampaignStrings(merged.VerdictRefs, in.VerdictRefs)
	merged.AttackPathRefs = mergeGlobalCampaignStrings(merged.AttackPathRefs, in.AttackPathRefs)
	merged.MissingEvidence = mergeGlobalCampaignStrings(merged.MissingEvidence, in.MissingEvidence)
	merged.VerifiedAnchorCount = maxGlobalCampaignInt(merged.VerifiedAnchorCount, in.VerifiedAnchorCount)
	merged.ObservedAnchorCount = maxGlobalCampaignInt(merged.ObservedAnchorCount, in.ObservedAnchorCount)

	incomingRuleset := strings.TrimSpace(in.RulesetVersion)
	existingRuleset := strings.TrimSpace(merged.RulesetVersion)
	if existingRuleset != "" && incomingRuleset != "" && existingRuleset != incomingRuleset {
		return GlobalCampaign{}, false, fmt.Errorf("%w: current=%q incoming=%q", ErrGlobalCampaignRulesetConflict, existingRuleset, incomingRuleset)
	}
	if existingRuleset == "" {
		merged.RulesetVersion = incomingRuleset
	}

	merged.CampaignRef = existing.CampaignRef
	merged.Revision = existing.Revision
	merged.State = existing.State
	merged = NewGlobalCampaign(merged)
	if merged.EvidenceHashSHA256 == existing.EvidenceHashSHA256 {
		return existing, false, nil
	}
	if existing.Revision == math.MaxUint64 {
		return GlobalCampaign{}, false, fmt.Errorf("global campaign revision overflow")
	}
	merged.Revision = existing.Revision + 1
	merged = NewGlobalCampaign(merged)
	if err := ValidateGlobalCampaign(merged); err != nil {
		return GlobalCampaign{}, false, fmt.Errorf("merged global campaign is invalid: %w", err)
	}
	return merged, true, nil
}

func validateGlobalCampaignManagedIdentityInput(in GlobalCampaignMaterializerInput) error {
	if strings.TrimSpace(in.ExistingCampaignRef) != "" || in.ExistingRevision != 0 || in.ExistingState != "" {
		return ErrGlobalCampaignManagedIdentityInput
	}
	return nil
}

func globalCampaignLookupAnchorsFromInput(in GlobalCampaignMaterializerInput) []globalCampaignAnchor {
	anchors := make([]globalCampaignAnchor, 0)
	anchors = appendGlobalCampaignAnchors(anchors, "observation", in.ObservationRefs)
	anchors = appendGlobalCampaignAnchors(anchors, "relation", in.RelationRefs)
	anchors = appendGlobalCampaignAnchors(anchors, "bridge_link", in.BridgeLinkRefs)
	anchors = appendGlobalCampaignAnchors(anchors, "campaign_genome", in.CampaignGenomeRefs)
	anchors = appendGlobalCampaignAnchors(anchors, "campaign_tempo", in.CampaignTempoRefs)
	anchors = appendGlobalCampaignAnchors(anchors, "behavior_signature", in.BehaviorSignatureRefs)
	anchors = appendGlobalCampaignAnchors(anchors, "incident", in.IncidentRefs)
	anchors = appendGlobalCampaignAnchors(anchors, "attack_path", in.AttackPathRefs)
	return normalizeGlobalCampaignAnchors(anchors)
}

func globalCampaignLookupAnchorsFromCampaign(campaign GlobalCampaign) []globalCampaignAnchor {
	anchors := make([]globalCampaignAnchor, 0)
	anchors = appendGlobalCampaignAnchors(anchors, "observation", campaign.ObservationRefs)
	anchors = appendGlobalCampaignAnchors(anchors, "relation", campaign.RelationRefs)
	anchors = appendGlobalCampaignAnchors(anchors, "bridge_link", campaign.BridgeLinkRefs)
	anchors = appendGlobalCampaignAnchors(anchors, "campaign_genome", campaign.CampaignGenomeRefs)
	anchors = appendGlobalCampaignAnchors(anchors, "campaign_tempo", campaign.CampaignTempoRefs)
	anchors = appendGlobalCampaignAnchors(anchors, "behavior_signature", campaign.BehaviorSignatureRefs)
	anchors = appendGlobalCampaignAnchors(anchors, "incident", campaign.IncidentRefs)
	anchors = appendGlobalCampaignAnchors(anchors, "attack_path", campaign.AttackPathRefs)
	return normalizeGlobalCampaignAnchors(anchors)
}

func appendGlobalCampaignAnchors(dst []globalCampaignAnchor, kind string, refs []string) []globalCampaignAnchor {
	for _, ref := range normalizeGlobalCampaignStrings(refs) {
		dst = append(dst, globalCampaignAnchor{Kind: kind, Ref: ref})
	}
	return dst
}

func normalizeGlobalCampaignAnchors(in []globalCampaignAnchor) []globalCampaignAnchor {
	for index := range in {
		in[index].Kind = strings.TrimSpace(in[index].Kind)
		in[index].Ref = strings.TrimSpace(in[index].Ref)
	}
	sort.Slice(in, func(i, j int) bool {
		if in[i].Kind == in[j].Kind {
			return in[i].Ref < in[j].Ref
		}
		return in[i].Kind < in[j].Kind
	})
	out := make([]globalCampaignAnchor, 0, len(in))
	for _, anchor := range in {
		if anchor.Kind == "" || anchor.Ref == "" {
			continue
		}
		if len(out) > 0 && out[len(out)-1] == anchor {
			continue
		}
		out = append(out, anchor)
	}
	return out
}

func globalCampaignAnchorsOverlap(a, b []globalCampaignAnchor) bool {
	a = normalizeGlobalCampaignAnchors(append([]globalCampaignAnchor{}, a...))
	b = normalizeGlobalCampaignAnchors(append([]globalCampaignAnchor{}, b...))
	i := 0
	j := 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			return true
		}
		if a[i].Kind < b[j].Kind || (a[i].Kind == b[j].Kind && a[i].Ref < b[j].Ref) {
			i++
		} else {
			j++
		}
	}
	return false
}

func lockGlobalCampaignAnchorsTx(ctx context.Context, tx *sql.Tx, anchors []globalCampaignAnchor) error {
	for _, anchor := range normalizeGlobalCampaignAnchors(append([]globalCampaignAnchor{}, anchors...)) {
		lockKey := "global-campaign-anchor:v1\x1f" + anchor.Kind + "\x1f" + anchor.Ref
		var advisoryResult any
		if err := tx.QueryRowContext(ctx,
			`SELECT pg_advisory_xact_lock(hashtextextended($1, 1))`, lockKey,
		).Scan(&advisoryResult); err != nil {
			return fmt.Errorf("lock global campaign anchor %s:%s: %w", anchor.Kind, anchor.Ref, err)
		}
	}
	return nil
}

func findGlobalCampaignCandidatesTx(ctx context.Context, tx *sql.Tx, anchors []globalCampaignAnchor) ([]GlobalCampaignCandidate, error) {
	anchors = normalizeGlobalCampaignAnchors(append([]globalCampaignAnchor{}, anchors...))
	if len(anchors) == 0 {
		return nil, nil
	}
	clauses := make([]string, 0, len(anchors))
	args := make([]any, 0, len(anchors)*2)
	for _, anchor := range anchors {
		kindPosition := len(args) + 1
		refPosition := kindPosition + 1
		clauses = append(clauses, fmt.Sprintf("(anchor.anchor_kind=$%d AND anchor.anchor_ref=$%d)", kindPosition, refPosition))
		args = append(args, anchor.Kind, anchor.Ref)
	}
	query := fmt.Sprintf(`
		SELECT anchor.campaign_ref, count(*)::bigint AS shared_anchor_count
		FROM global_campaign_anchor_index AS anchor
		JOIN global_campaign_current AS current_campaign
		  ON current_campaign.campaign_ref=anchor.campaign_ref
		 AND current_campaign.revision=anchor.revision
		WHERE %s
		GROUP BY anchor.campaign_ref
		ORDER BY shared_anchor_count DESC, anchor.campaign_ref ASC`, strings.Join(clauses, " OR "))

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query global campaign candidates: %w", err)
	}
	defer rows.Close()

	type candidateRef struct {
		CampaignRef string
		SharedCount int64
	}
	refs := make([]candidateRef, 0)
	for rows.Next() {
		var candidate candidateRef
		if err := rows.Scan(&candidate.CampaignRef, &candidate.SharedCount); err != nil {
			return nil, fmt.Errorf("scan global campaign candidate: %w", err)
		}
		refs = append(refs, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global campaign candidates: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close global campaign candidate rows: %w", err)
	}

	candidates := make([]GlobalCampaignCandidate, 0, len(refs))
	for _, ref := range refs {
		stored, found, err := loadGlobalCampaignRowTx(ctx, tx, ref.CampaignRef, false)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("%w: candidate %s disappeared during lookup", ErrGlobalCampaignStoredCorrupt, ref.CampaignRef)
		}
		campaign, err := decodeStoredGlobalCampaign(ref.CampaignRef, stored)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, GlobalCampaignCandidate{
			Campaign:          campaign,
			SharedAnchorCount: ref.SharedCount,
		})
	}
	return candidates, nil
}

func replaceGlobalCampaignAnchorIndexTx(ctx context.Context, tx *sql.Tx, campaign GlobalCampaign) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM global_campaign_anchor_index WHERE campaign_ref=$1`, campaign.CampaignRef,
	); err != nil {
		return fmt.Errorf("clear global campaign anchor index for %s: %w", campaign.CampaignRef, err)
	}
	for _, anchor := range globalCampaignLookupAnchorsFromCampaign(campaign) {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO global_campaign_anchor_index
				(campaign_ref,revision,anchor_kind,anchor_ref)
			VALUES ($1,$2,$3,$4)`,
			campaign.CampaignRef,
			int64(campaign.Revision),
			anchor.Kind,
			anchor.Ref,
		); err != nil {
			return fmt.Errorf("index global campaign anchor %s:%s: %w", anchor.Kind, anchor.Ref, err)
		}
	}
	return nil
}

func mergeGlobalCampaignStrings(current, incoming []string) []string {
	merged := make([]string, 0, len(current)+len(incoming))
	merged = append(merged, current...)
	merged = append(merged, incoming...)
	return normalizeGlobalCampaignStrings(merged)
}

func maxGlobalCampaignInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
