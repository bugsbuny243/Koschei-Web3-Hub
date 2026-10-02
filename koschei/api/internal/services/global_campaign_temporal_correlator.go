package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	GlobalCampaignTemporalCorrelatorVersion = "koschei.global-campaign-temporal-correlator.v1"
	GlobalCampaignTemporalMaxEvents         = 4096
	GlobalCampaignTemporalMaxCorrelations   = 10000
)

var (
	ErrGlobalCampaignTemporalEvidenceRefRequired = errors.New("global campaign temporal evidence reference is required")
	ErrGlobalCampaignTemporalTimestampRequired   = errors.New("global campaign temporal evidence timestamp is required")
	ErrGlobalCampaignTemporalEvidenceConflict    = errors.New("global campaign temporal evidence reference has divergent canonical content")
	ErrGlobalCampaignTemporalEvidenceState       = errors.New("global campaign temporal evidence state is unsupported")
	ErrGlobalCampaignTemporalVerifiedRefBoundary = errors.New("global campaign temporal verified reference must also be present in the canonical context references")
	ErrGlobalCampaignTemporalBudgetExceeded      = errors.New("global campaign temporal correlation budget exceeded")
)

var globalCampaignTemporalWindows = []struct {
	Name     string
	Duration time.Duration
}{
	{Name: "1m", Duration: time.Minute},
	{Name: "5m", Duration: 5 * time.Minute},
	{Name: "15m", Duration: 15 * time.Minute},
	{Name: "1h", Duration: time.Hour},
	{Name: "24h", Duration: 24 * time.Hour},
}

// GlobalCampaignTemporalEvent is a reference-first event projection. Ref must
// identify already-retained evidence. Generic RelationRefs and BridgeLinkRefs
// are correlation context only. A caller may populate VerifiedRelationRefs or
// VerifiedBridgeLinkRefs only from an already-verified persistent relation or
// bridge-link contract; the temporal layer never upgrades generic refs itself.
// SubjectID is the canonical Global Radar subject identity and is required on
// both sides before an explicit relation/bridge can upgrade a pair to VERIFIED.
// The correlator never invents a timestamp, entity attribution, bridge proof,
// verdict, grade, or containment decision.
type GlobalCampaignTemporalEvent struct {
	Ref                    string    `json:"ref"`
	SubjectID              string    `json:"subject_id,omitempty"`
	Network                string    `json:"network"`
	Kind                   string    `json:"kind"`
	ObservedAt             time.Time `json:"observed_at"`
	EvidenceState          string    `json:"evidence_state"`
	RelationRefs           []string  `json:"relation_refs,omitempty"`
	BridgeLinkRefs         []string  `json:"bridge_link_refs,omitempty"`
	VerifiedRelationRefs   []string  `json:"verified_relation_refs,omitempty"`
	VerifiedBridgeLinkRefs []string  `json:"verified_bridge_link_refs,omitempty"`
	Invalidated            bool      `json:"invalidated"`
}

// GlobalCampaignTemporalCorrelation describes a deterministic event-time
// relationship candidate. Temporal proximity alone is never VERIFIED. A
// verified same-network result requires distinct canonical subjects plus a
// shared explicit verified relation; a verified cross-network result requires
// distinct canonical subjects plus a shared explicit verified bridge link.
// Otherwise the result remains observed/watch/inferred context.
type GlobalCampaignTemporalCorrelation struct {
	LeftRef                string   `json:"left_ref"`
	RightRef               string   `json:"right_ref"`
	LeftSubjectID          string   `json:"left_subject_id,omitempty"`
	RightSubjectID         string   `json:"right_subject_id,omitempty"`
	LeftNetwork            string   `json:"left_network"`
	RightNetwork           string   `json:"right_network"`
	Window                 string   `json:"window"`
	DeltaSeconds           int64    `json:"delta_seconds"`
	EvidenceState          string   `json:"evidence_state"`
	CrossNetwork           bool     `json:"cross_network"`
	WatchOnly              bool     `json:"watch_only"`
	RelationRefs           []string `json:"relation_refs,omitempty"`
	BridgeLinkRefs         []string `json:"bridge_link_refs,omitempty"`
	VerifiedRelationRefs   []string `json:"verified_relation_refs,omitempty"`
	VerifiedBridgeLinkRefs []string `json:"verified_bridge_link_refs,omitempty"`
}

type GlobalCampaignTemporalCorrelationReport struct {
	Version                string                              `json:"version"`
	CampaignRef            string                              `json:"campaign_ref"`
	Available              bool                                `json:"available"`
	Complete               bool                                `json:"complete"`
	Status                 string                              `json:"status"`
	EventCount             int                                 `json:"event_count"`
	ActiveEventCount       int                                 `json:"active_event_count"`
	CorrelationCount       int                                 `json:"correlation_count"`
	VerifiedLinkCount      int                                 `json:"verified_link_count"`
	CrossNetworkCount      int                                 `json:"cross_network_count"`
	InvalidatedRefs        []string                            `json:"invalidated_refs"`
	Correlations           []GlobalCampaignTemporalCorrelation `json:"correlations"`
	FingerprintSHA256      string                              `json:"fingerprint_sha256"`
	VerdictAuthority       bool                                `json:"verdict_authority"`
	GradeAuthority         bool                                `json:"grade_authority"`
	ContainmentAuthority   bool                                `json:"containment_authority"`
	SameOperatorClaim      bool                                `json:"same_operator_claim"`
	RealWorldIdentityClaim bool                                `json:"real_world_identity_claim"`
	WrongdoingClaim        bool                                `json:"wrongdoing_claim"`
	Limitations            []string                            `json:"limitations"`
}

// BuildGlobalCampaignTemporalCorrelation is deterministic over canonical event
// content. Input arrival order does not affect the result or fingerprint.
// Exact duplicate refs are idempotent; divergent content for one ref fails
// closed. Invalidated/reorg-revoked evidence remains visible in InvalidatedRefs
// but cannot participate in active correlations. Work is bounded so a dense
// telemetry batch cannot create unbounded quadratic output; larger campaigns
// must be chunked and replayed through deterministic windows by the caller.
func BuildGlobalCampaignTemporalCorrelation(campaignRef string, events []GlobalCampaignTemporalEvent) (GlobalCampaignTemporalCorrelationReport, error) {
	if len(events) > GlobalCampaignTemporalMaxEvents {
		return GlobalCampaignTemporalCorrelationReport{}, fmt.Errorf(
			"%w: events=%d max_events=%d",
			ErrGlobalCampaignTemporalBudgetExceeded,
			len(events),
			GlobalCampaignTemporalMaxEvents,
		)
	}

	out := GlobalCampaignTemporalCorrelationReport{
		Version:         GlobalCampaignTemporalCorrelatorVersion,
		CampaignRef:     strings.TrimSpace(campaignRef),
		Complete:        true,
		Status:          "no_temporal_correlations_observed",
		InvalidatedRefs: []string{},
		Correlations:    []GlobalCampaignTemporalCorrelation{},
		Limitations: []string{
			"Temporal proximity alone is correlation context and never proves common control, identity, intent or wrongdoing.",
			"Cross-network VERIFIED continuity requires distinct canonical subjects and an explicit shared verified bridge-link reference; timing similarity or a generic bridge reference alone remains non-verified context.",
			"Same-network VERIFIED correlation requires distinct canonical subjects and an explicit shared verified relation reference; co-occurrence or a generic relation reference alone remains non-verified context.",
			"signed_artifact is distinct from verified on-chain evidence and cannot by itself upgrade a temporal correlation to VERIFIED.",
			"Invalidated or reorg-revoked evidence is excluded from active correlations; missing evidence is not interpreted as safety.",
			"Temporal work is batch-bounded; callers must deterministically chunk and replay larger campaign timelines rather than relying on silent truncation.",
		},
	}

	normalized, invalidated, err := normalizeGlobalCampaignTemporalEvents(events)
	if err != nil {
		return GlobalCampaignTemporalCorrelationReport{}, err
	}
	out.EventCount = len(normalized) + len(invalidated)
	out.ActiveEventCount = len(normalized)
	out.InvalidatedRefs = invalidated

	for i := 0; i < len(normalized); i++ {
		for j := i + 1; j < len(normalized); j++ {
			delta := normalized[j].ObservedAt.Sub(normalized[i].ObservedAt)
			if delta < 0 {
				return GlobalCampaignTemporalCorrelationReport{}, fmt.Errorf("temporal event ordering invariant violated")
			}
			window, ok := globalCampaignTemporalWindow(delta)
			if !ok {
				break
			}
			if len(out.Correlations) >= GlobalCampaignTemporalMaxCorrelations {
				return GlobalCampaignTemporalCorrelationReport{}, fmt.Errorf(
					"%w: correlations=%d max_correlations=%d",
					ErrGlobalCampaignTemporalBudgetExceeded,
					len(out.Correlations)+1,
					GlobalCampaignTemporalMaxCorrelations,
				)
			}
			correlation := correlateGlobalCampaignTemporalPair(normalized[i], normalized[j], window, delta)
			out.Correlations = append(out.Correlations, correlation)
			if correlation.EvidenceState == "verified" {
				out.VerifiedLinkCount++
			}
			if correlation.CrossNetwork {
				out.CrossNetworkCount++
			}
		}
	}

	out.CorrelationCount = len(out.Correlations)
	out.Available = out.CorrelationCount > 0
	if out.Available {
		out.Status = "temporal_correlations_observed"
	}
	if out.VerifiedLinkCount > 0 {
		out.Status = "verified_explicit_links_with_temporal_context_observed"
	}
	out.FingerprintSHA256 = hashGlobalCampaignTemporalCorrelationReport(out)
	return out, nil
}

func normalizeGlobalCampaignTemporalEvents(events []GlobalCampaignTemporalEvent) ([]GlobalCampaignTemporalEvent, []string, error) {
	byRef := make(map[string]GlobalCampaignTemporalEvent, len(events))
	hashByRef := make(map[string]string, len(events))
	for _, raw := range events {
		event := raw
		event.Ref = strings.TrimSpace(event.Ref)
		if event.Ref == "" {
			return nil, nil, ErrGlobalCampaignTemporalEvidenceRefRequired
		}
		if event.ObservedAt.IsZero() {
			return nil, nil, fmt.Errorf("%w: %s", ErrGlobalCampaignTemporalTimestampRequired, event.Ref)
		}
		event.ObservedAt = event.ObservedAt.UTC()
		event.SubjectID = strings.TrimSpace(event.SubjectID)
		event.Network = normalizeRadarNetwork(event.Network)
		event.Kind = strings.TrimSpace(event.Kind)
		event.EvidenceState = strings.ToLower(strings.TrimSpace(event.EvidenceState))
		if !validGlobalCampaignTemporalEvidenceState(event.EvidenceState) {
			return nil, nil, fmt.Errorf("%w: ref=%s state=%q", ErrGlobalCampaignTemporalEvidenceState, event.Ref, event.EvidenceState)
		}
		event.RelationRefs = normalizeGlobalCampaignStrings(event.RelationRefs)
		event.BridgeLinkRefs = normalizeGlobalCampaignStrings(event.BridgeLinkRefs)
		event.VerifiedRelationRefs = normalizeGlobalCampaignStrings(event.VerifiedRelationRefs)
		event.VerifiedBridgeLinkRefs = normalizeGlobalCampaignStrings(event.VerifiedBridgeLinkRefs)
		if !globalCampaignTemporalRefsSubset(event.VerifiedRelationRefs, event.RelationRefs) {
			return nil, nil, fmt.Errorf("%w: ref=%s kind=relation", ErrGlobalCampaignTemporalVerifiedRefBoundary, event.Ref)
		}
		if !globalCampaignTemporalRefsSubset(event.VerifiedBridgeLinkRefs, event.BridgeLinkRefs) {
			return nil, nil, fmt.Errorf("%w: ref=%s kind=bridge_link", ErrGlobalCampaignTemporalVerifiedRefBoundary, event.Ref)
		}

		canonicalHash := hashGlobalCampaignTemporalEvent(event)
		if previousHash, exists := hashByRef[event.Ref]; exists {
			if previousHash != canonicalHash {
				return nil, nil, fmt.Errorf("%w: %s", ErrGlobalCampaignTemporalEvidenceConflict, event.Ref)
			}
			continue
		}
		hashByRef[event.Ref] = canonicalHash
		byRef[event.Ref] = event
	}

	active := make([]GlobalCampaignTemporalEvent, 0, len(byRef))
	invalidated := make([]string, 0)
	for _, event := range byRef {
		if event.Invalidated {
			invalidated = append(invalidated, event.Ref)
			continue
		}
		active = append(active, event)
	}
	if len(active)+len(invalidated) > GlobalCampaignTemporalMaxEvents {
		return nil, nil, fmt.Errorf(
			"%w: unique_events=%d max_events=%d",
			ErrGlobalCampaignTemporalBudgetExceeded,
			len(active)+len(invalidated),
			GlobalCampaignTemporalMaxEvents,
		)
	}
	sort.Strings(invalidated)
	sort.SliceStable(active, func(i, j int) bool {
		if !active[i].ObservedAt.Equal(active[j].ObservedAt) {
			return active[i].ObservedAt.Before(active[j].ObservedAt)
		}
		if active[i].Network != active[j].Network {
			return active[i].Network < active[j].Network
		}
		if active[i].SubjectID != active[j].SubjectID {
			return active[i].SubjectID < active[j].SubjectID
		}
		if active[i].Ref != active[j].Ref {
			return active[i].Ref < active[j].Ref
		}
		return active[i].Kind < active[j].Kind
	})
	return active, invalidated, nil
}

func validGlobalCampaignTemporalEvidenceState(state string) bool {
	switch state {
	case "verified", "observed", "signed_artifact", "inferred", "unknown":
		return true
	default:
		return false
	}
}

func globalCampaignTemporalWindow(delta time.Duration) (string, bool) {
	for _, window := range globalCampaignTemporalWindows {
		if delta <= window.Duration {
			return window.Name, true
		}
	}
	return "", false
}

func correlateGlobalCampaignTemporalPair(left, right GlobalCampaignTemporalEvent, window string, delta time.Duration) GlobalCampaignTemporalCorrelation {
	sharedRelations := intersectGlobalCampaignStrings(left.RelationRefs, right.RelationRefs)
	sharedBridges := intersectGlobalCampaignStrings(left.BridgeLinkRefs, right.BridgeLinkRefs)
	sharedVerifiedRelations := intersectGlobalCampaignStrings(left.VerifiedRelationRefs, right.VerifiedRelationRefs)
	sharedVerifiedBridges := intersectGlobalCampaignStrings(left.VerifiedBridgeLinkRefs, right.VerifiedBridgeLinkRefs)
	crossNetwork := left.Network != right.Network
	distinctCanonicalSubjects := left.SubjectID != "" && right.SubjectID != "" && left.SubjectID != right.SubjectID
	explicitVerifiedLink := false
	if distinctCanonicalSubjects && left.Network != "" && right.Network != "" && left.EvidenceState == "verified" && right.EvidenceState == "verified" {
		if crossNetwork {
			explicitVerifiedLink = len(sharedVerifiedBridges) > 0
		} else {
			explicitVerifiedLink = len(sharedVerifiedRelations) > 0
		}
	}

	state := "observed"
	watchOnly := true
	if explicitVerifiedLink {
		state = "verified"
		watchOnly = false
	} else if left.EvidenceState == "unknown" || right.EvidenceState == "unknown" {
		state = "unknown"
	} else if left.EvidenceState == "inferred" || right.EvidenceState == "inferred" {
		state = "inferred"
	}

	return GlobalCampaignTemporalCorrelation{
		LeftRef:                left.Ref,
		RightRef:               right.Ref,
		LeftSubjectID:          left.SubjectID,
		RightSubjectID:         right.SubjectID,
		LeftNetwork:            left.Network,
		RightNetwork:           right.Network,
		Window:                 window,
		DeltaSeconds:           int64(delta / time.Second),
		EvidenceState:          state,
		CrossNetwork:           crossNetwork,
		WatchOnly:              watchOnly,
		RelationRefs:           sharedRelations,
		BridgeLinkRefs:         sharedBridges,
		VerifiedRelationRefs:   sharedVerifiedRelations,
		VerifiedBridgeLinkRefs: sharedVerifiedBridges,
	}
}

func globalCampaignTemporalRefsSubset(subset, superset []string) bool {
	if len(subset) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(superset))
	for _, value := range superset {
		set[value] = struct{}{}
	}
	for _, value := range subset {
		if _, ok := set[value]; !ok {
			return false
		}
	}
	return true
}

func intersectGlobalCampaignStrings(left, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return []string{}
	}
	set := make(map[string]struct{}, len(left))
	for _, value := range left {
		set[value] = struct{}{}
	}
	out := make([]string, 0)
	for _, value := range right {
		if _, ok := set[value]; ok {
			out = append(out, value)
		}
	}
	return normalizeGlobalCampaignStrings(out)
}

func hashGlobalCampaignTemporalEvent(event GlobalCampaignTemporalEvent) string {
	payload, err := json.Marshal(event)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func hashGlobalCampaignTemporalCorrelationReport(report GlobalCampaignTemporalCorrelationReport) string {
	report.FingerprintSHA256 = ""
	payload, err := json.Marshal(report)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}
