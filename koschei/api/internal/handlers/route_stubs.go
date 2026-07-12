package handlers

import (
	"errors"
	"log"
	"net/http"
)

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	h.NeonRegister(w, r)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	h.NeonLogin(w, r)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable"})
		return
	}
	if err := ensureOwnerSchema(r.Context(), h.DB); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "profile storage unavailable"})
		return
	}
	summary, err := h.provisionMember(r.Context(), claims)
	if errors.Is(err, errAccountDisabled) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "account_disabled", "message": "Account access is disabled."})
		return
	}
	if err != nil {
		log.Printf("provisionMember failed: sub=%s email=%s err=%v", claims.Sub, claims.Email, err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "profile provisioning unavailable"})
		return
	}
	p, err := h.upsertProfile(r.Context(), claims.Sub, claims.Email)
	if errors.Is(err, errAccountDisabled) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "account_disabled", "message": "Account access is disabled."})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "profile provisioning unavailable"})
		return
	}
	profile := map[string]any{
		"id":                p.ID,
		"auth_subject":      p.AuthSubject,
		"email":             p.Email,
		"role":              firstNonEmpty(p.Role, "member"),
		"plan_id":           firstNonEmpty(summary.Plan, p.PlanID, "free"),
		"plan":              firstNonEmpty(summary.Plan, p.PlanID, "free"),
		"credits":           p.Credits,
		"outputs_total":     summary.OutputsTotal,
		"outputs_remaining": summary.OutputsRemaining,
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": profile})
}
