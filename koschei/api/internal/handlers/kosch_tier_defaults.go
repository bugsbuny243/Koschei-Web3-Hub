package handlers

import (
	"os"
	"strings"
)

func init() {
	if strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_TIER_BASIC")) == "" {
		_ = os.Setenv("KOSCHEI_TOKEN_TIER_BASIC", "25000")
	}
}
