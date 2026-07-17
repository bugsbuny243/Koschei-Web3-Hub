package services

import (
	"os"
	"strings"
)

// AutomaticBackgroundScanningEnabled is the master switch for every
// quota-consuming background scanner. It is intentionally opt-in: an absent,
// malformed, or false value keeps automatic scanning disabled while manual
// owner/customer endpoints remain available.
func AutomaticBackgroundScanningEnabled() bool {
	if OwnerUnlimitedAutomaticScanningEnabled() {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("KOSCHEI_AUTOMATIC_SCANNING_ENABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// OwnerUnlimitedAutomaticScanningEnabled is an explicit test-only operating
// mode for the owner deployment. It starts the automatic pipeline and removes
// Koschei's local RPC/report quotas; upstream provider throttling still applies.
// Keeping this separate from the normal master switch prevents an absent
// configuration from silently enabling quota-consuming workers.
func OwnerUnlimitedAutomaticScanningEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("KOSCHEI_OWNER_UNLIMITED_AUTOSCAN_ENABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
