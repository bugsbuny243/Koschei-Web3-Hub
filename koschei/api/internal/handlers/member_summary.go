package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

const (
	freePlanID          = "free"
	freeOutputsIncluded = 100
)

type memberSummaryResponse struct {
	OK               bool   `json:"ok"`
	Email            string `json:"email"`
	Plan             string `json:"plan"`
	OutputsTotal     int    `json:"outputs_total"`
	OutputsRemaining int    `json:"outputs_remaining"`
}

func (h *Handler) MemberSummary(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	summary, err := h.provisionMember(r.Context(), claims)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "member summary unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) provisionMember(ctx context.Context, claims neonJWTClaims) (memberSummaryResponse, error) {
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	sub := strings.TrimSpace(claims.Sub)
	if email == "" || sub == "" {
		return memberSummaryResponse{}, errors.New("verified token is missing member identity")
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return memberSummaryResponse{}, err
	}
	defer tx.Rollback()

	// Serialize entitlement initialization per normalized email so concurrent
	// provisioning requests cannot create multiple free entitlements.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, email); err != nil {
		return memberSummaryResponse{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO app_user_profiles (auth_subject, email)
		VALUES ($1, $2)
		ON CONFLICT (auth_subject) DO UPDATE SET email = EXCLUDED.email`, sub, email); err != nil {
		return memberSummaryResponse{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO entitlements (email, plan_id, outputs_total, outputs_remaining, status)
		SELECT $1, $2, $3, $3, 'active'
		WHERE NOT EXISTS (
			SELECT 1 FROM entitlements WHERE lower(email) = $1 AND status = 'active'
		)`, email, freePlanID, freeOutputsIncluded); err != nil {
		return memberSummaryResponse{}, err
	}

	summary := memberSummaryResponse{OK: true, Email: email}
	if err := tx.QueryRowContext(ctx, `
		SELECT plan_id, outputs_total, outputs_remaining
		FROM entitlements
		WHERE lower(email) = $1 AND status = 'active'
		ORDER BY outputs_remaining DESC
		LIMIT 1`, email).Scan(&summary.Plan, &summary.OutputsTotal, &summary.OutputsRemaining); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return memberSummaryResponse{}, errors.New("active entitlement missing after initialization")
		}
		return memberSummaryResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return memberSummaryResponse{}, err
	}
	return summary, nil
}
