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
	freeOutputsIncluded = 0
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

	summary, err := provisionMemberTx(ctx, sqlTxStore{tx: tx}, sub, email)
	if err != nil {
		return memberSummaryResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return memberSummaryResponse{}, err
	}
	return summary, nil
}

func provisionMemberTx(ctx context.Context, store appProfileStore, sub, email string) (memberSummaryResponse, error) {
	// Serialize profile and entitlement initialization per normalized identity so
	// concurrent provisioning requests cannot create duplicate app profiles or
	// multiple active zero-output free records. Premium output access is never granted here.
	profile := authUser{}
	if err := upsertAppProfileTx(ctx, store, sub, email, &profile); err != nil {
		return memberSummaryResponse{}, err
	}
	if _, err := store.ExecContext(ctx, `
		INSERT INTO entitlements (email, plan_id, outputs_total, outputs_remaining, status)
		SELECT lower($1), 'free', $2, $2, 'active'
		WHERE NOT EXISTS (
			SELECT 1
			FROM entitlements
			WHERE lower(email) = lower($1)
			  AND status = 'active'
			  AND COALESCE(plan_id, 'free') = 'free'
		)`, email, freeOutputsIncluded); err != nil {
		return memberSummaryResponse{}, err
	}

	summary := memberSummaryResponse{OK: true, Email: email}
	if err := store.QueryRowContext(ctx, `
		WITH active_entitlements AS (
			SELECT COALESCE(plan_id, 'free') AS plan_id,
			       COALESCE(outputs_total, 0) AS outputs_total,
			       COALESCE(outputs_remaining, 0) AS outputs_remaining,
			       created_at
			FROM entitlements
			WHERE lower(email) = lower($1)
			  AND status = 'active'
		), totals AS (
			SELECT COALESCE(SUM(outputs_total), 0)::int + COALESCE((SELECT credits FROM app_user_profiles WHERE auth_subject = $2), 0) AS outputs_total,
			       COALESCE(SUM(outputs_remaining), 0)::int + COALESCE((SELECT credits FROM app_user_profiles WHERE auth_subject = $2), 0) AS outputs_remaining
			FROM active_entitlements
		), paid_plan AS (
			SELECT plan_id
			FROM active_entitlements
			WHERE plan_id <> 'free'
			ORDER BY CASE plan_id WHEN 'studio' THEN 3 WHEN 'builder' THEN 2 WHEN 'starter' THEN 1 ELSE 0 END DESC,
			         created_at DESC
			LIMIT 1
		)
		SELECT CASE WHEN (SELECT plan_id FROM paid_plan) IS NOT NULL THEN (SELECT plan_id FROM paid_plan) WHEN COALESCE((SELECT credits FROM app_user_profiles WHERE auth_subject = $2), 0) > 0 THEN 'owner_grant' ELSE 'free' END AS plan_id,
		       outputs_total,
		       outputs_remaining
		FROM totals`, email, sub).Scan(&summary.Plan, &summary.OutputsTotal, &summary.OutputsRemaining); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return memberSummaryResponse{}, errors.New("active entitlement missing after initialization")
		}
		return memberSummaryResponse{}, err
	}
	return summary, nil
}
