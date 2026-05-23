package handlers

import "net/http"

func (h *Handler) Credits(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	email := claims.Email
	var total int
	if err := h.DB.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM credit_events WHERE email=$1`, email).Scan(&total); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"email": email, "credits": total})
}
