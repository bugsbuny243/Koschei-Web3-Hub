package handlers

import (
	"strings"
	"testing"

	"koschei/api/internal/services"
)

func TestOwnerRadarNarrativeUsesDriverFirstV2(t *testing.T) {
	usd := 16000.0
	holder := services.HolderIntelligence{Available: true, Top1Percentage: 62, Top10Percentage: 88, RiskBearingOwnerCount: 5, WalletsWithParsedEvidence: 3, Market: services.TokenMarketSnapshot{LiquidityUSD: 10000}, Rows: []services.HolderIntelligenceRow{{OwnerWallet: "WalletA", OwnerResolved: true, RiskBearing: true, ReferenceUSDValue: &usd}}}
	modules := []map[string]any{{"module": "Holder Concentration", "module_id": "holder_concentration", "risk_index": 70, "verified": true, "signed": true, "signals": map[string]any{}}}
	text := ownerRadarNarrative("target", map[string]any{"risk_index": 70, "risk_level": "high", "signed": true}, map[string]any{}, map[string]any{"top_1_percentage": 62.0, "top_10_percentage": 88.0}, map[string]any{}, modules, holder, services.LaunchForensicsAnalysis{OwnersRequested: 5, OwnersWithTradeHistory: 3})
	if !strings.HasPrefix(text, "WalletA") || !strings.Contains(text, "62.00%") || !strings.Contains(text, "havuzun ~1.6 katı") {
		t.Fatalf("narrative=%s", text)
	}
	for _, forbidden := range []string{"Koschei bu tokenı", "Olumlu sinyaller:", "Pratik sonuç:"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("template phrase %q leaked: %s", forbidden, text)
		}
	}
}

func TestOwnerRadarNarrativePendingKeepsKnownNumbersAndOneLimitsParagraph(t *testing.T) {
	holder := services.HolderIntelligence{Available: true, Top1Percentage: 99.42, Top10Percentage: 100, FinalVerdictBlocked: true, Rows: []services.HolderIntelligenceRow{{OwnerWallet: "WalletA", OwnerResolved: true, RiskBearing: true}}}
	text := ownerRadarNarrative("target", map[string]any{"risk_index": nil, "risk_level": "unknown", "signed": false}, map[string]any{}, map[string]any{"top_1_percentage": 99.42, "top_10_percentage": 100.0}, map[string]any{}, nil, holder, services.LaunchForensicsAnalysis{OwnersRequested: 19, OwnersWithTradeHistory: 1})
	if !strings.Contains(text, "Top 1 99.42%") || strings.Count(text, "Sınırlar:") != 1 || !strings.Contains(text, "eksik gözlem güvenlik sinyali sayılmadı") {
		t.Fatalf("narrative=%s", text)
	}
}
