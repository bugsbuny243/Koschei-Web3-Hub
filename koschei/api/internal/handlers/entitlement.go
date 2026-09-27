package handlers

import "net/http"

// RequireActiveEntitlement is kept as a compatibility wrapper for older call
// sites. Commercial access is determined only by an active Professional SaaS
// entitlement; KOSC holdings are never consulted. A verified KOSC settlement
// may create that entitlement through the commercial ledger.
func (h *Handler) RequireActiveEntitlement(next http.HandlerFunc) http.HandlerFunc {
	return h.RequirePlanTier("professional", next)
}

func (h *Handler) RequirePremiumAccess(next http.HandlerFunc) http.HandlerFunc {
	return h.RequireActiveEntitlement(next)
}
