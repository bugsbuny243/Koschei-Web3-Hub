package handlers

import (
	"net/http"
	"strings"
)

type premiumAccessStatus struct {
	Active            bool   `json:"active"`
	Source            string `json:"source"`
	PackageActive     bool   `json:"package_active"`
	TokenGateEnabled  bool   `json:"token_gate_enabled"`
	TokenConfigured   bool   `json:"token_configured"`
	WalletVerified    bool   `json:"wallet_verified"`
	TokenTier         string `json:"token_tier"`
	TokenAmount       string `json:"token_amount"`
	RequiredTokenTier string `json:"required_token_tier"`
}

func (h *Handler) PremiumAccessStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	email := normalizedClaimEmail(claims)
	if email == "" && strings.TrimSpace(claims.Sub) != "" && h.DB != nil {
		_ = h.DB.QueryRowContext(r.Context(), `
			SELECT lower(email)
			FROM app_user_profiles
			WHERE auth_subject=$1 AND status='active'`, strings.TrimSpace(claims.Sub)).Scan(&email)
	}

	packageActive, err := h.hasActiveEntitlementAccess(r.Context(), claims.Sub, email)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "premium_access_unavailable"})
		return
	}
	if packageActive {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true,
			"access": decidePremiumAccess(true, tokenAccessEvaluation{}),
		})
		return
	}

	tokenAccess, err := h.evaluateTokenAccess(r.Context(), claims.Sub)
	if err != nil {
		if accessErr, ok := err.(tokenAccessError); ok {
			writeJSON(w, accessErr.Status, map[string]string{"error": accessErr.Code})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "token_access_unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"access": decidePremiumAccess(false, tokenAccess),
	})
}

func decidePremiumAccess(packageActive bool, token tokenAccessEvaluation) premiumAccessStatus {
	status := premiumAccessStatus{
		Active:            packageActive,
		Source:            "none",
		PackageActive:     packageActive,
		TokenGateEnabled:  token.GateEnabled,
		TokenConfigured:   token.Configured,
		WalletVerified:    token.WalletVerified,
		TokenTier:         token.Tier,
		TokenAmount:       token.Amount,
		RequiredTokenTier: "basic",
	}
	if status.TokenTier == "" {
		status.TokenTier = "none"
	}
	if status.TokenAmount == "" {
		status.TokenAmount = "0"
	}
	if packageActive {
		status.Source = "package"
		return status
	}
	if token.GateEnabled && token.Configured && token.WalletVerified && tokenTierRank(token.Tier) >= tokenTierRank("basic") {
		status.Active = true
		status.Source = "token"
	}
	return status
}
