package services

import (
	"testing"
	"time"
)

func TestHardenUnifiedCreatorSellRemainsObservedWhenRouteThresholdsAreSupported(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	report := UnifiedRadarBehaviorReport{
		Mint: "MintOne",
		Signals: []UnifiedRadarSignal{{
			RuleID: UnifiedRuleCreatorSellAcceleration, Title: "Creator sell acceleration",
			EvidenceStatus: "verified", Triggered: true, GradeEffect: "compounding_input",
			Summary: "Creator produced 3 verified sells.", Metrics: map[string]any{},
			Signatures: []string{"ledger-signature-one", "ledger-signature-two"}, ObservedAt: now,
		}},
	}
	verification := CreatorSellVerification{
		CandidateSignatures:      []string{"ledger-signature-one", "ledger-signature-two"},
		VerifiedSignatures:       []string{"ledger-signature-one", "ledger-signature-two"},
		RouteAttributedSellCount: 2,
		RouteAttributedSellSOL:   1.5,
		TransactionsParsed:       2,
	}
	got := HardenUnifiedRadarBehavior(report, verification, HolderClusterAnalysis{})
	if len(got.Signals) != 1 {
		t.Fatalf("signals=%d", len(got.Signals))
	}
	signal := got.Signals[0]
	if signal.EvidenceStatus != "observed" || !signal.Triggered || signal.GradeEffect != "compounding_input" {
		t.Fatalf("creator sell hardening=%#v", signal)
	}
	if len(signal.Signatures) != 2 || len(signal.EvidenceKeys) != 2 {
		t.Fatalf("verified support refs=%#v", signal)
	}
	if signal.Metrics["route_attributed_sell_count"] != 2 || signal.Metrics["route_attributed_sell_sol"] != 1.5 {
		t.Fatalf("route metrics=%#v", signal.Metrics)
	}
	if len(got.Evidence) != 0 {
		t.Fatalf("ledger-derived acceleration must not create actor evidence rows: %#v", got.Evidence)
	}
}

func TestHardenUnifiedCreatorSellWithdrawsWithoutRouteThresholdSupport(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	report := UnifiedRadarBehaviorReport{
		Mint: "MintOne",
		Signals: []UnifiedRadarSignal{{
			RuleID: UnifiedRuleCreatorSellAcceleration, Title: "Creator sell acceleration",
			EvidenceStatus: "observed", Triggered: true, GradeEffect: "compounding_input",
			Summary: "Creator one-hour sell flow accelerated.", Metrics: map[string]any{},
			Signatures: []string{"ledger-one", "ledger-two"}, ObservedAt: now,
		}},
	}
	verification := CreatorSellVerification{
		CandidateSignatures:      []string{"ledger-one", "ledger-two"},
		VerifiedSignatures:       []string{"ledger-one"},
		RouteAttributedSellCount: 1,
		RouteAttributedSellSOL:   0.8,
		TransactionsParsed:       2,
	}
	got := HardenUnifiedRadarBehavior(report, verification, HolderClusterAnalysis{})
	signal := got.Signals[0]
	if signal.Triggered || signal.GradeEffect != "none" {
		t.Fatalf("unsupported C003 remained grade-changing: %#v", signal)
	}
	if got.TriggeredRuleCount != 0 {
		t.Fatalf("triggered rule count=%d", got.TriggeredRuleCount)
	}
	if len(signal.Signatures) != 1 || signal.Signatures[0] != "ledger-one" {
		t.Fatalf("verified signatures=%v", signal.Signatures)
	}
}

func TestHardenUnifiedDominantExitRequiresCompleteOutboundEvidenceLine(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	report := UnifiedRadarBehaviorReport{
		Mint: "MintOne",
		Signals: []UnifiedRadarSignal{{
			RuleID: UnifiedRuleDominantHolderFirstExit, Title: "Dominant holder exit",
			EvidenceStatus: "verified", Triggered: true, GradeEffect: "compounding_input",
			Scope: "earliest_verified_exit_in_bounded_window", Summary: "exit",
			Metrics:      map[string]any{"slot": int64(99), "amount": 1000.0},
			EvidenceKeys: []string{"dominant-holder-exit:sig-one"},
			Signatures:   []string{"sig-one"}, ObservedAt: now,
		}},
	}
	cluster := HolderClusterAnalysis{Wallets: []HolderClusterWallet{{
		Rank: 1, Wallet: "HolderOne",
		FlowObservations: []HolderClusterFlowObservation{{
			SourceWallet: "HolderOne", Destination: "PoolOne", Direction: "outbound", Amount: 1000,
			Slot: 99, Signature: "sig-one", ProgramIDs: []string{"DexProgramOne"},
		}},
	}}}
	got := HardenUnifiedRadarBehavior(report, CreatorSellVerification{}, cluster)
	if got.Signals[0].EvidenceStatus != "verified" || !got.Signals[0].Triggered {
		t.Fatalf("exit signal=%#v", got.Signals[0])
	}
	if got.Signals[0].Metrics["direction"] != "outbound" {
		t.Fatalf("direction=%v", got.Signals[0].Metrics["direction"])
	}
	if len(got.Evidence) != 1 {
		t.Fatalf("evidence rows=%d", len(got.Evidence))
	}
	line := BuildActorDefenseEvidenceLine(got.Evidence[0])
	if !line.EvidenceLineComplete {
		t.Fatalf("canonical line gaps=%v", line.EvidenceGaps)
	}
	if line.SourceWallet != "HolderOne" || line.DestinationWallet != "PoolOne" || line.Program != "DexProgramOne" {
		t.Fatalf("canonical line=%#v", line)
	}
}

func TestHardenUnifiedDominantExitRejectsInboundHolderObservation(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	report := UnifiedRadarBehaviorReport{
		Mint: "MintOne",
		Signals: []UnifiedRadarSignal{{
			RuleID: UnifiedRuleDominantHolderFirstExit, Title: "Dominant holder exit",
			EvidenceStatus: "verified", Triggered: true, GradeEffect: "compounding_input",
			Scope: "earliest_verified_exit_in_bounded_window", Summary: "candidate exit",
			Metrics:      map[string]any{"holder_wallet": "HDix", "slot": int64(99), "amount": 0.003123},
			EvidenceKeys: []string{"dominant-holder-exit:inbound-sig"},
			Signatures:   []string{"inbound-sig"}, ObservedAt: now,
		}},
	}
	cluster := HolderClusterAnalysis{Wallets: []HolderClusterWallet{{
		Rank: 1, Wallet: "HDix",
		FlowObservations: []HolderClusterFlowObservation{{
			SourceWallet: "4pta", Destination: "HDix", Direction: "inbound", Kind: "inbound_token_sender_context",
			Amount: 0.003123, Slot: 99, Signature: "inbound-sig", ProgramIDs: []string{"TokenProgram"},
		}},
	}}}
	got := HardenUnifiedRadarBehavior(report, CreatorSellVerification{}, cluster)
	signal := got.Signals[0]
	if signal.Triggered {
		t.Fatalf("inbound holder observation triggered URD-C004: %#v", signal)
	}
	if signal.GradeEffect != "none" || signal.EvidenceStatus != "observed" {
		t.Fatalf("inbound downgrade=%#v", signal)
	}
	if len(signal.Signatures) != 0 || len(signal.EvidenceKeys) != 0 || len(got.Evidence) != 0 {
		t.Fatalf("inbound observation retained grade-changing evidence: signal=%#v evidence=%#v", signal, got.Evidence)
	}
}

func TestHardenUnifiedDominantExitDowngradesIncompleteLine(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	report := UnifiedRadarBehaviorReport{
		Mint: "MintOne",
		Signals: []UnifiedRadarSignal{{
			RuleID: UnifiedRuleDominantHolderFirstExit, EvidenceStatus: "verified", Triggered: true,
			EvidenceKeys: []string{"dominant-holder-exit:sig-one"}, Signatures: []string{"sig-one"},
			Metrics: map[string]any{"slot": int64(99), "amount": 1000.0}, ObservedAt: now,
		}},
	}
	cluster := HolderClusterAnalysis{Wallets: []HolderClusterWallet{{
		Rank: 1, Wallet: "HolderOne",
		FlowObservations: []HolderClusterFlowObservation{{
			SourceWallet: "HolderOne", Destination: "", Direction: "outbound", Amount: 1000,
			Slot: 99, Signature: "sig-one", ProgramIDs: []string{},
		}},
	}}}
	got := HardenUnifiedRadarBehavior(report, CreatorSellVerification{}, cluster)
	if got.Signals[0].Triggered {
		t.Fatalf("incomplete exit remained triggered=%#v", got.Signals[0])
	}
	if len(got.Evidence) != 0 {
		t.Fatalf("incomplete exit produced evidence=%#v", got.Evidence)
	}
}
