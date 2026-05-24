package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Handler struct {
	DB            *sql.DB
	AdminPassword string
	Limiter       *rateLimiter
	DBInitError   string
}

func (h *Handler) dbAvailable(ctx context.Context) error {
	if h.DB == nil {
		if h.DBInitError != "" {
			return errors.New(h.DBInitError)
		}
		return errors.New("database handle is nil")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return h.DB.PingContext(ctx)
}

func (h *Handler) RequireDB(w http.ResponseWriter) bool {
	if err := h.dbAvailable(context.Background()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable", "details": err.Error()})
		return false
	}
	return true
}

func (h *Handler) DBPingError() error {
	if err := h.dbAvailable(context.Background()); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func isTransientDBError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	patterns := []string{"driver: bad connection", "connection reset", "connection closed", "broken pipe", "eof"}
	for _, p := range patterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
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

func (h *Handler) OwnerAuth(w http.ResponseWriter, r *http.Request) bool {
	return h.ownerAuth(w, r)
}
