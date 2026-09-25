package main

import (
	"os"
	"strings"
	"testing"
)

func readSurfaceV3(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func TestCustomerSurfaceV3KeepsOneSimpleNavigationAndScanEntry(t *testing.T) {
	product := readSurfaceV3(t, "public/js/koschei-product-v2.js")
	for _, required := range []string{
		"['/','Home']",
		"['/scan','Scan']",
		"['/reports','Activity']",
		"['/dashboard','Workspace']",
		"['/pricing','Plans']",
		"customer-mobile-nav-v3",
		"customer-scan-flow-v3.js",
		"customer-result-guidance-v3.js",
		"customer-workspace-plans-v3.js",
	} {
		if !strings.Contains(product, required) {
			t.Fatalf("customer product shell missing %q", required)
		}
	}

	scan := readSurfaceV3(t, "public/js/customer-scan-flow-v3.js")
	for _, required := range []string{
		"Advanced scan options",
		"Target type override",
		"Site URL detected",
		"Serialized Solana transaction detected",
		"Solana address detected",
		"Ambiguous Solana addresses stay explicit",
	} {
		if !strings.Contains(scan, required) {
			t.Fatalf("customer scan flow missing %q", required)
		}
	}
	if strings.Contains(scan, "fetch(") {
		t.Fatal("customer scan presentation must not create a parallel evidence/API decision path")
	}
}

func TestCustomerResultGuidanceV3UsesCanonicalPolicyAndThreatPresentationOnly(t *testing.T) {
	guidance := readSurfaceV3(t, "public/js/customer-result-guidance-v3.js")
	for _, required := range []string{
		".customer-result-action",
		"threat_anticipation",
		"watch_signals",
		"koschei:customer-premium-mounted",
		"WHAT COULD HAPPEN",
		"WHAT TO WATCH",
		"WHAT TO DO NOW",
		"No deterministic blocking rule fired",
		"Koschei cannot establish a safe decision path",
	} {
		if !strings.Contains(guidance, required) {
			t.Fatalf("customer result guidance missing %q", required)
		}
	}
	for _, forbidden := range []string{"risk_index", "risk score", "Math.round", "Math.random", "fetch("} {
		if strings.Contains(guidance, forbidden) {
			t.Fatalf("customer result guidance must not calculate or fetch decision truth: found %q", forbidden)
		}
	}
}

func TestWorkspaceV3UsesSaaSEntitlementNotTokenHoldings(t *testing.T) {
	workspace := readSurfaceV3(t, "public/js/customer-workspace-plans-v3.js")
	for _, required := range []string{
		"/api/auth/premium-access",
		"starter",
		"professional",
		"enterprise",
		"Plans change capacity and eligible operational surfaces",
	} {
		if !strings.Contains(workspace, required) {
			t.Fatalf("workspace plan surface missing %q", required)
		}
	}
	for _, forbidden := range []string{"KOSCH", "token_access_snapshots", "wallet balance"} {
		if strings.Contains(workspace, forbidden) {
			t.Fatalf("workspace commercial access regressed to token authorization: found %q", forbidden)
		}
	}
}

func TestOwnerSurfaceV4UsesOneCanonicalRuntimeAndProfessionalAuthority(t *testing.T) {
	owner := readSurfaceV3(t, "public/js/owner-control-center.js")
	for _, required := range []string{
		"__koscheiOwnerCanonicalV4",
		"/api/owner/users",
		"Professional is the only operational paid plan",
		"/api/owner/token-telemetry",
		"Historical KOSCH observations · audit only",
		"Missing data stays unavailable, never zero or safe by default.",
	} {
		if !strings.Contains(owner, required) {
			t.Fatalf("owner v4 surface missing %q", required)
		}
	}
	for _, forbidden := range []string{"/api/owner/kosch-access", "KOSCH premium", "Free core + KOSCH"} {
		if strings.Contains(owner, forbidden) {
			t.Fatalf("owner v4 surface still contains retired contract %q", forbidden)
		}
	}

	v3 := readSurfaceV3(t, "public/js/owner-operations-v3.js")
	customers := readSurfaceV3(t, "public/js/owner-customer-directory.js")
	for path, body := range map[string]string{
		"owner-operations-v3.js":      v3,
		"owner-customer-directory.js": customers,
	} {
		if !strings.Contains(body, "if(window.__koscheiOwnerCanonicalV4)return;") {
			t.Fatalf("%s can still race the canonical owner runtime", path)
		}
	}

	html := readSurfaceV3(t, "public/owner-production.html")
	if !strings.Contains(html, "/js/owner-control-center.js?v=13") {
		t.Fatal("owner canonical runtime cache version was not advanced")
	}
}

func TestCustomerRouteVisibilityIsNotOwnedByOwnerDashboardCSS(t *testing.T) {
	css := readSurfaceV3(t, "public/css/koschei.css")
	if strings.Contains(css, ".page{display:none}.page.active{display:block}") {
		t.Fatal("owner dashboard visibility leaked into shared customer .page contract")
	}
	for _, required := range []string{
		"body.owner-command-v2 .page{display:none}",
		"body.owner-command-v2 .page.active{display:block}",
		".customer-main>main.customer-route-visible{display:block!important",
	} {
		if !strings.Contains(css, required) {
			t.Fatalf("shared stylesheet missing scoped visibility boundary %q", required)
		}
	}

	command := readSurfaceV3(t, "public/js/customer-command-center-v1.js")
	if !strings.Contains(command, "main.classList.add('customer-route-visible')") {
		t.Fatal("customer shell does not restore mounted route visibility")
	}

	scan := readSurfaceV3(t, "public/scan.html")
	for _, required := range []string{
		"body.koschei-enterprise main.page{display:block!important",
		"/js/customer-command-center-v1.js?v=5",
	} {
		if !strings.Contains(scan, required) {
			t.Fatalf("scan recovery contract missing %q", required)
		}
	}

	for _, path := range []string{"public/docs.html", "public/docs-api.html", "public/docs-sdk.html"} {
		doc := readSurfaceV3(t, path)
		if !strings.Contains(doc, ".page{display:block!important}") {
			t.Fatalf("%s can still be hidden by shared owner CSS", path)
		}
		if strings.Contains(doc, "active Enterprise") || strings.Contains(doc, "Enterprise SaaS entitlement") {
			t.Fatalf("%s still advertises retired Enterprise authorization", path)
		}
	}
}
