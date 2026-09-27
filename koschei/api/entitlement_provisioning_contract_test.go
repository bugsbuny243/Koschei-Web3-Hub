package main

import (
	"os"
	"strings"
	"testing"
)

func TestEntitlementStoreProvisioningContract(t *testing.T) {
	body, err := os.ReadFile("scripts/provision-entitlement-store.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS app_user_profiles",
		"auth_subject TEXT UNIQUE",
		"status TEXT NOT NULL DEFAULT 'active'",
		"banned_at TIMESTAMPTZ",
		"ban_reason TEXT",
		"CREATE TABLE IF NOT EXISTS entitlements",
		"payment_provider TEXT",
		"external_payment_id TEXT",
		"outputs_total INTEGER NOT NULL DEFAULT 0",
		"outputs_remaining INTEGER NOT NULL DEFAULT 0",
		"starts_at TIMESTAMPTZ DEFAULT now()",
		"expires_at TIMESTAMPTZ",
		"order_id UUID",
		"CREATE TABLE IF NOT EXISTS credit_events",
		"CREATE TABLE IF NOT EXISTS billing_provider_events",
		"raw_sha256 TEXT NOT NULL",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("provisioning contract missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"security_radar_verdicts",
		"web3_jobs",
		"risk_assessments",
		"global_radar",
		"owner_client_orders",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("commercial provisioning leaks application table %q", forbidden)
		}
	}
}

func TestEntitlementCommercialShapeMigration(t *testing.T) {
	body, err := os.ReadFile("migrations/120_entitlement_commercial_shape.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	for _, required := range []string{
		"ADD COLUMN IF NOT EXISTS starts_at TIMESTAMPTZ DEFAULT now()",
		"ADD COLUMN IF NOT EXISTS order_id UUID",
		"COALESCE(starts_at, created_at, now())",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("commercial-shape migration missing %q", required)
		}
	}
}
