package handlers

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"time"

	"koschei/api/internal/router"
)

var errAccountDisabled = errors.New("account disabled")

type appProfileStore interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type sqlTxStore struct{ tx *sql.Tx }

func (s sqlTxStore) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	return s.tx.ExecContext(ctx, q, args...)
}
func (s sqlTxStore) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	return s.tx.QueryRowContext(ctx, q, args...)
}

type authUser struct {
	ID          string
	AuthSubject string
	Email       string
	Role        string
	PlanID      string
	Plan        string
	Credits     int
}

func upsertAppProfileTx(ctx context.Context, store appProfileStore, sub, email string, _ *authUser) error {
	res, err := store.ExecContext(ctx, `
		UPDATE app_user_profiles
		SET email = lower($2),
			banned_at = NULL,
			ban_reason = NULL,
			updated_at = now()
		WHERE auth_subject = $1
		  AND status = 'active'`, sub, email)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}

	var blocked bool
	if err := store.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM app_user_profiles WHERE auth_subject=$1 AND status IN ('banned','removed'))`, sub).Scan(&blocked); err != nil {
		return err
	}
	if blocked {
		return errAccountDisabled
	}

	res, err = store.ExecContext(ctx, `
		WITH chosen AS (
			SELECT id
			FROM app_user_profiles
			WHERE lower(email) = lower($2)
			  AND status = 'active'
			ORDER BY updated_at DESC, created_at DESC
			LIMIT 1
		)
		UPDATE app_user_profiles
		SET auth_subject = $1,
			banned_at = NULL,
			ban_reason = NULL,
			updated_at = now()
		WHERE id = (SELECT id FROM chosen)`, sub, email)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}

	if err := store.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM app_user_profiles WHERE lower(email)=lower($1) AND status IN ('banned','removed'))`, email).Scan(&blocked); err != nil {
		return err
	}
	if blocked {
		return errAccountDisabled
	}

	_, err = store.ExecContext(ctx, `
		INSERT INTO app_user_profiles (auth_subject,email,plan_id,credits,status,created_at,updated_at)
		VALUES ($1,lower($2),'free',0,'active',now(),now())
		ON CONFLICT (auth_subject) DO NOTHING`, sub, email)
	return err
}

func insufficientOutputsResponse() map[string]any {
	return map[string]any{"error": "active_entitlement_required", "message": "Active Koschei package entitlement is required."}
}

func generateID() string {
	id, err := newUUID()
	if err != nil {
		return "fallback"
	}
	return id
}

func (h *Handler) upsertAppProfile(ctx context.Context, subject, email string) (authUser, error) {
	profile := authUser{}
	err := h.DB.QueryRowContext(ctx, `
		UPDATE app_user_profiles
		SET email = lower($2),
			banned_at = NULL,
			ban_reason = NULL,
			updated_at = now()
		WHERE auth_subject = $1
		  AND status = 'active'
		RETURNING id::text, auth_subject, email, COALESCE(plan_id,''), COALESCE(credits,0)`, subject, email).Scan(&profile.ID, &profile.AuthSubject, &profile.Email, &profile.PlanID, &profile.Credits)
	if err == nil {
		profile.Plan = profile.PlanID
		return profile, nil
	}
	if err != sql.ErrNoRows {
		return profile, err
	}

	var blocked bool
	if err := h.DB.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM app_user_profiles WHERE auth_subject=$1 AND status IN ('banned','removed'))`, subject).Scan(&blocked); err != nil {
		return profile, err
	}
	if blocked {
		return profile, errAccountDisabled
	}

	err = h.DB.QueryRowContext(ctx, `
		WITH chosen AS (
			SELECT id
			FROM app_user_profiles
			WHERE lower(email) = lower($2)
			  AND status = 'active'
			ORDER BY updated_at DESC, created_at DESC
			LIMIT 1
		)
		UPDATE app_user_profiles
		SET auth_subject = $1,
			banned_at = NULL,
			ban_reason = NULL,
			updated_at = now()
		WHERE id = (SELECT id FROM chosen)
		RETURNING id::text, auth_subject, email, COALESCE(plan_id,''), COALESCE(credits,0)`, subject, email).Scan(&profile.ID, &profile.AuthSubject, &profile.Email, &profile.PlanID, &profile.Credits)
	if err == nil {
		profile.Plan = profile.PlanID
		return profile, nil
	}
	if err != sql.ErrNoRows {
		return profile, err
	}

	if err := h.DB.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM app_user_profiles WHERE lower(email)=lower($1) AND status IN ('banned','removed'))`, email).Scan(&blocked); err != nil {
		return profile, err
	}
	if blocked {
		return profile, errAccountDisabled
	}

	err = h.DB.QueryRowContext(ctx, `
		INSERT INTO app_user_profiles (auth_subject,email,plan_id,credits,status,created_at,updated_at)
		VALUES ($1,lower($2),'free',0,'active',now(),now())
		ON CONFLICT (auth_subject) DO NOTHING
		RETURNING id::text, auth_subject, email, COALESCE(plan_id,''), COALESCE(credits,0)`, subject, email).Scan(&profile.ID, &profile.AuthSubject, &profile.Email, &profile.PlanID, &profile.Credits)
	if errors.Is(err, sql.ErrNoRows) {
		return profile, errors.New("profile conflict")
	}
	profile.Plan = profile.PlanID
	return profile, err
}

func tokenLooksLikeJWT(token string) bool { return len(strings.Split(token, ".")) == 3 }

func shortError(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 240 {
		return s[:240]
	}
	return s
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

func (h *Handler) callTogetherWithSystemTimeoutAndMaxTokens(model, systemPrompt, userPrompt string, timeout time.Duration, maxTokens int) (string, error) {
	resp, err := router.Chat(context.Background(), router.ChatRequest{System: systemPrompt, Prompt: userPrompt, Model: model, Timeout: timeout, MaxTokens: maxTokens, Temperature: 0.2})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}
