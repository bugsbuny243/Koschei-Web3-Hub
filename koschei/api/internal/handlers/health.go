package handlers

import "net/http"

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	if !h.RequireDB(w) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
