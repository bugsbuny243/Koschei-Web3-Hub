package handlers

import (
	"net/http"
	"os"
	"strings"
	"time"
)

// PublicTokenStatus exposes only declared launch-readiness information. A mint
// is not represented as live until the later on-chain verification gate passes.
func (h *Handler) PublicTokenStatus(w http.ResponseWriter, r *http.Request) {
	mint := strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_MINT"))
	phase := "planning"
	if mint != "" {
		phase = "verification_pending"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"configured": mint != "",
		"phase":      phase,
		"network":    firstNonEmptyString(strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_NETWORK")), "solana-mainnet"),
		"mint":       mint,
		"identity": map[string]string{
			"name":   strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_NAME")),
			"symbol": strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_SYMBOL")),
		},
		"treasury": strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_TREASURY")),
		"public_links": map[string]string{
			"disclosure":     strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_DISCLOSURE_URL")),
			"vesting":        strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_VESTING_URL")),
			"liquidity_lock": strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_LIQUIDITY_LOCK_URL")),
		},
		"policy": map[string]bool{
			"investment_return_promised": false,
			"price_guarantee":            false,
			"hidden_minting_allowed":     false,
			"utility_first":              true,
		},
		"generated_at": time.Now().UTC(),
	})
}
