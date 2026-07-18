package services

import (
	"strings"
	"testing"
	"time"
)

func baseC006Report() UnifiedRadarBehaviorReport {
	return UnifiedRadarBehaviorReport{RulesetVersion: UnifiedRadarRulesetVersionV110, Mint: "MintA", CreatorWallet: "Creator111", Signals: []UnifiedRadarSignal{}, Evidence: []ActorDefenseEvidenceRecord{}, GeneratedAt: time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)}
}

func baseC006Relation() CrossTokenCreatorHolderTransfer {
	return CrossTokenCreatorHolderTransfer{Available: true, Status: "verified", Mint: "MintA", CreatorWallet: "Creator111", RecipientTokenAccount: "RecipientATA111", RecipientOwnerWallet: "Holder111", RecipientOwnerResolved: true, TransferSignature: "Sig111", Slot: 12345, Direction: "creator_to_recipient_owner", Amount: 650000000, Supply: 1000000000, OtherTokens: []RepeatDominantHolderMatch{{Mint: "MintB", Percentage: 81.85, Rank: 1, ScannedAt: "2026-07-17T00:00:00Z"}}, ObservedAt: time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)}
}

func TestC006SingleVerifiedCrossTokenRelationCapsC(t *testing.T) {
	report := ApplyCrossTokenCreatorHolderTransferRuleV120(baseC006Report(), baseC006Relation(), time.Time{})
	signal := unifiedSignalByID(t, report.Signals, UnifiedRuleCrossTokenCreatorHolderTransfer)
	if !signal.Triggered || signal.EvidenceStatus != "verified" || signal.GradeEffect != "hard_cap_C" {
		t.Fatalf("signal=%#v", signal)
	}
	if signal.Signatures[0] != "Sig111" || signal.Metrics["slot"].(int64) != 12345 {
		t.Fatalf("refs incomplete: %#v", signal)
	}
	verdict := EvaluateUnifiedRadarVerdictV120("MintA", ActorDefenseRuleVerdict{}, report)
	if verdict.Grade != "C" || verdict.RulesetVersion != UnifiedRadarRulesetVersionV120 || !strings.Contains(strings.Join(verdict.DecisionPath, "\n"), "URD-C006") {
		t.Fatalf("verdict=%#v", verdict)
	}
}

func TestC006TwoOtherTokensCapsD(t *testing.T) {
	relation := baseC006Relation()
	relation.OtherTokens = append(relation.OtherTokens, RepeatDominantHolderMatch{Mint: "MintC", Percentage: 58.73, Rank: 2, ScannedAt: "2026-07-17T00:00:00Z"})
	report := ApplyCrossTokenCreatorHolderTransferRuleV120(baseC006Report(), relation, time.Time{})
	if signal := unifiedSignalByID(t, report.Signals, UnifiedRuleCrossTokenCreatorHolderTransfer); signal.GradeEffect != "hard_cap_D" {
		t.Fatalf("signal=%#v", signal)
	}
	if verdict := EvaluateUnifiedRadarVerdictV120("MintA", ActorDefenseRuleVerdict{}, report); verdict.Grade != "D" {
		t.Fatalf("verdict=%#v", verdict)
	}
}

func TestC006RecipientOwnerUnresolvedEvidencePending(t *testing.T) {
	relation := baseC006Relation()
	relation.RecipientOwnerWallet = ""
	relation.RecipientOwnerResolved = false
	report := ApplyCrossTokenCreatorHolderTransferRuleV120(baseC006Report(), relation, time.Time{})
	signal := unifiedSignalByID(t, report.Signals, UnifiedRuleCrossTokenCreatorHolderTransfer)
	if signal.Triggered || !strings.Contains(signal.Summary, "EVIDENCE PENDING") || len(signal.Limitations) == 0 {
		t.Fatalf("signal=%#v", signal)
	}
}

func TestC006AggregateOnlyNoSignatureCannotTrigger(t *testing.T) {
	relation := baseC006Relation()
	relation.TransferSignature = ""
	relation.Slot = 0
	report := ApplyCrossTokenCreatorHolderTransferRuleV120(baseC006Report(), relation, time.Time{})
	if signal := unifiedSignalByID(t, report.Signals, UnifiedRuleCrossTokenCreatorHolderTransfer); signal.Triggered || !strings.Contains(signal.Summary, "EVIDENCE PENDING") {
		t.Fatalf("signal=%#v", signal)
	}
}

func TestC006NoCrossTokenDominanceDoesNotTrigger(t *testing.T) {
	relation := baseC006Relation()
	relation.OtherTokens = nil
	report := ApplyCrossTokenCreatorHolderTransferRuleV120(baseC006Report(), relation, time.Time{})
	signal := unifiedSignalByID(t, report.Signals, UnifiedRuleCrossTokenCreatorHolderTransfer)
	if signal.Triggered || signal.EvidenceStatus != "verified" {
		t.Fatalf("signal=%#v", signal)
	}
}

func TestC006RulesetVersionBump(t *testing.T) {
	report := ApplyCrossTokenCreatorHolderTransferRuleV120(baseC006Report(), baseC006Relation(), time.Time{})
	if report.RulesetVersion != "koschei-unified-radar-rules-v1.2.0" {
		t.Fatalf("report ruleset=%q", report.RulesetVersion)
	}
	if verdict := EvaluateUnifiedRadarVerdictV120("MintA", ActorDefenseRuleVerdict{}, report); verdict.RulesetVersion != "koschei-unified-radar-rules-v1.2.0" {
		t.Fatalf("verdict ruleset=%q", verdict.RulesetVersion)
	}
}
