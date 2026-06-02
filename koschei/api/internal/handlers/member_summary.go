package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log"
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
		log.Printf("member summary failed: sub=%s email=%s err=%v", claims.Sub, claims.Email, err)
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
		WITH updated_by_subject AS (
			UPDATE app_user_profiles
			SET email = lower($2), updated_at = now()
			WHERE auth_subject = $1
			RETURNING id
		),
		updated_by_email AS (
			UPDATE app_user_profiles
			SET auth_subject = $1, updated_at = now()
			WHERE lower(email) = lower($2)
			  AND NOT EXISTS (SELECT 1 FROM updated_by_subject)
			RETURNING id
		),
		inserted AS (
			INSERT INTO app_user_profiles (auth_subject, email)
			SELECT $1, lower($2)
			WHERE NOT EXISTS (SELECT 1 FROM updated_by_subject)
			  AND NOT EXISTS (SELECT 1 FROM updated_by_email)
			RETURNING id
		)
		SELECT id FROM updated_by_subject
		UNION ALL SELECT id FROM updated_by_email
		UNION ALL SELECT id FROM inserted
		LIMIT 1`, sub, email); err != nil {
		return memberSummaryResponse{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO entitlements (email, plan_id, outputs_total, outputs_remaining, status)
		SELECT
			lower($1),
			(SELECT id FROM plans WHERE id = 'free' LIMIT 1),
			100,
			100,
			'active'
		WHERE NOT EXISTS (
			SELECT 1
			FROM entitlements
			WHERE lower(email) = lower($1)
			  AND status = 'active'
		)`, email); err != nil {
		return memberSummaryResponse{}, err
	}

	summary := memberSummaryResponse{OK: true, Email: email}
	if err := tx.QueryRowContext(ctx, `
		SELECT
			COALESCE(plan_id, 'free') AS plan_id,
			COALESCE(outputs_total, 100) AS outputs_total,
			COALESCE(outputs_remaining, 100) AS outputs_remaining
		FROM entitlements
		WHERE lower(email) = lower($1)
		  AND status = 'active'
		ORDER BY outputs_remaining DESC, created_at DESC
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
