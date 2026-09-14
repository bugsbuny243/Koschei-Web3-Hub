package main

import (
	"os"
	"strings"
	"testing"
)

func TestPrimaryPublicSurfacesAreSourceEnglish(t *testing.T) {
	files := []string{
		"public/owner-production.html",
		"public/scan.html",
		"public/dashboard.html",
	}
	for _, path := range files {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(body)
		if !strings.Contains(text, `<html lang="en">`) {
			t.Errorf("%s is not source-English", path)
		}
		for _, forbidden := range []string{
			"Veriyi yenile",
			"Tam Radar",
			"Taramayı Başlat",
			"Token Tara",
			"Ana Sayfa",
			"Kontrol ediliyor",
			"Giriş",
			"Çıkış",
			"public-solana-scan-tr.js",
		} {
			if strings.Contains(text, forbidden) {
				t.Errorf("%s still contains Turkish product copy %q", path, forbidden)
			}
		}
	}
}

func TestCanonicalInvestigationSurfaceMountsProfessionalModesAndEvidenceControllers(t *testing.T) {
	body, err := os.ReadFile("public/scan.html")
	if err != nil {
		t.Fatalf("read canonical investigation page: %v", err)
	}
	text := string(body)
	for _, required := range []string{
		"Koschei Web3 | Intelligence Desk",
		"Investigate an address.",
		"Choose a network, collect the evidence, and inspect what controls the asset.",
		"data-scan-mode=\"token\"",
		"data-scan-mode=\"transaction\"",
		"data-scan-mode=\"deep\"",
		"arvis-premium-contract.js",
		"customer-arvis-premium-suite.js",
		"data-customer-arvis-result",
		"public-solana-scan.js?v=13",
		"Every material state keeps its source boundary",
		"Unknown stays unknown",
		"Presentation never outranks evidence maturity.",
		"No wallet connection, signing or private keys.",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("canonical investigation page missing %q", required)
		}
	}
	for _, forbidden := range []string{`data-scan-mode="quick"`, `href="/safe-check"`, `href="/transaction-shield"`, `href="/security-radar"`} {
		if strings.Contains(text, forbidden) {
			t.Errorf("canonical investigation page contains retired or duplicate scanner contract %q", forbidden)
		}
	}
}

func TestDashboardIsCurrentCustomerSecurityWorkspace(t *testing.T) {
	body, err := os.ReadFile("public/dashboard.html")
	if err != nil {
		t.Fatalf("read dashboard: %v", err)
	}
	text := string(body)
	for _, required := range []string{
		"Koschei Web3 | Intelligence Console",
		"Customer security workspace",
		"Security Overview",
		"What do you want to investigate?",
		"ARVIS intelligence map",
		"Live operational truth",
		"No fake telemetry",
		"Live account state",
		"Security Capabilities",
		"Missing evidence remains unknown.",
		"Solana is the live chain core.",
		"Read-only analysis. No wallet connection or private keys.",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("dashboard missing workspace contract %q", required)
		}
	}
	for _, forbidden := range []string{`id="mint"`, `id="scan"`, "/api/token/scan", "data-customer-arvis-result", "Signed report vault", "PROFESSIONAL · ARVIS COMMAND UNIVERSE"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("dashboard contains duplicate or retired workspace behavior %q", forbidden)
		}
	}
}

func TestUnifiedScanBehaviorKeepsRealLegacyEndpointsWithoutExposingQuickModeUI(t *testing.T) {
	body, err := os.ReadFile("public/js/public-solana-scan.js")
	if err != nil {
		t.Fatalf("read unified scan behavior: %v", err)
	}
	text := string(body)
	for _, required := range []string{
		"/api/arvis/preflight",
		"/api/token/scan",
		"/api/public/transaction-simulate",
		"Quick Check",
		"Token Investigation",
		"Simulate Transaction",
		"Deep Radar",
		"Missing evidence = no safety decision",
		"window.__koscheiUnifiedScanNavigation",
		"script.src='/js/unified-scan-navigation.js?v=1'",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("unified scan behavior missing %q", required)
		}
	}
}
