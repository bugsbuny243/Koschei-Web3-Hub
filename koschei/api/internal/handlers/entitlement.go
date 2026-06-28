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
			_ = h.DB.QueryRowContext(r.Context(), `SELECT lower(email) FROM app_user_profiles WHERE auth_subject=$1 AND status='active'`, strings.TrimSpace(claims.Sub)).Scan(&email)
		}
		active, err := h.hasActiveEntitlementAccess(r.Context(), claims.Sub, email)
		if err != nil {
			writeAPIError(w, http.StatusServiceUnavailable, APICodeInternalError, "Entitlement access could not be verified", nil)
			return
		}
		if !active {
			writeAPIError(w, http.StatusForbidden, APICodePackageRequired, "Active Koschei package with remaining outputs required", nil)
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
			FROM entitlements e
			WHERE lower(e.email) = lower($1)
			  AND e.status = 'active'
			  AND COALESCE(e.plan_id, '') <> ''
			  AND COALESCE(e.plan_id, '') <> 'free'
			  AND COALESCE(e.outputs_remaining, 0) > 0
			  AND (e.expires_at IS NULL OR e.expires_at > now())
			  AND EXISTS (
				SELECT 1
				FROM app_user_profiles p
				WHERE lower(p.email) = lower(e.email)
				  AND p.status = 'active'
			  )
		)`, email).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return active, err
}

func (h *Handler) hasActiveEntitlementAccess(ctx context.Context, authSubject, email string) (bool, error) {
	if h.DB == nil {
		return false, errors.New("database unavailable")
	}
	authSubject = strings.TrimSpace(authSubject)
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		email = entitlementEmailFromSubject(authSubject)
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var active bool
	err := h.DB.QueryRowContext(ctx, `
		WITH identity AS (
			SELECT lower(p.email) AS email
			FROM app_user_profiles p
			WHERE p.status = 'active'
			  AND (
				($1 <> '' AND p.auth_subject = $1)
				OR ($2 <> '' AND lower(p.email) = lower($2))
			  )
			ORDER BY CASE WHEN $1 <> '' AND p.auth_subject = $1 THEN 0 ELSE 1 END,
			         p.updated_at DESC,
			         p.created_at DESC
			LIMIT 1
		)
		SELECT EXISTS (
			SELECT 1
			FROM entitlements e
			JOIN identity i ON lower(e.email) = i.email
			WHERE e.status = 'active'
			  AND COALESCE(e.plan_id, '') <> ''
			  AND COALESCE(e.plan_id, '') <> 'free'
			  AND COALESCE(e.outputs_remaining, 0) > 0
			  AND (e.expires_at IS NULL OR e.expires_at > now())
		)`, authSubject, email).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return active, err
}
