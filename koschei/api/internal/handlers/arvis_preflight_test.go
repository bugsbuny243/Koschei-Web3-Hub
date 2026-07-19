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
	if got.Decision != "blocked" || got.RiskLevel != "high" || got.Score != 98 {
		t.Fatalf("got %+v, want blocked high score 98", got)
	}
}

func TestARVISPreflightMarksAddressCoverageIncomplete(t *testing.T) {
	req := arvisPreflightRequest{Target: "So11111111111111111111111111111111111111112", Kind: "token", Intent: "buy"}
	got := applyARVISPreflightScope(req, evaluateARVISPreflight(req))
	if got.Scope != "preflight_only" || got.CoverageWarning == "" || len(got.NotChecked) < 4 {
		t.Fatalf("coverage contract missing: %+v", got)
	}
	if got.Decision == "allow" || got.RiskLevel == "low" {
		t.Fatalf("address preflight implied safety: %+v", got)
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
