package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"
)

func (h *Handler) RequireActiveEntitlement(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := userFromContext(r.Context())
		if !ok {
			writeAPIError(w, http.StatusUnauthorized, APICodeUnauthorized, "Unauthorized", nil)
			return
		}
		email := normalizedClaimEmail(claims)
		if email == "" && strings.TrimSpace(claims.Sub) != "" && h.DB != nil {
			_ = h.DB.QueryRowContext(r.Context(), `SELECT lower(email) FROM app_user_profiles WHERE auth_subject=$1`, strings.TrimSpace(claims.Sub)).Scan(&email)
		}
		active, err := h.hasActiveEntitlement(r.Context(), email)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, APICodeInternalError, "Entitlement access could not be verified", nil)
			return
		}
		if !active {
			writeAPIError(w, http.StatusForbidden, APICodePackageRequired, "Active Koschei package required", nil)
			return
		}
		next(w, r)
	}
}

func (h *Handler) hasActiveEntitlement(ctx context.Context, email string) (bool, error) {
	if h.DB == nil {
		return false, errors.New("database unavailable")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var active bool
	err := h.DB.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM entitlements
			WHERE lower(email) = lower($1)
			  AND status = 'active'
			  AND (expires_at IS NULL OR expires_at > now())
		)`, email).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return active, err
}
