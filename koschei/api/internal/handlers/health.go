package handlers

import (
	"log"
	"net/http"
)

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	if err := h.DBPingError(); err != nil {
		log.Printf("health check database ping failed: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status":   "error",
			"database": "unavailable",
			"details":  err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":   "ok",
		"database": "connected",
	})
}
