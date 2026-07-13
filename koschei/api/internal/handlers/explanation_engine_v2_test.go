package handlers

import (
	"strings"
	"testing"

	"koschei/api/internal/services"
)

func explanationFixture(top1, top10 float64) scanExplanationInput {
	usd := 22900.0
	return scanExplanationInput{
		Target: "Mint", RiskIndex: 75, RiskLevel: "critical", Signed: true, Policy: "evidence_backed",
		Distribution: map[string]any{"top_1_percentage": top1, "top_10_percentage": top10, "protocol_controlled_percentage": 5.0},
		Holder:       services.HolderIntelligence{Available: true, Top1Percentage: top1, Top10Percentage: top10, RiskBearingOwnerCount: 4, WalletsWithParsedEvidence: 4, Market: services.TokenMarketSnapshot{Available: true, LiquidityUSD: 14400}, Rows: []services.HolderIntelligenceRow{{OwnerWallet: "GV6UUmNxxVdC52", OwnerResolved: true, RiskBearing: true, CirculatingPercentage: top1, ReferenceUSDValue: &usd}}},
		Launch:       services.LaunchForensicsAnalysis{OwnersRequested: 4, OwnersWithTradeHistory: 4},
		Modules:      []map[string]any{{"module": "Holder Concentration", "module_id": "holder_concentration", "risk_index": 70, "verified": true, "signed": true, "signals": map[string]any{}}},
	}
}

func TestExplanationV2UsesDifferentLeadFamilies(t *testing.T) {
	critical := buildScanExplanationV2(explanationFixture(78.67, 94))
	coordinatedInput := explanationFixture(18, 55)
	coordinatedInput.Cluster = services.HolderClusterAnalysis{Available: true, RiskIndex: 66, SynchronizedWalletCount: 3, SharedFundingGroupCount: 1, LinkedHolderPercentage: 42}
	coordinated := buildScanExplanationV2(coordinatedInput)
	cleanInput := explanationFixture(8, 35)
	cleanInput.RiskIndex = 18
	cleanInput.RiskLevel = "low"
	cleanInput.Modules = []map[string]any{{"module": "Authority", "module_id": "token_authority_scanner", "risk_index": 5, "verified": true, "signed": true, "signals": map[string]any{"mint_authority_present": false, "freeze_authority_present": false}}}
	clean := buildScanExplanationV2(cleanInput)
	insufficientInput := explanationFixture(40, 80)
	insufficientInput.Signed = false
	insufficientInput.Policy = "withhold"
	insufficient := buildScanExplanationV2(insufficientInput)
	classes := map[string]bool{critical.CaseClass: true, coordinated.CaseClass: true, clean.CaseClass: true, insufficient.CaseClass: true}
	for _, expected := range []string{"critical_concentration", "coordinated_cluster", "clean_distributed", "insufficient_evidence"} {
		if !classes[expected] {
			t.Fatalf("missing class %s: %#v", expected, classes)
		}
	}
	if critical.Lead == coordinated.Lead || coordinated.Lead == clean.Lead || clean.Lead == insufficient.Lead {
		t.Fatalf("template leads did not vary")
	}
	if len(clean.Text)*2 >= len(critical.Text) {
		t.Fatalf("clean explanation must be under half critical length: clean=%d critical=%d", len(clean.Text), len(critical.Text))
	}
}

func TestExplanationV2CollapsesSparseForensicsIntoOneLimitsLine(t *testing.T) {
	in := explanationFixture(12, 45)
	in.Signed = false
	in.Policy = "withhold"
	in.Launch = services.LaunchForensicsAnalysis{OwnersRequested: 19, OwnersWithTradeHistory: 1, SniperCount: 0}
	got := buildScanExplanationV2(in)
	if strings.Count(got.Limits, "Sınırlar:") != 1 || !strings.Contains(got.Limits, "19 owner'dan yalnızca 1'i") || !strings.Contains(got.Limits, "0 sniper sonucu") {
		t.Fatalf("limits=%s", got.Limits)
	}
	if strings.Contains(got.Text, "Olumlu sinyaller") {
		t.Fatalf("boilerplate leaked: %s", got.Text)
	}
}

func TestExplanationV2RepeatDominantHolderOwnsLead(t *testing.T) {
	in := explanationFixture(78.67, 95)
	matches := []services.RepeatDominantHolderMatch{{Mint: "9cRCn9rGT8V2imeM2BaKs13yhMEais3ruM3rPvTGpump", Percentage: 58.71, ScannedAt: "2026-07-12T10:00:00Z"}, {Mint: "6QPvGr1L7aXGybpGKvvG8LtFDV9dRzK6QbSpRNJJonYM", Percentage: 78.67, ScannedAt: "2026-07-13T10:00:00Z"}}
	line := services.RepeatDominantEvidenceLine("GV6UUmNxxVdC52", matches, 30)
	in.RepeatDominant = []services.RepeatDominantHolderEvidence{{OwnerWallet: "GV6UUmNxxVdC52", CurrentPercentage: 78.67, TokenCount: 2, RiskWeight: 76, Matches: matches, EvidenceLine: line}}
	got := buildScanExplanationV2(in)
	if got.DominantDriver != "repeat_dominant_holder" || !strings.HasPrefix(got.Lead, "REPEAT DOMINANT HOLDER") || !strings.Contains(got.Lead, "son 30 gün") {
		t.Fatalf("lead=%s driver=%s", got.Lead, got.DominantDriver)
	}
}
