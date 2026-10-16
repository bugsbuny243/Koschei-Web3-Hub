package handlers

import (
	"strings"
	"testing"
)

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

func TestARVISPreflightOfficialMintNeverBypassesCriticalClaimInstructions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		intent string
		note   string
		score  int
	}{
		{name: "seed_request", intent: "official reward claim", note: "enter your seed phrase", score: 98},
		{name: "unlimited_approval", intent: "approve all", note: "official claim", score: 94},
		{name: "delegate_change", intent: "setauthority", note: "airdrop claim", score: 94},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := arvisPreflightRequest{Target: officialKOSCMint, Kind: "token", Intent: tc.intent, Note: tc.note}
			out := applyARVISPreflightScope(req, evaluateARVISPreflight(req))
			if out.Decision != "blocked" || out.RiskLevel != "high" || out.Score < tc.score {
				t.Fatalf("official token mint must not override unsafe signing instructions: %+v", out)
			}
			if len(out.NotChecked) == 0 || out.CoverageWarning == "" {
				t.Fatalf("official mint must retain structural scan limitations: %+v", out)
			}
		})
	}
}

func TestARVISPreflightOfficialMintWithoutClaimStillNeedsReview(t *testing.T) {
	req := arvisPreflightRequest{Target: officialKOSCMint, Kind: "token", Intent: "identify mint"}
	out := applyARVISPreflightScope(req, evaluateARVISPreflight(req))
	if out.Decision != "review" || out.RiskLevel == "low" ||
		out.Score >= 90 || len(out.NotChecked) == 0 {
		t.Fatalf("official mint is not proof of full wallet or liquidity safety: %+v", out)
	}
}

func TestARVISPreflightClaimURLNeverCertifiesPageOrSignedTransaction(t *testing.T) {
	for _, req := range []arvisPreflightRequest{
		{Target: "https://unknown.example/claim", Kind: "url", Intent: "airdrop"},
		{Target: "https://tradepigloball.co/claim", Kind: "url", Intent: "connect wallet"},
		{Target: "javascript://tradepigloball.co/claim", Kind: "site", Intent: "sign"},
	} {
		out := applyARVISPreflightScope(req, evaluateARVISPreflight(req))
		if out.Scope != "preflight_only" || len(out.NotChecked) < 4 || out.CoverageWarning == "" ||
			out.Decision == "allow" || out.RiskLevel == "low" {
			t.Fatalf("lexical URL analysis must not imply site or transaction safety: %+v", out)
		}
		if !strings.Contains(out.CoverageWarning, "Sayfa açılmadı") ||
			!strings.Contains(out.CoverageWarning, "imzalanacak işlem incelenmedi") {
			t.Fatalf("claim URL coverage limits not exposed: %+v", out)
		}
	}
}

func TestARVISPreflightRejectsNonWebAndCredentialedOriginTrust(t *testing.T) {
	if arvisPreflightHost("javascript://tradepigloball.co/claim") != "" ||
		arvisPreflightHost("file://tradepigloball.co/claim") != "" {
		t.Fatal("non-web protocol must not be recognized as a trusted website")
	}
	req := arvisPreflightRequest{Target: "https://login@tradepigloball.co/claim", Kind: "url", Intent: "connect wallet"}
	out := evaluateARVISPreflight(req)
	if out.Decision != "warn" || out.Score < 85 {
		t.Fatalf("userinfo in claim URL must not get a trusted decision: %+v", out)
	}
	for _, reason := range out.Reasons {
		if strings.Contains(reason, "Host adı Koschei alan adı") {
			t.Fatalf("credentialed URL incorrectly received a trusted-host reason: %+v", out)
		}
	}
}
