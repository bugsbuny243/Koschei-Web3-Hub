package main

import (
	"os"
	"strings"
	"testing"
)

func TestCommercialReadinessCustomerSurfaces(t *testing.T) {
	pricing := mustReadCommercialSurface(t, "public/pricing.html")
	for _, required := range []string{
		"CURRENT DEPLOYMENT · COMMERCIAL TRUTH",
		"Do not sell what the runtime cannot deliver.",
		"Professional remains the current server-side entitlement contract",
		"Commercial authorization exists. Commercial checkout is not live.",
		"Starter and Enterprise remain product-package directions, not active sale packages in this deployment.",
		"The server decides access. The browser never invents it.",
		"NOT LIVE · DB-backed secure checkout in the current stateless process",
	} {
		if !strings.Contains(pricing, required) {
			t.Errorf("pricing missing %q", required)
		}
	}
	for _, forbidden := range []string{
		`data-polar-plan="professional"`,
		"polar-checkout-v1.js",
		`id="earlyAccessForm"`,
		"<h2>Free Core</h2>",
		"$299 / month",
		"$999 / month",
		"$4,999 / month",
	} {
		if strings.Contains(pricing, forbidden) {
			t.Errorf("pricing exposes unavailable or retired commercial surface %q", forbidden)
		}
	}

	dashboard := mustReadCommercialSurface(t, "public/dashboard.html")
	for _, required := range []string{
		"Durable history",
		"Continuous monitoring",
		"Persisted alerts",
		"NOT LIVE",
		"PERSISTENCE OFF",
		"The current production process is intentionally stateless",
	} {
		if !strings.Contains(dashboard, required) {
			t.Errorf("dashboard commercial truth boundary missing %q", required)
		}
	}
	for _, retired := range []string{
		"public/js/customer-reports-v2.js",
		"public/js/customer-watchlist-v2.js",
		"public/js/customer-arvis-chat-v1.js",
		"public/js/customer-arvis-metaverse-v1.js",
		"public/js/polar-checkout-v1.js",
	} {
		if _, err := os.Stat(retired); err == nil {
			t.Errorf("retired or unavailable customer runtime returned: %s", retired)
		}
	}

	navigation := mustReadCommercialSurface(t, "public/js/unified-scan-navigation.js")
	if !strings.Contains(navigation, ".customer-sidebar__nav,.customer-command-palette") {
		t.Error("scan navigation does not protect mode-specific customer navigation")
	}
	if !strings.Contains(navigation, "group.matches(protectedCustomerNavSelector)") {
		t.Error("scan navigation does not exclude the customer sidebar from scan-link collapse")
	}

	workspaceCSS := mustReadCommercialSurface(t, "public/css/koschei.css")
	if !strings.Contains(workspaceCSS, ".koschei-safety-strip{display:none!important}") {
		t.Error("dashboard self-promo strip is not suppressed from the customer workspace")
	}

	universeCSS := mustReadCommercialSurface(t, "public/css/koschei.css")
	for _, required := range []string{"body.koschei-universe", ".universe-entry", ".professional-lock"} {
		if !strings.Contains(universeCSS, required) {
			t.Errorf("universe visual system missing %q", required)
		}
	}
}

func mustReadCommercialSurface(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}
