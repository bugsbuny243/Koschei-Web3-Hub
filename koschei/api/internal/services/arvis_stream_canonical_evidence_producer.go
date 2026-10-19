package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// evaluateArvisStreamCanonicalEvidenceFinal is the final AUTHORITY BOUNDARY
// between the canonical actor/behavior investigator and the stream receipt.
// The caller must supply genuine, already-collected evidence from the same
// token investigation: legacy ARVIS arms are not eligible inputs. We never
// inject a source citation, promote observed evidence to verified, or copy an
// arm's signing flag into the canonical result.
//
// This function deliberately does not enable the automatic stream worker.
// Provider provenance/coverage is not yet established at the worker callsite.
func evaluateArvisStreamCanonicalEvidenceFinal(
	event arvisStreamTarget,
	actor ActorDefenseRuleVerdict,
	behavior UnifiedRadarBehaviorReport,
	registry GlobalRadarTrustedVerdictRegistry,
) (UnifiedRadarVerdict, error) {
	if event.Network != "solana-mainnet" ||
		(event.ModuleID != ModulePumpSybilRadar && event.ModuleID != ModuleRaydiumPoolGuardian) ||
		strings.TrimSpace(event.Target) == "" || strings.TrimSpace(event.Signature) == "" ||
		strings.TrimSpace(event.ID) == "" {
		return UnifiedRadarVerdict{}, fmt.Errorf("%w: invalid stream source or unsupported provider", errArvisStreamCanonicalFinalUnavailable)
	}
	if strings.TrimSpace(behavior.Mint) != event.Target ||
		behavior.RulesetVersion != UnifiedRadarRulesetVersionV140 ||
		behavior.GeneratedAt.IsZero() ||
		actor.RulesetVersion != ActorDefenseRulesetVersion {
		return UnifiedRadarVerdict{}, fmt.Errorf("%w: actor/behavior target or ruleset unavailable", errArvisStreamInsufficientEvidence)
	}

	verifiedActor := false
	sourceCited := false
	verifiedHard := false
	for _, hit := range actor.TriggeredRules {
		if hit.EvidenceStatus != "verified" {
			continue
		}
		if !strings.HasPrefix(hit.RuleID, "ARD-") {
			continue
		}
		verifiedActor = true
		if hit.Tier == "hard" {
			verifiedHard = true
		}
		if arvisVerifiedRuleCitesStreamEvent(hit.Signatures, hit.EvidenceKeys, event.Signature) {
			sourceCited = true
		}
	}
	// A hard-trigger ceiling must be anchored by a verified hard actor rule,
	// not merely by a grade/flag in a caller-supplied actor projection.
	if actor.Verdict == "hard_trigger" && !verifiedHard {
		return UnifiedRadarVerdict{}, fmt.Errorf("%w: actor hard trigger has no verified hard evidence", errArvisStreamInsufficientEvidence)
	}

	verifiedBehavior := false
	for _, signal := range behavior.Signals {
		if !signal.Triggered || signal.EvidenceStatus != "verified" ||
			!strings.HasPrefix(signal.RuleID, "URD-") {
			continue
		}
		verifiedBehavior = true
		if arvisVerifiedRuleCitesStreamEvent(signal.Signatures, signal.EvidenceKeys, event.Signature) {
			sourceCited = true
		}
	}
	if !verifiedActor || !verifiedBehavior || !sourceCited {
		return UnifiedRadarVerdict{}, fmt.Errorf("%w: verified actor and behavior rules citing the stream source are required", errArvisStreamInsufficientEvidence)
	}

	// This is the same deterministic v1.4 grading path used for the full
	// customer/owner investigation. Missing real grade triggers must never
	// produce a signed "A" or other synthetic positive verdict.
	verdict := EvaluateUnifiedRadarVerdictV140(event.Target, actor, behavior)
	if verdict.Grade == "-" || strings.TrimSpace(verdict.Grade) == "" {
		return UnifiedRadarVerdict{}, fmt.Errorf("%w: canonical decision has no supported grade trigger", errArvisStreamInsufficientEvidence)
	}
	// Pin the original collector's evidence-generation time. Replaying the
	// exact SAME verified evidence must never mint a different Ed25519
	// decision just because a worker retry happened seconds later.
	verdict.GeneratedAt = behavior.GeneratedAt.UTC()
	verdict = FinalizeUnifiedRadarVerdictContractForNetwork(event.Target, event.Network, verdict)
	if !verdict.Signed || verdict.Signature == "" || verdict.KeyID == "" {
		return UnifiedRadarVerdict{}, fmt.Errorf("%w: canonical Ed25519 signer unavailable", errArvisStreamCanonicalFinalUnavailable)
	}
	if err := validateArvisStreamCanonicalVerdict(event, verdict, registry); err != nil {
		// Source binding and trusted-key verification MUST happen before any
		// queue state is changed. The signature cannot authorize its own key.
		return UnifiedRadarVerdict{}, fmt.Errorf("%w: %v", errArvisStreamCanonicalFinalUnavailable, err)
	}
	return verdict, nil
}

func arvisVerifiedRuleCitesStreamEvent(signatures, keys []string, sourceSignature string) bool {
	if sourceSignature == "" {
		return false
	}
	for _, signature := range signatures {
		if strings.TrimSpace(signature) == sourceSignature {
			return true
		}
	}
	for _, key := range keys {
		if strings.TrimSpace(key) == "event-signature:"+sourceSignature {
			return true
		}
	}
	return false
}

// persistArvisStreamCanonicalEvidenceFinalTx permits an already-collected,
// verified actor/behavior decision to reach the IMMUTABLE signed-final receipt
// in the SAME PostgreSQL transaction as its stream processing status.
// The transaction must be committed by its owner; negative decisions make
// no write whatsoever, and the legacy 14 evidence arms never mint a final.
func persistArvisStreamCanonicalEvidenceFinalTx(
	ctx context.Context, tx *sql.Tx,
	event arvisStreamTarget,
	actor ActorDefenseRuleVerdict,
	behavior UnifiedRadarBehaviorReport,
	registry GlobalRadarTrustedVerdictRegistry,
) (UnifiedRadarVerdict, error) {
	if tx == nil {
		return UnifiedRadarVerdict{}, errors.New("ARVIS canonical evidence transaction unavailable")
	}
	final, err := evaluateArvisStreamCanonicalEvidenceFinal(event, actor, behavior, registry)
	if err != nil {
		return UnifiedRadarVerdict{}, err
	}
	if err := persistArvisStreamCanonicalFinalTx(ctx, tx, event, final, registry); err != nil {
		return UnifiedRadarVerdict{}, err
	}
	return final, nil
}

// persistArvisStreamCanonicalEvidenceFinal is the production entry point for
// a future approved live actor/behavior investigator. It does not contact RPC
// providers itself and MUST NOT be called with inferred legacy arm findings.
// A provider-to-actor/behavior adapter with native provenance acceptance is
// still required before the current disabled worker can be activated.
func persistArvisStreamCanonicalEvidenceFinal(
	ctx context.Context, db *sql.DB,
	event arvisStreamTarget,
	actor ActorDefenseRuleVerdict,
	behavior UnifiedRadarBehaviorReport,
	registry GlobalRadarTrustedVerdictRegistry,
) (UnifiedRadarVerdict, error) {
	if db == nil {
		return UnifiedRadarVerdict{}, errors.New("ARVIS canonical evidence database unavailable")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return UnifiedRadarVerdict{}, fmt.Errorf("begin ARVIS canonical evidence transaction: %w", err)
	}
	defer tx.Rollback()
	final, err := persistArvisStreamCanonicalEvidenceFinalTx(ctx, tx, event, actor, behavior, registry)
	if err != nil {
		return UnifiedRadarVerdict{}, err
	}
	if err := tx.Commit(); err != nil {
		return UnifiedRadarVerdict{}, fmt.Errorf("commit ARVIS canonical evidence receipt: %w", err)
	}
	return final, nil
}
