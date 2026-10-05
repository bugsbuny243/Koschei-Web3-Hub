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

func TestCanonicalInvestigationSurfaceIsOneCustomerRadar(t *testing.T) {
	body, err := os.ReadFile("public/scan.html")
	if err != nil {
		t.Fatalf("read canonical investigation page: %v", err)
	}
	text := string(body)
	for _, required := range []string{
		"ARVIS Radar",
		"ONE RADAR · ONE TARGET",
		`id="scanForm"`,
		`id="radarMode"`,
		`id="scanNetwork"`,
		`id="target"`,
		`id="customerUniversalResultsWrap"`,
		"customer-universal-address-scan-v1.js?v=3",
		"public-solana-scan.js?v=14",
		"One target only.",
		"Missing evidence stays unknown.",
		"never signs, and never broadcasts.",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("canonical investigation page missing %q", required)
		}
	}
	if strings.Count(text, "<form ") != 1 {
		t.Error("canonical investigation page must have one request form")
	}
	for _, forbidden := range []string{
		`<option value="token">Solana token</option>`,
		`<option value="quick">Site or URL</option>`,
		`<option value="transaction">Solana transaction before signing</option>`,
		`<option value="spending">EVM token allowance</option>`,
		`href="/safe-check"`,
		`href="/transaction-shield"`,
		`href="/security-radar"`,
		`id="customerUniversalScanForm"`,
		`id="evmAuthorityForm"`,
		`id="evmSpendingForm"`,
		"customer-transaction-preflight-v1.js",
		"evm-spending-intelligence.js",
		"customer-command-center-v1.js",
		"customer-arvis-premium-suite.js",
	} {
		if strings.Contains(text, forbidden) {
			t.Errorf("canonical investigation page exposes duplicate or retired scanner UI %q", forbidden)
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
		"Koschei Web3 | Customer Workspace",
		"Customer workspace",
		"One radar. Your evidence.",
		"ARVIS Radar",
		"Your latest activity",
		"CHECKING ACCOUNT DATA",
		`id="dashboardUniversalScanForm"`,
		`id="dashboardScanNetwork"`,
		"Missing evidence remains unknown.",
		"Unavailable records remain unavailable.",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("dashboard missing workspace contract %q", required)
		}
	}
	if strings.Count(text, "<form ") != 1 {
		t.Error("dashboard must have one radar entry form")
	}
	for _, forbidden := range []string{`id="mint"`, `id="scan"`, "/api/token/scan", "data-customer-arvis-result", "Signed report vault", "PROFESSIONAL · ARVIS COMMAND UNIVERSE", `class="tool-card`, `class="intel-map`, `href="/scan?mode=deep"`} {
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
		"script.src='/js/unified-scan-navigation.js?v=2'",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("unified scan behavior missing %q", required)
		}
	}
}
