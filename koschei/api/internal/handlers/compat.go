package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

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
	_, err := store.ExecContext(ctx, `
		INSERT INTO app_user_profiles (auth_subject,email,plan_id,credits,created_at,updated_at)
		VALUES ($1,lower($2),'free',0,now(),now())
		ON CONFLICT (auth_subject) DO UPDATE SET email=lower(EXCLUDED.email), updated_at=now()`, sub, email)
	return err
}

func insufficientOutputsResponse() map[string]any {
	return map[string]any{"error": "insufficient_credits", "message": "Yetersiz kredi. Lütfen kredi paketinizi yükseltin."}
}

func generateID() string {
	id, err := newUUID()
	if err != nil {
		return "fallback"
	}
	return id
}

var _ = http.MethodGet

func (h *Handler) upsertAppProfile(ctx context.Context, subject, email string) (authUser, error) {
	profile := authUser{}
	err := h.DB.QueryRowContext(ctx, `
		INSERT INTO app_user_profiles (auth_subject,email,plan_id,credits,created_at,updated_at)
		VALUES ($1,lower($2),'free',0,now(),now())
		ON CONFLICT (auth_subject) DO UPDATE SET email=lower(EXCLUDED.email), updated_at=now()
		RETURNING id::text, auth_subject, email, COALESCE(plan_id,''), COALESCE(credits,0)`, subject, email).Scan(&profile.ID, &profile.AuthSubject, &profile.Email, &profile.PlanID, &profile.Credits)
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
	if strings.TrimSpace(os.Getenv("TOGETHER_API_KEY")) == "" {
		return "", errors.New("together api key is not configured")
	}
	return "", errors.New("together client is disabled in test-safe build")
}

func isAdmin(r *http.Request) bool { _, err := r.Cookie("koschei_admin"); return err == nil }
