package handlers

import "testing"

func TestDefaultKOSCHTierThresholdsAreCommerciallyDistinct(t *testing.T) {
	t.Setenv("KOSCHEI_TOKEN_TIER_BASIC", "")
	t.Setenv("KOSCHEI_TOKEN_TIER_PRO", "")
	t.Setenv("KOSCHEI_TOKEN_TIER_ENTERPRISE", "")

	values, raw, err := configuredTokenThresholds(6)
	if err != nil {
		t.Fatalf("configured thresholds: %v", err)
	}
	if values["basic"] != "25000" || values["pro"] != "250000" || values["enterprise"] != "2000000" {
		t.Fatalf("unexpected defaults: %#v", values)
	}
	if raw["basic"].Cmp(raw["pro"]) >= 0 || raw["pro"].Cmp(raw["enterprise"]) >= 0 {
		t.Fatalf("tier thresholds are not strictly increasing: %#v", values)
	}
}

func TestKOSCHTierThresholdsRemainEnvDriven(t *testing.T) {
	t.Setenv("KOSCHEI_TOKEN_TIER_BASIC", "100")
	t.Setenv("KOSCHEI_TOKEN_TIER_PRO", "200")
	t.Setenv("KOSCHEI_TOKEN_TIER_ENTERPRISE", "300")

	values, _, err := configuredTokenThresholds(6)
	if err != nil {
		t.Fatalf("configured thresholds: %v", err)
	}
	if values["basic"] != "100" || values["pro"] != "200" || values["enterprise"] != "300" {
		t.Fatalf("env overrides ignored: %#v", values)
	}
}
