package handlers

import "testing"

func TestARVISPreflightBlocksMissingTarget(t *testing.T) {
	got := evaluateARVISPreflight(arvisPreflightRequest{})
	if got.Decision != "blocked" || got.RiskLevel != "high" {
		t.Fatalf("got %+v, want blocked high", got)
	}
}

func TestARVISPreflightBlocksRecoveryPhraseRequest(t *testing.T) {
	got := evaluateARVISPreflight(arvisPreflightRequest{Target: "https://example.com", Intent: "asks for recovery phrase"})
	if got.Decision != "blocked" || got.RiskLevel != "critical" || got.Score != 100 {
		t.Fatalf("got %+v, want blocked critical score 100", got)
	}
}

func TestARVISPreflightRaisesRiskForGuarantees(t *testing.T) {
	got := evaluateARVISPreflight(arvisPreflightRequest{Target: "So11111111111111111111111111111111111111112", Kind: "token", Intent: "guaranteed 100x"})
	if got.RiskLevel != "medium" && got.RiskLevel != "high" {
		t.Fatalf("got %+v, want at least medium risk", got)
	}
	if len(got.Reasons) == 0 || len(got.NextSteps) == 0 {
		t.Fatalf("got %+v, want reasons and next steps", got)
	}
}
