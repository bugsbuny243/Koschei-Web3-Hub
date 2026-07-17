package services

import "testing"

func TestInvestigationCoverageDoesNotCountArchitectureAsEvidence(t *testing.T) {
	arms := []SecurityRadarVerdict{
		{Module: "Authority", ModuleID: ModuleTokenAuthorityScanner, Signed: true, Evidence: []string{"authority parsed"}, Signals: map[string]any{"execution_status": ArvisExecutionCompleted, "evidence_status": "verified", "finding_observed": true}},
		{Module: "MEV", ModuleID: ModuleMEVShield, Signed: false, Evidence: []string{"transaction required"}, Signals: map[string]any{"evidence_status": "insufficient_evidence"}},
		{Module: "Liquidity", ModuleID: ModuleLiquidityMovement, Signed: false, Evidence: []string{"pool unresolved"}, Signals: map[string]any{"execution_status": ArvisExecutionEvidencePending, "evidence_status": "insufficient_evidence"}},
	}
	coverage := BuildArvisInvestigationCoverage(arms)
	if coverage.CapabilityTotal != 3 || coverage.EvidenceProducing != 1 {
		t.Fatalf("coverage=%#v", coverage)
	}
	if coverage.NotApplicable != 1 {
		t.Fatalf("MEV mint-level collector should be not applicable: %#v", coverage)
	}
	if coverage.EvidencePending != 1 || coverage.Status != "partial_investigation" {
		t.Fatalf("pending/status=%#v", coverage)
	}
}

func TestLaunchForensicsCompletesLaunchCollectors(t *testing.T) {
	t.Setenv("SOLANA_RPC_URL", "")
	req := SecurityRadarRequest{Target: "MintLaunch111111111111111111111111111111111", Network: "solana-mainnet", Mode: "manual_test"}
	analysis := AnalyzeArvisRadars(req)
	forensics := LaunchForensicsAnalysis{
		Available: true, Status: "verified_launch_forensics", DataSource: "ata_history",
		OwnersRequested: 4, OwnersWithTradeHistory: 4, LedgerTradeCount: 7,
		LaunchSlot: 123, LaunchTime: "2026-07-17T00:00:00Z", SniperCount: 1,
		RhythmBotCount: 1, FlipperCount: 1, AccumulatorCount: 1, CreatorLinkedCount: 1,
		Profiles: []LaunchActorProfile{}, Timeline: []LaunchTimelineEntry{}, Findings: []string{"launch evidence"}, Limitations: []string{},
	}
	analysis = ApplyLaunchForensicsToAnalysis(analysis, req, forensics)
	seen := map[string]SecurityRadarVerdict{}
	for _, arm := range analysis.Arms {
		seen[arm.ModuleID] = arm
	}
	for _, moduleID := range []string{ModulePumpSybilRadar, ModuleLaunchDistribution, ModuleSniperTimingDetector} {
		arm, ok := seen[moduleID]
		if !ok || !arm.Signed || arvisSignalString(arm.Signals, "execution_status") != ArvisExecutionCompleted {
			t.Fatalf("collector %s=%#v", moduleID, arm)
		}
	}
}

func TestPrimaryPairOutsideRaydiumIsNotApplicable(t *testing.T) {
	t.Setenv("SOLANA_RPC_URL", "")
	req := SecurityRadarRequest{Target: "MintPool11111111111111111111111111111111111", Network: "solana-mainnet", Mode: "manual_test"}
	analysis := AnalyzeArvisRadars(req)
	market := TokenMarketSnapshot{
		Available: true, Provider: "test", Mint: req.Target, LiquidityUSD: 100000,
		BestPairAddress: "Pair111111111111111111111111111111111111", BestPairDEX: "meteora",
		BestPairLiquidityUSD: 100000, BestPairVolume24hUSD: 50000, PairCount: 1,
	}
	analysis = ApplyCreatorAndLiquidityEvidenceToAnalysis(analysis, req, "Creator111111111111111111111111111111111", market, LaunchForensicsAnalysis{})
	for _, arm := range analysis.Arms {
		if arm.ModuleID != ModuleRaydiumPoolGuardian {
			continue
		}
		if status := arvisSignalString(arm.Signals, "execution_status"); status != ArvisExecutionNotApplicable {
			t.Fatalf("raydium status=%q arm=%#v", status, arm)
		}
		return
	}
	t.Fatal("raydium arm missing")
}

func TestRepeatActorQueryWithoutMatchIsCompleted(t *testing.T) {
	t.Setenv("SOLANA_RPC_URL", "")
	req := SecurityRadarRequest{Target: "MintRepeat111111111111111111111111111111111", Network: "solana-mainnet", Mode: "manual_test"}
	analysis := AnalyzeArvisRadars(req)
	analysis = ApplyRepeatDominantHolderEvidenceToAnalysis(analysis, req, nil)
	for _, arm := range analysis.Arms {
		if arm.ModuleID != ModuleRepeatActorScan {
			continue
		}
		if !arm.Signed || arvisSignalString(arm.Signals, "execution_status") != ArvisExecutionCompleted {
			t.Fatalf("repeat arm=%#v", arm)
		}
		if arvisSignalBool(arm.Signals, "finding_observed") {
			t.Fatalf("no-match query marked as finding: %#v", arm.Signals)
		}
		return
	}
	t.Fatal("repeat actor arm missing")
}

func TestCoverageMetadataIsAttachedAfterExtensions(t *testing.T) {
	t.Setenv("SOLANA_RPC_URL", "")
	req := SecurityRadarRequest{Target: "MintCoverage11111111111111111111111111111111", Network: "solana-mainnet", Mode: "manual_test"}
	analysis := AnalyzeArvisRadars(req)
	analysis = ApplyLaunchForensicsToAnalysis(analysis, req, LaunchForensicsAnalysis{})
	analysis = ApplyCreatorAndLiquidityEvidenceToAnalysis(analysis, req, "", TokenMarketSnapshot{}, LaunchForensicsAnalysis{})
	raw, ok := analysis.Bundle.Metadata["investigation_coverage"]
	if !ok {
		t.Fatal("investigation coverage metadata missing")
	}
	coverage, ok := raw.(ArvisInvestigationCoverage)
	if !ok || coverage.CapabilityTotal != 14 {
		t.Fatalf("coverage metadata=%#v", raw)
	}
}
