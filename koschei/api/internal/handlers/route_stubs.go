package handlers

import "net/http"

func notImplemented(w http.ResponseWriter, name string) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "not_implemented", "handler": name})
}

func (h *Handler) premiumStub(w http.ResponseWriter, r *http.Request, name string) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if _, err := h.requirePremiumOutput(claims.Sub); err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}
	notImplemented(w, name)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	h.NeonRegister(w, r)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	h.NeonLogin(w, r)
}

func (h *Handler) StartOTPLogin(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "StartOTPLogin")
}
func (h *Handler) VerifyOTPLogin(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "VerifyOTPLogin")
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	profile := map[string]any{"auth_subject": claims.Sub, "email": claims.Email, "role": "member", "plan_id": "free", "plan": "free", "credits": 0, "outputs_total": 0, "outputs_remaining": 0}
	if h.DB != nil {
		summary, err := h.provisionMember(r.Context(), claims)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "profile_unavailable"})
			return
		}
		p, err := h.upsertProfile(r.Context(), claims.Sub, claims.Email)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "profile_unavailable"})
			return
		}
		profile = map[string]any{
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
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": profile})
}

func (h *Handler) AIGenerate(w http.ResponseWriter, r *http.Request) { h.premiumStub(w, r, "AIGenerate") }
func (h *Handler) AIJobs(w http.ResponseWriter, r *http.Request)     { h.premiumStub(w, r, "AIJobs") }
