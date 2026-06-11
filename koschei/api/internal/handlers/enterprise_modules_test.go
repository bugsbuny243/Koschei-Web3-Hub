package handlers

import "testing"

func TestBuildMEVRiskReportHighRisk(t *testing.T) {
	report := buildMEVRiskReport(mevAnalyzeRequest{InputAmountUSD: 25_000, SlippageBPS: 350, PoolLiquidityUSD: 500_000})
	if report.RiskScore < 70 {
		t.Fatalf("RiskScore = %d, want high risk >= 70", report.RiskScore)
	}
	if report.RiskLevel != "YÜKSEK" {
		t.Fatalf("RiskLevel = %q, want YÜKSEK", report.RiskLevel)
	}
	if report.EstimatedLossUSD <= 0 || !report.JitoTipUsed || report.MEVSavedUSD != report.EstimatedLossUSD {
		t.Fatalf("unexpected MEV economics: %+v", report)
	}
}

func TestLiquidityDrainScoreCritical(t *testing.T) {
	score := liquidityDrainScore(liquidityRadarRequest{ReserveDropPct: 55, RemovedLiquidity: 75_000, BlockDelay: 1})
	if score < 90 {
		t.Fatalf("liquidityDrainScore() = %d, want >= 90", score)
	}
}

func TestDAOProposalRiskScoreOutflow(t *testing.T) {
	score := daoProposalRiskScore(daoProposalRiskRequest{EstimatedOutflowUSD: 250_000, SignerCount: 6, RequiredSigners: 2, Instructions: []string{"transfer treasury", "set_authority"}})
	if score < 75 {
		t.Fatalf("daoProposalRiskScore() = %d, want >= 75", score)
	}
}
