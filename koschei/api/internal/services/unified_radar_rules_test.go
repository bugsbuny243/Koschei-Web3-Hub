package services

import (
	"strings"
	"testing"
	"time"
)

func TestUnifiedVolumeLiquidityGapRule(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	report := EvaluateUnifiedRadarBehavior("MintOne", "CreatorOne", TokenMarketSnapshot{
		Available: true, Volume24hUSD: 800000, LiquidityUSD: 100000, ObservedAt: now,
	}, HolderIntelligence{}, HolderClusterAnalysis{}, CreatorSellAcceleration{}, now)
	signal := unifiedSignalByID(t, report.Signals, UnifiedRuleVolumeLiquidityGap)
	if !signal.Triggered || signal.EvidenceStatus != "observed" {
		t.Fatalf("volume/liquidity signal=%#v", signal)
	}
	if signal.Metrics["volume_liquidity_ratio"] != 8.0 {
		t.Fatalf("ratio=%v", signal.Metrics["volume_liquidity_ratio"])
	}
}

func TestUnifiedHolderLiquidityPressureRule(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	value := 60000.0
	report := EvaluateUnifiedRadarBehavior("MintOne", "CreatorOne", TokenMarketSnapshot{
		Available: true, LiquidityUSD: 100000, PriceUSD: 0.01, ObservedAt: now,
	}, HolderIntelligence{
		Available: true, TopOwnerPercentage: 40, TopOwnerBalance: 6000000,
		TopOwnerReferenceUSDValue: &value,
	}, HolderClusterAnalysis{}, CreatorSellAcceleration{}, now)
	signal := unifiedSignalByID(t, report.Signals, UnifiedRuleHolderLiquidityPressure)
	if !signal.Triggered || signal.Metrics["position_liquidity_ratio"] != 0.6 {
		t.Fatalf("holder/liquidity signal=%#v", signal)
	}
}

func TestUnifiedCreatorSellAccelerationRule(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	report := EvaluateUnifiedRadarBehavior("MintOne", "CreatorOne", TokenMarketSnapshot{}, HolderIntelligence{}, HolderClusterAnalysis{}, CreatorSellAcceleration{
		Available: true, Status: "creator_sell_windows_observed", Mint: "MintOne", CreatorWallet: "CreatorOne",
		RecentSellCount: 3, RecentSellSOL: 4, BaselineSellCount: 2, BaselineSellSOL: 6,
		BaselineHourlySellSOL: 1, AccelerationMultiple: 4, Triggered: true,
		Signatures: []string{"sell-signature"}, ObservedAt: now,
	}, now)
	signal := unifiedSignalByID(t, report.Signals, UnifiedRuleCreatorSellAcceleration)
	if !signal.Triggered || signal.EvidenceStatus != "verified" || len(signal.Signatures) != 1 {
		t.Fatalf("creator sell signal=%#v", signal)
	}
}

func TestUnifiedDominantHolderExitUsesBoundedLanguage(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	cluster := HolderClusterAnalysis{Wallets: []HolderClusterWallet{{
		Rank: 1, Wallet: "HolderOne", HolderPercentage: 61.5,
		HistoryExhausted: false, SignaturesObserved: 20, ParsedTransactions: 3,
		FlowObservations: []HolderClusterFlowObservation{{
			SourceWallet: "HolderOne", Destination: "PoolOne", Kind: "dex_program_exit_context",
			Amount: 1000, Slot: 99, Signature: "exit-signature",
		}},
	}}}
	report := EvaluateUnifiedRadarBehavior("MintOne", "CreatorOne", TokenMarketSnapshot{}, HolderIntelligence{}, cluster, CreatorSellAcceleration{}, now)
	signal := unifiedSignalByID(t, report.Signals, UnifiedRuleDominantHolderFirstExit)
	if !signal.Triggered || signal.EvidenceStatus != "verified" {
		t.Fatalf("dominant exit signal=%#v", signal)
	}
	if signal.Scope != "earliest_verified_exit_in_bounded_window" || !strings.Contains(signal.Summary, "not claimed") {
		t.Fatalf("scope/summary=%q %q", signal.Scope, signal.Summary)
	}
}

func TestUnifiedVerdictJoinsActorAndMarketRulesWithoutNumber(t *testing.T) {
	actor := ActorDefenseRuleVerdict{
		Grade: "-", Verdict: "single_observation", RulesetVersion: ActorDefenseRulesetVersion,
		TriggeredRules: []ActorDefenseRuleHit{{
			RuleID: ActorRuleCompoundCreatorReuse, Tier: "compounding", EvidenceStatus: "verified",
			GradeEffect: "compounding_input", Count: 2, Summary: "creator reused",
		}},
	}
	behavior := UnifiedRadarBehaviorReport{Signals: []UnifiedRadarSignal{{
		RuleID: UnifiedRuleVolumeLiquidityGap, Title: "gap", EvidenceStatus: "observed",
		Triggered: true, GradeEffect: "compounding_input", Summary: "gap observed",
	}}}
	verdict := EvaluateUnifiedRadarVerdict("MintOne", actor, behavior)
	if verdict.Grade != "B" || verdict.Verdict != "compounding_rule" || !verdict.Signed {
		t.Fatalf("unified verdict=%#v", verdict)
	}
	if verdict.Signature == "" || strings.Contains(verdict.Signature, "/100") {
		t.Fatalf("invalid signature=%q", verdict.Signature)
	}
}

func TestUnifiedInferredSignalIsWatchOnly(t *testing.T) {
	behavior := UnifiedRadarBehaviorReport{Signals: []UnifiedRadarSignal{{
		RuleID: "URD-W999", Title: "watch", EvidenceStatus: "inferred", Triggered: false,
		GradeEffect: "none", Summary: "watch only",
	}}}
	verdict := EvaluateUnifiedRadarVerdict("MintOne", ActorDefenseRuleVerdict{}, behavior)
	if verdict.Grade != "-" || verdict.Verdict != "watch_only" || verdict.Signed {
		t.Fatalf("watch verdict=%#v", verdict)
	}
}

func unifiedSignalByID(t *testing.T, signals []UnifiedRadarSignal, id string) UnifiedRadarSignal {
	t.Helper()
	for _, signal := range signals {
		if signal.RuleID == id {
			return signal
		}
	}
	t.Fatalf("signal %s not found", id)
	return UnifiedRadarSignal{}
}
