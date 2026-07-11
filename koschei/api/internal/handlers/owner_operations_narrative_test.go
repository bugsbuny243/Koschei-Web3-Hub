package handlers

import (
	"strings"
	"testing"
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
	text := ownerRadarNarrative("4ko5tSr5o3H4v1sFtjTSd9MPUW7yx5AFCpkNPoL6pump", final, warning, distribution, map[string]any{}, modules)
	for _, expected := range []string{
		"53/100 ile ORTA risk seviyesinde", "tek başına ciddi bir balina", "Olumlu sinyaller:",
		"ana risk sürücüsü Sniper Timing Detector", "Creator/deployer cüzdanı bu taramada doğrulanamadı",
		"Pratik sonuç:",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in narrative: %s", expected, text)
		}
	}
	if strings.Contains(text, "ARVIS kararı: MEDIUM, risk") {
		t.Fatalf("machine-like legacy summary returned: %s", text)
	}
}

func TestOwnerRadarNarrativeEvidencePending(t *testing.T) {
	text := ownerRadarNarrative("target", map[string]any{"risk_index": nil, "risk_level": "unknown", "signed": false}, map[string]any{}, map[string]any{}, map[string]any{}, nil)
	if !strings.Contains(text, "EVIDENCE PENDING") || strings.Contains(text, "/100") {
		t.Fatalf("unexpected pending narrative: %s", text)
	}
}
