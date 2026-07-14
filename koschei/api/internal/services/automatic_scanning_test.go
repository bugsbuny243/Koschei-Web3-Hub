package services

import "testing"

func TestAutomaticBackgroundScanningDisabledByDefault(t *testing.T) {
	t.Setenv("KOSCHEI_AUTOMATIC_SCANNING_ENABLED", "")
	if AutomaticBackgroundScanningEnabled() {
		t.Fatal("automatic scanning must be disabled when the master switch is absent")
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
