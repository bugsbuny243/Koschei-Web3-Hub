package main

import (
	"os"
	"strings"
	"testing"
)

func TestHomepageKeepsKoscheiWeb3EvidenceFirstProductTruth(t *testing.T) {
	body, err := os.ReadFile("public/index.html")
	if err != nil {
		t.Fatalf("read homepage: %v", err)
	}
	text := string(body)
	for _, required := range []string{
		"Koschei Web3 | Security Validation & Risk Intelligence",
		"See the blind spot.",
		"Solana live production core",
		"Material conclusions stay traceable to evidence; missing evidence stays unknown.",
		"No custody · no private keys",
		"ARVIS is not a generic risk-score machine.",
		"Attack Path",
		"Evidence provenance",
		"Protocol Defense Validation",
		"<span class=\"state building\">Building</span>",
		"Cross-chain Intelligence",
		"<span class=\"state building\">Expansion</span>",
		"STRUCTURE ONLY · NOT LIVE TELEMETRY",
		"Test the defense, not just the address.",
		"/css/koschei-home.css?v=1",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("homepage missing evidence-first product contract %q", required)
		}
	}
	for _, forbidden := range []string{
		"Koschei Web3 | The Universe",
		"Web3 defenses?<br>Who tests them?",
		"data-koschei-home-scan",
		"koschei-universe-v1.css",
		"koschei-home-universe-v2.css",
		"koschei-global-shell.js?v=4",
		"Koschei Web3 | Security World",
		">Security World<",
		"SECURITY WORLD / TOPOLOGY",
		"Koschei ARVIS | Evidence-Backed Web3 Security",
		"NO VALID PROOF = NO SIGNATURE",
		"homepage-score-label",
		"homepage-preflight-v2.js",
		"STATIC HTML + VANILLA JS",
		"ETHEREUM</b><small>LIVE",
		"TRON</b><small>LIVE",
		"100% secure",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("homepage regressed into retired/demo-first or unsupported presentation: found %q", forbidden)
		}
	}
}

func TestHomepageExposesRequiredPaddleDomainReviewPolicies(t *testing.T) {
	body, err := os.ReadFile("public/index.html")
	if err != nil {
		t.Fatalf("read homepage: %v", err)
	}
	text := string(body)
	for _, policyLink := range []string{
		`href="/terms.html">Terms</a>`,
		`href="/privacy.html">Privacy</a>`,
		`href="/refund-policy.html">Refunds</a>`,
	} {
		if !strings.Contains(text, policyLink) {
			t.Fatalf("homepage missing required Paddle domain-review policy link %q", policyLink)
		}
	}
}
