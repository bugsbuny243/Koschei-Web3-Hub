package handlers

import (
	"strings"
	"testing"
)

func TestBuildRiskScanResult(t *testing.T) {
	result := buildRiskScanResult()
	if !result.OK || result.RiskLevel != "review_required" || result.Score != 65 {
		t.Fatalf("buildRiskScanResult() = %#v", result)
	}
	if result.UsedAI || result.UsedFallback {
		t.Fatalf("production flags = used_ai:%t used_fallback:%t", result.UsedAI, result.UsedFallback)
	}
	if len(result.Checklist) == 0 || len(result.RecommendedFixes) == 0 {
		t.Fatal("risk result must include checklist items and recommended fixes")
	}
	if result.Disclaimer != riskDisclaimer {
		t.Fatalf("Disclaimer = %q, want %q", result.Disclaimer, riskDisclaimer)
	}
}

func TestRiskScanSummary(t *testing.T) {
	summary := buildRiskScanResult().summary("example.wallet")
	for _, want := range []string{"Risk scan for example.wallet", "Risk level: review_required", "Score: 65", "Checklist:", "Recommended fixes:", riskDisclaimer} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q: %s", want, summary)
		}
	}
}
