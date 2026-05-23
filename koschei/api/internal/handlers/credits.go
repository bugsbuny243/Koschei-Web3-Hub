package handlers

import "net/http"

func (h *Handler) Credits(w http.ResponseWriter, r *http.Request) {
	email, ok := emailFromAuthHeader(r)
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	var total int
	if err := h.DB.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM credits_ledger WHERE email=$1`, email).Scan(&total); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"email": email, "credits": total})
}
