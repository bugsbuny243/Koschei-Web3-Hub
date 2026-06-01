package handlers

import "net/http"

func (handler *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "auth-api"})
}
