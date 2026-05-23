package handlers

import "net/http"

func (h *Handler) Credits(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	var total int
	if err := h.DB.QueryRow(`SELECT credits FROM app_user_profiles WHERE auth_subject=$1`, claims.Sub).Scan(&total); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"email": claims.Email, "credits": total})
}
