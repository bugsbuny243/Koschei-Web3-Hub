package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Handler struct {
	DB            *sql.DB
	AdminPassword string
	Limiter       *rateLimiter
	DBInitError   string
}

func (h *Handler) RequireDB(w http.ResponseWriter) bool {
	if h.DB == nil {
		resp := map[string]string{"error": "database unavailable"}
		if h.DBInitError != "" {
			resp["details"] = h.DBInitError
		}
		writeJSON(w, http.StatusServiceUnavailable, resp)
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := h.DB.PingContext(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable", "details": err.Error()})
		return false
	}
	return true
}

func (h *Handler) DBPingError() error {
	if h.DB == nil {
		if h.DBInitError != "" {
			return fmt.Errorf(h.DBInitError)
		}
		return fmt.Errorf("database connection is not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return h.DB.PingContext(ctx)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decodeJSON(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	return json.NewDecoder(r.Body).Decode(dst)
}

func (h *Handler) ownerAuth(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("x-admin-password") != h.AdminPassword || h.AdminPassword == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return false
	}
	return true
}
