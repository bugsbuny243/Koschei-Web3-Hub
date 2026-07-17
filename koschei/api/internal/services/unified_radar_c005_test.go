package services

import (
	"testing"
	"time"
)

func c005Holder(share float64) HolderIntelligence {
	return HolderIntelligence{
		Available:               true,
		OwnerAggregationApplied: true,
		CirculatingSupply:       1_000_000,
		TopOwnerPercentage:      share,
		Rows: []HolderIntelligenceRow{
			{
				Rank:                   1,
				OwnerWallet:            "Owner111111111111111111111111111111111",
				OwnerResolved:          true,
				RiskBearing:            true,
				ExcludedFromHolderRisk: false,
			},
		},
	}
}

func TestC005OwnerResolvedFThreshold(t *testing.T) {
	now := time.Date(2026, 7, 17, 6, 0, 0, 0, time.UTC)
	behavior := UnifiedRadarBehaviorReport{Mint: "MintF", Signals: []UnifiedRadarSignal{}, Evidence: []ActorDefenseEvidenceRecord{}, GeneratedAt: now}
	behavior = ApplyOwnerConcentrationRuleV110(behavior, c005Holder(70), now)

	if behavior.RulesetVersion != UnifiedRadarRulesetVersionV110 {
		t.Fatalf("ruleset=%q", behavior.RulesetVersion)
	}
	if len(behavior.Signals) != 1 {
		t.Fatalf("signals=%d", len(behavior.Signals))
	}
	signal := behavior.Signals[0]
	if !signal.Triggered || signal.EvidenceStatus != "verified" || signal.GradeEffect != "hard_cap_F" {
		t.Fatalf("signal=%#v", signal)
	}
	if len(signal.EvidenceKeys) != 1 || signal.EvidenceKeys[0] != "owner:Owner111111111111111111111111111111111" {
		t.Fatalf("evidence_keys=%v", signal.EvidenceKeys)
	}

	verdict := EvaluateUnifiedRadarVerdictV110("MintF", ActorDefenseRuleVerdict{}, behavior)
	if verdict.Grade != "F" || verdict.Verdict != "hard_trigger" || !verdict.Signed || verdict.Signature == "" {
		t.Fatalf("verdict=%#v", verdict)
	}
}

func TestC005OwnerResolvedDThreshold(t *testing.T) {
	now := time.Date(2026, 7, 17, 6, 0, 0, 0, time.UTC)
	behavior := UnifiedRadarBehaviorReport{Mint: "MintD", Signals: []UnifiedRadarSignal{}, Evidence: []ActorDefenseEvidenceRecord{}, GeneratedAt: now}
	behavior = ApplyOwnerConcentrationRuleV110(behavior, c005Holder(50), now)
	verdict := EvaluateUnifiedRadarVerdictV110("MintD", ActorDefenseRuleVerdict{}, behavior)

	if behavior.Signals[0].GradeEffect != "hard_cap_D" || verdict.Grade != "D" || !verdict.Signed {
		t.Fatalf("behavior=%#v verdict=%#v", behavior, verdict)
	}
}

func TestC005RawTokenAccountConcentrationCannotTrigger(t *testing.T) {
	now := time.Date(2026, 7, 17, 6, 0, 0, 0, time.UTC)
	holder := c005Holder(95)
	holder.OwnerAggregationApplied = false
	holder.Rows = []HolderIntelligenceRow{
		{Rank: 1, RawPercentage: 95, OwnerResolved: false, RiskBearing: true},
	}
	behavior := UnifiedRadarBehaviorReport{Mint: "RawAccountMint", Signals: []UnifiedRadarSignal{}, Evidence: []ActorDefenseEvidenceRecord{}, GeneratedAt: now}
	behavior = ApplyOwnerConcentrationRuleV110(behavior, holder, now)
	signal := behavior.Signals[0]

	if signal.Triggered || signal.EvidenceStatus != "unverified" || signal.GradeEffect != "none" {
		t.Fatalf("raw account concentration triggered C005: %#v", signal)
	}
	verdict := EvaluateUnifiedRadarVerdictV110("RawAccountMint", ActorDefenseRuleVerdict{}, behavior)
	if verdict.Grade != "-" || verdict.Signed {
		t.Fatalf("raw account verdict=%#v", verdict)
	}
}

func TestC005ExcludedInfrastructureOwnerCannotTrigger(t *testing.T) {
	now := time.Date(2026, 7, 17, 6, 0, 0, 0, time.UTC)
	holder := c005Holder(88)
	holder.Rows[0].ExcludedFromHolderRisk = true
	behavior := UnifiedRadarBehaviorReport{Mint: "InfraMint", Signals: []UnifiedRadarSignal{}, Evidence: []ActorDefenseEvidenceRecord{}, GeneratedAt: now}
	behavior = ApplyOwnerConcentrationRuleV110(behavior, holder, now)

	if behavior.Signals[0].Triggered || behavior.Signals[0].EvidenceStatus != "unverified" {
		t.Fatalf("infrastructure owner triggered C005: %#v", behavior.Signals[0])
	}
}
