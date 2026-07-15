package handlers

import (
	"os"
	"strings"
)

// The token-access parser already treats environment values as authoritative.
// Install the product default before handlers are constructed so an omitted
// Render env cannot silently fall back to the historical dust-holder value.
func init() {
	if strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_TIER_BASIC")) == "" {
		_ = os.Setenv("KOSCHEI_TOKEN_TIER_BASIC", "25000")
	}
}
