package cryptobrief

import (
	"strings"
	"testing"
)

func TestFormatARVISResultMirrorsCanonicalFields(t *testing.T) {
	body := FormatARVISResult("target-1", "solana-mainnet", map[string]any{
		"status":            "ready",
		"has_live_evidence": true,
		"final_verdict": map[string]any{
			"verdict":        "withhold",
			"grade":          "C",
			"risk_level":     "high",
			"recommendation": "Do not proceed until the flagged authority evidence is reviewed.",
		},
	})
	for _, want := range []string{
		"ARVIS taraması tamamlandı",
		"Hedef: target-1",
		"Ağ: solana-mainnet",
		"Sonuç: withhold",
		"Risk: HIGH",
		"ARVIS derece: C",
		"Kanıt: doğrulanmış/canlı kanıt mevcut",
		"Do not proceed until the flagged authority evidence is reviewed.",
		"https://tradepigloball.co/dashboard",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("formatted ARVIS Telegram result missing %q: %s", want, body)
		}
	}
}

func TestFormatARVISResultNeverConvertsMissingEvidenceToSafe(t *testing.T) {
	body := FormatARVISResult("target-2", "ethereum-mainnet", map[string]any{
		"status":            "withheld",
		"has_live_evidence": false,
		"final_verdict": map[string]any{
			"verdict": "unknown",
		},
	})
	if !strings.Contains(body, "Kanıt: eksik veya doğrulanmamış — güvenli kabul edilmez") {
		t.Fatalf("missing-evidence warning absent: %s", body)
	}
	lower := strings.ToLower(body)
	for _, forbidden := range []string{"güvenli ✅", "safe ✅", "risk yok", "risksiz"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("missing evidence was upgraded to safety with %q: %s", forbidden, body)
		}
	}
}

func TestFormatARVISResultRejectsNilEnvelope(t *testing.T) {
	if got := FormatARVISResult("target", "solana-mainnet", nil); got != "" {
		t.Fatalf("nil canonical result should not create Telegram content: %q", got)
	}
}
