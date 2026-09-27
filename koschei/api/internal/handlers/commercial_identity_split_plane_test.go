package handlers

import (
	"os"
	"strings"
	"testing"
)

func TestCommercialIdentitySplitPlaneSourceContract(t *testing.T) {
	memberBody, err := os.ReadFile("member_summary.go")
	if err != nil {
		t.Fatal(err)
	}
	member := string(memberBody)
	for _, required := range []string{
		"if h.DB != nil",
		"provisionMemberOnDB(ctx, h.DB, sub, email)",
		"if h.EntitlementDB != nil && h.EntitlementDB != h.DB",
		"provisionMemberOnDB(ctx, h.EntitlementDB, sub, email)",
		"return entitlementSummary, nil",
	} {
		if !strings.Contains(member, required) {
			t.Fatalf("split-plane member provisioning missing %q", required)
		}
	}

	platformBody, err := os.ReadFile("platform.go")
	if err != nil {
		t.Fatal(err)
	}
	platform := string(platformBody)
	for _, required := range []string{
		`"provider": "professional_saas_entitlement"`,
		`"mode":     "public_proof_plus_professional_saas"`,
		`"professional": map[string]any{`,
		`"price_usd": 199`,
		`"billing_provider":    "polar"`,
		`"token_access":        "payment_channel_not_enabled"`,
		`"plan":  summary.Plan`,
	} {
		if !strings.Contains(platform, required) {
			t.Fatalf("commercial config/provision contract missing %q", required)
		}
	}
	for _, forbidden := range []string{
		`"billing_provider":    "paddle"`,
		`"starter":      map[string]any`,
		`"enterprise":   map[string]any`,
		"if !h.RequireDB(w)",
	} {
		if strings.Contains(platform, forbidden) {
			t.Fatalf("retired commercial contract remains: %q", forbidden)
		}
	}
}
