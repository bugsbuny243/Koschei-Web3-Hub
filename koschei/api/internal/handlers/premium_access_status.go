package handlers

import (
	"net/http"
	"strings"
	"time"
)

type premiumAccessStatus struct {
	Active              bool       `json:"active"`
	Source              string     `json:"source"`
	TokenGateEnabled    bool       `json:"token_gate_enabled"`
	TokenConfigured     bool       `json:"token_configured"`
	WalletVerified      bool       `json:"wallet_verified"`
	TokenTier           string     `json:"token_tier"`
	TokenAmount         string     `json:"token_amount"`
	RequiredTokenTier   string     `json:"required_token_tier"`
	QuotaDaily          int        `json:"quota_daily"`
	QuotaUsedToday      int        `json:"quota_used_today"`
	QuotaRemainingToday int        `json:"quota_remaining_today"`
	QuotaResetsAt       *time.Time `json:"quota_resets_at,omitempty"`
}

func (h *Handler) PremiumAccessStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
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

	quota := newScanQuotaStatus(tokenAccess.Tier, configuredKOSCHDailyQuota(tokenAccess.Tier), time.Now().UTC())
	if tokenTierRank(tokenAccess.Tier) >= tokenTierRank("basic") {
		email := strings.ToLower(strings.TrimSpace(claims.Email))
		if email == "" {
			email = entitlementEmailFromSubject(claims.Sub)
		}
		if email != "" {
			quota, err = (postgresScanQuotaLedger{DB: h.DB}).Status(r.Context(), email, tokenAccess.Tier, configuredKOSCHDailyQuota(tokenAccess.Tier), time.Now().UTC())
			if err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "quota_unavailable"})
				return
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"access": decidePremiumAccess(tokenAccess, quota),
	})
}

func decidePremiumAccess(token tokenAccessEvaluation, quotaValues ...scanQuotaStatus) premiumAccessStatus {
	status := premiumAccessStatus{
		Active:            false,
		Source:            "none",
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
	if token.GateEnabled && token.Configured && token.WalletVerified && tokenTierRank(token.Tier) >= tokenTierRank("basic") {
		status.Active = true
		status.Source = "token"
		quota := newScanQuotaStatus(token.Tier, configuredKOSCHDailyQuota(token.Tier), time.Now().UTC())
		if len(quotaValues) > 0 {
			quota = quotaValues[0]
		}
		status.QuotaDaily = quota.Limit
		status.QuotaUsedToday = quota.Used
		status.QuotaRemainingToday = quota.Remaining
		reset := quota.ResetsAt
		status.QuotaResetsAt = &reset
	}
	return status
}
