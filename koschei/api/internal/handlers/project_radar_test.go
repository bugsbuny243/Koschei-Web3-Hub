package handlers

import "testing"

func TestNormalizeProjectRadarRequest(t *testing.T) {
	req, err := normalizeProjectRadarRequest(projectRadarRequest{ProjectName: " Demo ", Website: "https://example.org", TwitterHandle: "https://x.com/demo_project?s=1", ChainEcosystem: "Solana", Category: "DePIN"})
	if err != nil {
		t.Fatalf("normalizeProjectRadarRequest() err = %v", err)
	}
	if req.WebsiteURL != "https://example.org" {
		t.Fatalf("WebsiteURL = %q, want https://example.org", req.WebsiteURL)
	}
	if req.TwitterHandle != "demo_project" {
		t.Fatalf("TwitterHandle = %q, want demo_project", req.TwitterHandle)
	}
	if req.Ecosystem != "Solana" {
		t.Fatalf("Ecosystem = %q, want Solana", req.Ecosystem)
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
	req, err := normalizeProjectRadarRequest(projectRadarRequest{
		ProjectName:      "Demo",
		Website:          "https://example.org",
		TwitterHandle:    "@demo",
		ChainEcosystem:   "Solana",
		Category:         "security",
		Description:      "Demo helps builders solve security review needs with public transparency workflows and no-custody safety checks before launch.",
		KnownTraction:    "Open docs and GitHub prototype.",
		TokenMintAddress: "",
	})
	if err != nil {
		t.Fatalf("normalizeProjectRadarRequest() err = %v", err)
	}
	result := buildProjectRadarResult(req)
	if !result.ManualReviewNeeded {
		t.Fatal("ManualReviewNeeded = false, want true")
	}
	for label, score := range map[string]int{
		"risk":        result.RiskScore.Score,
		"opportunity": result.OpportunityScore.Score,
		"public_good": result.PublicGoodScore.Score,
	} {
		if score < 0 || score > 100 {
			t.Fatalf("%s score out of bounds: %d", label, score)
		}
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
