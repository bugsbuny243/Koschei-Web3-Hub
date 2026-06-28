package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"
)

type customerPackageStatus struct {
	HasActivePackage bool       `json:"has_active_package"`
	PlanID           *string    `json:"plan_id"`
	Status           string     `json:"status"`
	OutputsTotal     int        `json:"outputs_total"`
	OutputsRemaining int        `json:"outputs_remaining"`
	RemainingOutputs int        `json:"remaining_outputs"`
	StartsAt         *time.Time `json:"starts_at,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at"`
	Warning          string     `json:"warning,omitempty"`
}

func (h *Handler) MePackage(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required")
		return
	}
	status, err := h.customerPackageStatus(r.Context(), claims.Sub, normalizedClaimEmail(claims))
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeInternalError, "Package access could not be verified", nil)
		return
	}
	writeAPIData(w, http.StatusOK, status)
}

func (h *Handler) customerPackageStatus(ctx context.Context, authSubject, email string) (customerPackageStatus, error) {
	if h.DB == nil {
		return customerPackageStatus{}, errors.New("database unavailable")
	}
	if err := ensurePaymentSchema(ctx, h.DB); err != nil {
		return customerPackageStatus{}, err
	}
	authSubject = strings.TrimSpace(authSubject)
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		email = entitlementEmailFromSubject(authSubject)
	}
	var planID, status string
	var startsAt, expiresAt sql.NullTime
	var outputsTotal, outputsRemaining int
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
		SELECT COALESCE(e.plan_id,''), COALESCE(e.status,''), COALESCE(e.outputs_total,0), COALESCE(e.outputs_remaining,0), e.starts_at, e.expires_at
		FROM entitlements e
		JOIN identity i ON lower(e.email) = i.email
		WHERE e.status = 'active'
		  AND COALESCE(e.plan_id, '') <> ''
		  AND COALESCE(e.plan_id, '') <> 'free'
		  AND COALESCE(e.outputs_remaining, 0) > 0
		  AND (e.expires_at IS NULL OR e.expires_at > now())
		ORDER BY e.updated_at DESC NULLS LAST, e.created_at DESC
		LIMIT 1`, authSubject, email).Scan(&planID, &status, &outputsTotal, &outputsRemaining, &startsAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return customerPackageStatus{HasActivePackage: false, PlanID: nil, Status: "none", ExpiresAt: nil}, nil
	}
	if err != nil {
		return customerPackageStatus{}, err
	}
	planID = normalizePackageID(planID)
	if planID == "" {
		return customerPackageStatus{}, errors.New("invalid active package")
	}
	if outputsTotal <= 0 {
		if count, ok := packageOutputCount(planID); ok {
			outputsTotal = count
		}
	}
	if outputsRemaining < 0 {
		outputsRemaining = 0
	}
	return customerPackageStatus{HasActivePackage: true, PlanID: &planID, Status: firstNonEmpty(status, "active"), OutputsTotal: outputsTotal, OutputsRemaining: outputsRemaining, RemainingOutputs: outputsRemaining, StartsAt: nullTimePtr(startsAt), ExpiresAt: nullTimePtr(expiresAt)}, nil
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
