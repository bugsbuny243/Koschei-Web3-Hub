package handlers

import "net/http"

func (h *Handler) Credits(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		writeJSON(w, 400, map[string]string{"error": "email required"})
		return
	}
	var total int
	// Runtime credit balance source-of-truth: credits_ledger only (never credit_events / credit_ledger).
	if err := h.DB.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM credits_ledger WHERE email=$1`, email).Scan(&total); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"email": email, "credits": total})
}
