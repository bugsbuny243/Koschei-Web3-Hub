package main

import (
	"os"
	"strings"
	"testing"
)

func TestCurrentPublicSurfacesAreSourceEnglish(t *testing.T) {
	files := []string{
		"public/index.html",
		"public/dashboard.html",
		"public/owner-production.html",
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

func TestRetiredClassicScanPageStaysRemoved(t *testing.T) {
	if _, err := os.Stat("public/scan.html"); err == nil {
		t.Fatal("retired classic scan page returned")
	}
}

func TestDashboardIsSingleCustomerSecurityPanel(t *testing.T) {
	body, err := os.ReadFile("public/dashboard.html")
	if err != nil {
		t.Fatalf("read dashboard: %v", err)
	}
	text := string(body)
	for _, required := range []string{
		"Koschei Web3 | Customer Panel",
		"Security Overview",
		"ARVIS intelligence map",
		"Live operational truth",
		"No fake telemetry",
		"Authenticated session",
		"Durable history",
		"Continuous monitoring",
		"Persisted alerts",
		"Feedback storage",
		"Exposure Report",
		"Report a gap",
		"Transaction Preflight",
		"LIVE · SOLANA MAINNET · READ ONLY",
		"NO SIGNING · NO BROADCAST",
		"transactionPreflightForm",
		"PERSISTENCE OFF",
		"Production truth only.",
		"Solana is the live chain core.",
		"NOT LIVE",
		"/css/koschei-dashboard.css?v=3",
		"/js/customer-workspace-v2.js?v=3",
		"/js/koschei-dashboard.js?v=5",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("dashboard missing customer panel contract %q", required)
		}
	}
	if got := strings.Count(text, `<link rel="stylesheet"`); got != 1 {
		t.Errorf("dashboard must load exactly one stylesheet, got %d", got)
	}
	for _, forbidden := range []string{
		`id="mint"`,
		`id="scan"`,
		`id="exposureForm"`,
		`id="feedbackForm"`,
		"/api/token/scan",
		"data-customer-arvis-result",
		"Signed report vault",
		"RECENT CANONICAL INVESTIGATION",
		"RECENT MONITORING ALERTS",
		"koschei.css",
		"customer-command-center-v1.css",
		"customer-command-universe-v2.css",
		"koschei-global-shell.js",
		"koschei-product-v2.js",
		"KOSCH Premium",
		"KOSCH holder",
	} {
		if strings.Contains(text, forbidden) {
			t.Errorf("dashboard contains duplicate, legacy or overstated surface %q", forbidden)
		}
	}
}

func TestRetainedLegacyScanRendererKeepsEvidenceBoundaries(t *testing.T) {
	body, err := os.ReadFile("public/js/public-solana-scan.js")
	if err != nil {
		t.Fatalf("read retained legacy scan renderer: %v", err)
	}
	text := string(body)
	for _, required := range []string{
		"/api/arvis/preflight",
		"/api/token/scan",
		"/api/public/transaction-simulate",
		"Missing evidence = no safety decision",
		"window.__koscheiUnifiedScanNavigation",
		"script.src='/js/unified-scan-navigation.js?v=1'",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("retained legacy scan renderer missing %q", required)
		}
	}
}
