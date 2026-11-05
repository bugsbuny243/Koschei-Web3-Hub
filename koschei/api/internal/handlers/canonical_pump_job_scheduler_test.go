package handlers

import (
	"testing"
	"time"

	"koschei/api/internal/services"
)

func TestCanonicalPumpSelectiveSchedulingSurvivesBroadSaverGates(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("KOSCHEI_OWNER_UNLIMITED_AUTOSCAN_ENABLED", "false")
	t.Setenv("KOSCHEI_AUTOMATIC_SCANNING_ENABLED", "false")
	t.Setenv("SOLANA_RPC_LIMIT_SAVER_ENABLED", "true")
	t.Setenv("PUMPPORTAL_ENABLED", "true")
	t.Setenv("PUMP_HIGH_VOLUME_RADAR_ENABLED", "true")

	if services.AutomaticBackgroundScanningEnabled() {
		t.Fatal("broad automatic scanning must remain disabled")
	}
	if !services.SolanaRPCLimitSaverEnabled() {
		t.Fatal("RPC saver must remain enabled")
	}
	if !canonicalPumpAutoSchedulingAllowed() {
		t.Fatal("explicit selective Pump scheduling must remain available under broad saver gates")
	}
	if got := canonicalPumpMaxJobsPerCycle(); got != 1 {
		t.Fatalf("selective Pump job cap=%d want=1", got)
	}
}

func TestCanonicalPumpSelectiveSchedulingRequiresPumpGate(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("KOSCHEI_OWNER_UNLIMITED_AUTOSCAN_ENABLED", "false")
	t.Setenv("KOSCHEI_AUTOMATIC_SCANNING_ENABLED", "true")
	t.Setenv("SOLANA_RPC_LIMIT_SAVER_ENABLED", "false")
	t.Setenv("PUMPPORTAL_ENABLED", "true")
	t.Setenv("PUMP_HIGH_VOLUME_RADAR_ENABLED", "false")

	if canonicalPumpAutoSchedulingAllowed() {
		t.Fatal("selective Pump scheduling must fail closed when PUMP_HIGH_VOLUME_RADAR_ENABLED=false")
	}
}

func TestCanonicalPumpCycleDelayBacksOffAndRecovers(t *testing.T) {
	for _, tc := range []struct {
		poll time.Duration
		failures int
		want time.Duration
	}{
		{time.Minute, 0, time.Minute},
		{time.Minute, 1, time.Minute},
		{time.Minute, 2, 2 * time.Minute},
		{time.Minute, 3, 4 * time.Minute},
		{time.Minute, 100, 10 * time.Minute},
		{15 * time.Minute, 2, 15 * time.Minute},
		{15 * time.Minute, 0, 15 * time.Minute},
	} {
		if got := canonicalPumpCycleDelay(tc.poll, tc.failures); got != tc.want {
			t.Errorf("poll=%s failures=%d: got %s want %s", tc.poll, tc.failures, got, tc.want)
		}
	}
}
