package handlers

import "net/http"

// RequireAPIKeyKOSCH binds developer API usage to the same verified KOSCH
// holder policy as customer-session product routes. API keys remain identity
// credentials; they do not bypass holder verification.
func (h *Handler) RequireAPIKeyKOSCH(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := apiPrincipalFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		active, err := h.hasTokenTierAccess(r.Context(), principal.AuthSubject, "basic")
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosch_access_unavailable"})
			return
		}
		if !active {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error":   "kosch_holder_required",
				"message": "Verified KOSCH holder access is required for developer API usage.",
			})
			return
		}
		next(w, r)
	}
}
