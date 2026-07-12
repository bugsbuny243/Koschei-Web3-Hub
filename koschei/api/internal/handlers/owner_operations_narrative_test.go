package handlers

import (
	"strings"
	"testing"

	"koschei/api/internal/services"
)

func TestOwnerRadarNarrativeExplainsMeaning(t *testing.T) {
	final := map[string]any{"risk_index": 53, "risk_level": "medium", "signed": true}
	warning := map[string]any{"positive_signals": []string{"Mint authority kapalı/revoked olarak gözlendi.", "Freeze authority kapalı/revoked olarak gözlendi."}}
	distribution := map[string]any{
		"available": true, "role_adjusted": true, "blocking_evidence_gap": false,
		"top_1_percentage": 2.4774, "top_10_percentage": 14.6475, "top_20_percentage": 22.108,
		"protocol_controlled_percentage": 2.0999, "dominant_role": "externally_owned_wallet",
	}
	modules := []map[string]any{{
		"module": "Sniper Timing Detector", "module_id": "sniper_timing_detector",
		"risk_index": 53, "risk_level": "medium", "verified": true, "signed": true,
		"verdict": "Ardışık slotlarda kümelenen alımlar ek inceleme gerektiriyor.",
	}}
	holder := services.HolderIntelligence{Available: true, Findings: []string{"En büyük owner bakiyesi ve referans USD değeri doğrulandı."}}
	text := ownerRadarNarrative("4ko5tSr5o3H4v1sFtjTSd9MPUW7yx5AFCpkNPoL6pump", final, warning, distribution, map[string]any{}, modules, holder)
	for _, expected := range []string{
		"53/100 ile ORTA risk seviyesinde", "tek başına ciddi bir balina", "Olumlu sinyaller:",
		"ana risk sürücüsü Sniper Timing Detector", "Creator/deployer cüzdanı bu taramada doğrulanamadı",
		"En büyük owner bakiyesi", "Pratik sonuç:",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in narrative: %s", expected, text)
		}
	}
	if strings.Contains(text, "ARVIS kararı: MEDIUM, risk") {
		t.Fatalf("machine-like legacy summary returned: %s", text)
	}
}

func TestOwnerRadarNarrativeEvidencePendingStillExplainsKnownHoldings(t *testing.T) {
	usd := 4971180.0
	holder := services.HolderIntelligence{
		Available: true, Supply: 1000000000, OwnerCount: 20, RiskBearingOwnerCount: 19,
		TopOwnerPercentage: 99.4236, WalletsWithObservedOutflow: 2, CommonExitGroupCount: 1,
		Market: services.TokenMarketSnapshot{Available: true, PriceUSD: 0.005, Volume24hUSD: 800000, LiquidityUSD: 200000, MarketCapUSD: 5000000},
		Rows: []services.HolderIntelligenceRow{{TokenAccounts: []string{"DominantTokenAccount"}, Balance: 994236000, RawPercentage: 99.4236, Role: "owner_unresolved", ReferenceUSDValue: &usd}},
	}
	warning := map[string]any{"reasons": []string{"Baskın token hesabının ekonomik rolü çözülemedi."}}
	text := ownerRadarNarrative("target", map[string]any{"risk_index": nil, "risk_level": "unknown", "signed": false}, warning, map[string]any{}, map[string]any{}, nil, holder)
	for _, expected := range []string{"EVIDENCE PENDING", "994236000", "99.4236%", "$4971180.00", "2 holder wallet", "1 ortak recipient-owner"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in pending narrative: %s", expected, text)
		}
	}
	if strings.Contains(text, "elde veri olmadığı") && !strings.Contains(text, "bu, elde veri olmadığı") {
		t.Fatalf("pending narrative hid known facts: %s", text)
	}
}
