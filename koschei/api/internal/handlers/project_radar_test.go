package handlers

import "testing"

func TestNormalizeProjectRadarRequest(t *testing.T) {
	req, err := normalizeProjectRadarRequest(projectRadarRequest{ProjectName: " Demo ", Website: "https://example.org", TwitterHandle: "https://x.com/demo_project?s=1", ChainEcosystem: "Solana", Category: "DePIN"})
	if err != nil {
		t.Fatalf("normalizeProjectRadarRequest() err = %v", err)
	}
	if req.TwitterHandle != "demo_project" {
		t.Fatalf("TwitterHandle = %q, want demo_project", req.TwitterHandle)
	}
	if req.Category != "depin" {
		t.Fatalf("Category = %q, want depin", req.Category)
	}
}

func TestNormalizeProjectRadarRequestRejectsUnsupportedCategory(t *testing.T) {
	_, err := normalizeProjectRadarRequest(projectRadarRequest{ProjectName: "Demo", Website: "https://example.org", TwitterHandle: "demo", ChainEcosystem: "Solana", Category: "memecoin"})
	if err == nil {
		t.Fatal("normalizeProjectRadarRequest() accepted unsupported category")
	}
}

func TestBuildProjectRadarResultAlwaysNeedsManualReview(t *testing.T) {
	req, err := normalizeProjectRadarRequest(projectRadarRequest{ProjectName: "Demo", Website: "https://example.org", TwitterHandle: "@demo", ChainEcosystem: "Solana", Category: "security"})
	if err != nil {
		t.Fatalf("normalizeProjectRadarRequest() err = %v", err)
	}
	result := buildProjectRadarResult(req, projectRadarTokenSignals{Hints: []string{"No token mint provided; token-specific risk hints need manual review."}}, projectRadarWalletSignals{Hints: []string{"No wallet address provided; wallet reputation hints need manual review."}})
	if !result.NeedsManualReview {
		t.Fatal("NeedsManualReview = false, want true")
	}
	if result.RiskScore < 0 || result.RiskScore > 100 || result.OpportunityScore < 0 || result.OpportunityScore > 100 || result.PublicGoodScore < 0 || result.PublicGoodScore > 100 {
		t.Fatalf("scores out of bounds: %#v", result)
	}
	if result.Disclaimer == "" {
		t.Fatal("Disclaimer is empty")
	}
}

func TestContainsSecretPhraseRejectsExplicitSecretMaterial(t *testing.T) {
	if !containsSecretPhrase("please scan this private key") {
		t.Fatal("containsSecretPhrase() = false, want true")
	}
}
