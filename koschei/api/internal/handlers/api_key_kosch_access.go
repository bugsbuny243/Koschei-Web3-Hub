package handlers

import (
	"context"
	"net/http"
	"strings"
)

// RequireAPIKeyKOSCH remains the compatibility entry point for basic API-key
// holder access. New developer routes use RequireAPIKeyTokenTier explicitly.
func (h *Handler) RequireAPIKeyKOSCH(next http.HandlerFunc) http.HandlerFunc {
	return h.RequireAPIKeyTokenTier("basic", next)
}

func (h *Handler) RequireAPIKeyTokenTier(required string, next http.HandlerFunc) http.HandlerFunc {
	required = strings.ToLower(strings.TrimSpace(required))
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := apiPrincipalFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		if tokenTierRank(required) == 0 {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "invalid_token_tier_configuration"})
			return
		}
		evaluation, err := h.evaluateTokenAccess(r.Context(), principal.AuthSubject)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosch_access_unavailable"})
			return
		}
		if !evaluation.GateEnabled || !evaluation.Configured || !evaluation.WalletVerified || tokenTierRank(evaluation.Tier) < tokenTierRank(required) {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error":         "token_tier_required",
				"required_tier": required,
				"current_tier":  evaluation.Tier,
			})
			return
		}
		ctx := context.WithValue(r.Context(), tokenAccessContextKey{}, evaluation)
		next(w, r.WithContext(ctx))
	}
}
