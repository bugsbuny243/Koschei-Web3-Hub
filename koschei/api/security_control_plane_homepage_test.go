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
		"Koschei Web3 | Security Intelligence",
		"Security decisions,",
		"with proof attached.",
		"What is observed stays observed. What is unknown stays unknown.",
		"<strong>Read-only</strong>",
		"<strong>Fail-closed</strong>",
		"<strong>No custody</strong><span>no private keys</span>",
		"Illustrative structure · not live telemetry",
		"UNKNOWN ≠ SAFE",
		"Evidence contract",
		"Every conclusion keeps its boundary.",
		"Observed is not verified",
		"Permission is not safety",
		"Absence is not proof",
		"Coverage without theatre",
		"Production core",
		"LIVE CORE",
		"PROBE READY",
		"/css/koschei-home.css?v=3",
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
