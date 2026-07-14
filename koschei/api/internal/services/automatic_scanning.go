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
	switch strings.ToLower(strings.TrimSpace(os.Getenv("KOSCHEI_AUTOMATIC_SCANNING_ENABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
