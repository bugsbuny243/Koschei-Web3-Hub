package services

import "testing"

func TestAutomaticBackgroundScanningDisabledByDefault(t *testing.T) {
	t.Setenv("KOSCHEI_AUTOMATIC_SCANNING_ENABLED", "")
	t.Setenv("KOSCHEI_OWNER_UNLIMITED_AUTOSCAN_ENABLED", "")
	if AutomaticBackgroundScanningEnabled() {
		t.Fatal("automatic scanning must be disabled when the master switch is absent")
	}
}

func TestOwnerUnlimitedModeExplicitlyEnablesScanningAndRemovesLocalLimits(t *testing.T) {
	t.Setenv("KOSCHEI_AUTOMATIC_SCANNING_ENABLED", "false")
	t.Setenv("KOSCHEI_OWNER_UNLIMITED_AUTOSCAN_ENABLED", "true")
	t.Setenv("SOLANA_RPC_LIMIT_SAVER_ENABLED", "true")
	t.Setenv("SOLANA_RPC_BUDGET_ENABLED", "true")
	t.Setenv("KOSCHEI_AUTO_RADAR_ENABLED", "0")

	if !AutomaticBackgroundScanningEnabled() || !SecurityRadarAutoEnabled() {
		t.Fatal("owner unlimited mode must start the automatic radar pipeline")
	}
	if SolanaRPCLimitSaverEnabled() || solanaRPCBudgetEnabled() {
		t.Fatal("owner unlimited mode must remove Koschei's local RPC limits")
	}
	if !ForceBackgroundRadarEnabled() {
		t.Fatal("owner unlimited mode must keep background radar workers active")
	}
	if got := pumpHighVolumeMaxReportsPerCycle(); got != 0 {
		t.Fatalf("report cycle limit = %d, want 0 (unlimited)", got)
	}
}

func TestAutomaticBackgroundScanningRequiresExplicitTrue(t *testing.T) {
	for _, value := range []string{"false", "0", "off", "invalid"} {
		t.Setenv("KOSCHEI_AUTOMATIC_SCANNING_ENABLED", value)
		if AutomaticBackgroundScanningEnabled() {
			t.Fatalf("automatic scanning unexpectedly enabled for %q", value)
		}
	}
	for _, value := range []string{"true", "1", "yes", "on"} {
		t.Setenv("KOSCHEI_AUTOMATIC_SCANNING_ENABLED", value)
		if !AutomaticBackgroundScanningEnabled() {
			t.Fatalf("automatic scanning did not enable for %q", value)
		}
	}
}
