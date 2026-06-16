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
		active, err := h.hasActiveEntitlementAccess(r.Context(), claims.Sub, email)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, APICodeInternalError, "Entitlement access could not be verified", nil)
			return
		}
		if !active {
			writeAPIError(w, http.StatusForbidden, APICodePackageRequired, "Active Koschei package or credits required", nil)
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
		SELECT EXISTS (
			SELECT 1
			FROM entitlements e
			LEFT JOIN app_user_profiles p ON lower(p.email) = lower(e.email)
			WHERE e.status = 'active'
			  AND COALESCE(e.plan_id, '') <> 'free'
			  AND (e.expires_at IS NULL OR e.expires_at > now())
			  AND (
				($1 <> '' AND p.auth_subject = $1)
				OR ($2 <> '' AND lower(e.email) = lower($2))
			  )
			UNION ALL
			SELECT 1
			FROM app_user_profiles p
			WHERE (($1 <> '' AND p.auth_subject=$1) OR ($2 <> '' AND lower(p.email)=lower($2)))
			  AND (COALESCE(p.credits,0) > 0 OR COALESCE(p.plan_id,'free') <> 'free')
			LIMIT 1
		)`, authSubject, email).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return active, err
}
