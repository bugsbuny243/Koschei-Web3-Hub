package handlers

import "net/http"

func (h *Handler) OwnerLogout(w http.ResponseWriter, r *http.Request) {
	h.revokeOwnerSession(r.Context(), r)
	clearOwnerSessionCookies(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Owner oturumu kapatıldı."})
}
