package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const koschScanQuotaKind = "scan"

type koschQuotaStatus struct {
	Tier       string    `json:"tier"`
	DailyLimit int       `json:"quota_daily"`
	UsedToday  int       `json:"quota_used_today"`
	Remaining  int       `json:"quota_remaining_today"`
	ResetsAt   time.Time `json:"quota_resets_at"`
}

type koschQuotaExceededError struct {
	Status koschQuotaStatus
}

func (e koschQuotaExceededError) Error() string { return "quota_exceeded" }

type quotaResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *quotaResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *quotaResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

func (w *quotaResponseWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func configuredKOSCHDailyQuota(tier string) int {
	tier = strings.ToLower(strings.TrimSpace(tier))
	name := ""
	fallback := 0
	switch tier {
	case "basic":
		name, fallback = "KOSCHEI_QUOTA_BASIC_DAILY", 5
	case "pro":
		name, fallback = "KOSCHEI_QUOTA_PRO_DAILY", 100
	case "enterprise":
		name, fallback = "KOSCHEI_QUOTA_ENTERPRISE_DAILY", 1000
	default:
		return 0
	}
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > 1000000 {
		return fallback
	}
	return value
}

func quotaUTCWindow(now time.Time) (time.Time, time.Time) {
	now = now.UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return start, start.Add(24 * time.Hour)
}

type scanQuotaIdentity func(*http.Request) (string, string, error)
type scanQuotaReserve func(context.Context, string, string, string) (premiumOutputReservation, koschQuotaStatus, error)
type scanQuotaRefund func(context.Context, premiumOutputReservation, string) error

func (h *Handler) EnforceScanQuota(next http.HandlerFunc) http.HandlerFunc {
	identity := func(r *http.Request) (string, string, error) {
		claims, ok := userFromContext(r.Context())
		if !ok {
			return "", "", errors.New("unauthorized")
		}
		if evaluation, ok := tokenAccessFromContext(r.Context()); ok {
			return claims.Sub, evaluation.Tier, nil
		}
		evaluation, err := h.evaluateTokenAccess(r.Context(), claims.Sub)
		if err != nil {
			return "", "", err
		}
		return claims.Sub, evaluation.Tier, nil
	}
	return enforceScanQuotaWith(identity, h.reserveKOSCHDailyQuota, h.refundPremiumOutputReservation, next)
}

func (h *Handler) EnforceAPIKeyScanQuota(next http.HandlerFunc) http.HandlerFunc {
	identity := func(r *http.Request) (string, string, error) {
		principal, ok := apiPrincipalFromContext(r.Context())
		if !ok {
			return "", "", errors.New("unauthorized")
		}
		if evaluation, ok := tokenAccessFromContext(r.Context()); ok {
			return principal.AuthSubject, evaluation.Tier, nil
		}
		evaluation, err := h.evaluateTokenAccess(r.Context(), principal.AuthSubject)
		if err != nil {
			return "", "", err
		}
		return principal.AuthSubject, evaluation.Tier, nil
	}
	return enforceScanQuotaWith(identity, h.reserveKOSCHDailyQuota, h.refundPremiumOutputReservation, next)
}

func enforceScanQuotaWith(identity scanQuotaIdentity, reserve scanQuotaReserve, refund scanQuotaRefund, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authSubject, tier, err := identity(r)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "quota_unavailable"})
			return
		}
		reservation, status, err := reserve(r.Context(), authSubject, tier, r.Method+" "+r.URL.Path)
		if err != nil {
			var exceeded koschQuotaExceededError
			if errors.As(err, &exceeded) {
				writeJSON(w, http.StatusTooManyRequests, map[string]any{
					"error":     "quota_exceeded",
					"tier":      exceeded.Status.Tier,
					"limit":     exceeded.Status.DailyLimit,
					"used":      exceeded.Status.UsedToday,
					"resets_at": exceeded.Status.ResetsAt,
				})
				return
			}
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "quota_unavailable"})
			return
		}

		recorder := &quotaResponseWriter{ResponseWriter: w}
		completed := false
		defer func() {
			if completed && recorder.Status() < http.StatusBadRequest {
				return
			}
			refundCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = refund(refundCtx, reservation, "kosch_scan_quota_refund")
		}()
		next(recorder, r.WithContext(context.WithValue(r.Context(), koschQuotaContextKey{}, status)))
		completed = true
	}
}

type koschQuotaContextKey struct{}

func koschQuotaFromContext(ctx context.Context) (koschQuotaStatus, bool) {
	status, ok := ctx.Value(koschQuotaContextKey{}).(koschQuotaStatus)
	return status, ok
}

func (h *Handler) reserveKOSCHDailyQuota(ctx context.Context, authSubject, tier, reason string) (premiumOutputReservation, koschQuotaStatus, error) {
	if h == nil || h.DB == nil {
		return premiumOutputReservation{}, koschQuotaStatus{}, errors.New("database unavailable")
	}
	tier = strings.ToLower(strings.TrimSpace(tier))
	limit := configuredKOSCHDailyQuota(tier)
	if limit <= 0 {
		return premiumOutputReservation{}, koschQuotaStatus{}, errors.New("token tier unavailable")
	}
	now := time.Now().UTC()
	day, reset := quotaUTCWindow(now)
	status := koschQuotaStatus{Tier: tier, DailyLimit: limit, ResetsAt: reset}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return premiumOutputReservation{}, status, err
	}
	defer tx.Rollback()

	email, err := activeProfileEmailTx(ctx, tx, authSubject)
	if err != nil {
		return premiumOutputReservation{}, status, err
	}
	planID := "kosch_daily_" + tier
	_, err = tx.ExecContext(ctx, `
		INSERT INTO entitlements
			(email, plan_id, outputs_total, outputs_remaining, status, expires_at,
			 quota_day, quota_tier, quota_kind, created_at, updated_at)
		VALUES (lower($1), $2, $3, $3, 'active', $4, $5::date, $6, $7, now(), now())
		ON CONFLICT (lower(email), quota_kind, quota_day)
		WHERE quota_day IS NOT NULL AND quota_kind IS NOT NULL
		DO UPDATE SET
			plan_id = EXCLUDED.plan_id,
			quota_tier = EXCLUDED.quota_tier,
			outputs_remaining = GREATEST(EXCLUDED.outputs_total - GREATEST(entitlements.outputs_total - entitlements.outputs_remaining, 0), 0),
			outputs_total = EXCLUDED.outputs_total,
			status = 'active',
			expires_at = EXCLUDED.expires_at,
			updated_at = now()`, email, planID, limit, reset, day, tier, koschScanQuotaKind)
	if err != nil {
		return premiumOutputReservation{}, status, err
	}

	var entitlementID, normalizedEmail string
	var remaining int
	err = tx.QueryRowContext(ctx, `
		UPDATE entitlements
		SET outputs_remaining = outputs_remaining - 1,
		    updated_at = now()
		WHERE id = (
			SELECT id
			FROM entitlements
			WHERE lower(email)=lower($1)
			  AND quota_day=$2::date
			  AND quota_kind=$3
			  AND status='active'
			  AND outputs_remaining > 0
			  AND expires_at > now()
			FOR UPDATE
		)
		RETURNING id::text, lower(email), outputs_remaining`, email, day, koschScanQuotaKind).Scan(&entitlementID, &normalizedEmail, &remaining)
	if errors.Is(err, sql.ErrNoRows) {
		var total, currentRemaining int
		_ = tx.QueryRowContext(ctx, `
			SELECT outputs_total, outputs_remaining
			FROM entitlements
			WHERE lower(email)=lower($1) AND quota_day=$2::date AND quota_kind=$3`, email, day, koschScanQuotaKind).Scan(&total, &currentRemaining)
		if total > 0 {
			status.DailyLimit = total
		}
		status.Remaining = currentRemaining
		status.UsedToday = status.DailyLimit - currentRemaining
		return premiumOutputReservation{}, status, koschQuotaExceededError{Status: status}
	}
	if err != nil {
		return premiumOutputReservation{}, status, err
	}
	if strings.TrimSpace(reason) == "" {
		reason = "kosch_scan_quota"
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO credit_events(email,amount,reason,event_type) VALUES(lower($1),-1,$2,'reserve')`, normalizedEmail, reason+"_reserved"); err != nil {
		return premiumOutputReservation{}, status, err
	}
	if err := tx.Commit(); err != nil {
		return premiumOutputReservation{}, status, err
	}
	status.Remaining = remaining
	status.UsedToday = status.DailyLimit - remaining
	return premiumOutputReservation{EntitlementID: entitlementID, Email: normalizedEmail, Reason: reason}, status, nil
}

func (h *Handler) currentKOSCHDailyQuota(ctx context.Context, authSubject, tier string) (koschQuotaStatus, error) {
	if h == nil || h.DB == nil {
		return koschQuotaStatus{}, errors.New("database unavailable")
	}
	tier = strings.ToLower(strings.TrimSpace(tier))
	limit := configuredKOSCHDailyQuota(tier)
	now := time.Now().UTC()
	day, reset := quotaUTCWindow(now)
	status := koschQuotaStatus{Tier: tier, DailyLimit: limit, Remaining: limit, ResetsAt: reset}
	if limit <= 0 {
		return status, nil
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return status, err
	}
	defer tx.Rollback()
	email, err := activeProfileEmailTx(ctx, tx, authSubject)
	if err != nil {
		return status, err
	}
	planID := "kosch_daily_" + tier
	var total, remaining int
	err = tx.QueryRowContext(ctx, `
		INSERT INTO entitlements
			(email, plan_id, outputs_total, outputs_remaining, status, expires_at,
			 quota_day, quota_tier, quota_kind, created_at, updated_at)
		VALUES (lower($1), $2, $3, $3, 'active', $4, $5::date, $6, $7, now(), now())
		ON CONFLICT (lower(email), quota_kind, quota_day)
		WHERE quota_day IS NOT NULL AND quota_kind IS NOT NULL
		DO UPDATE SET
			plan_id = EXCLUDED.plan_id,
			quota_tier = EXCLUDED.quota_tier,
			outputs_remaining = GREATEST(EXCLUDED.outputs_total - GREATEST(entitlements.outputs_total - entitlements.outputs_remaining, 0), 0),
			outputs_total = EXCLUDED.outputs_total,
			status = 'active',
			expires_at = EXCLUDED.expires_at,
			updated_at = now()
		RETURNING outputs_total, outputs_remaining`, email, planID, limit, reset, day, tier, koschScanQuotaKind).Scan(&total, &remaining)
	if err != nil {
		return status, err
	}
	if err := tx.Commit(); err != nil {
		return status, err
	}
	status.DailyLimit = total
	status.Remaining = remaining
	status.UsedToday = total - remaining
	return status, nil
}

func activeProfileEmailTx(ctx context.Context, tx *sql.Tx, authSubject string) (string, error) {
	var email string
	err := tx.QueryRowContext(ctx, `
		SELECT lower(email)
		FROM app_user_profiles
		WHERE auth_subject=$1 AND status='active'
		ORDER BY updated_at DESC, created_at DESC
		LIMIT 1`, strings.TrimSpace(authSubject)).Scan(&email)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("active profile required")
	}
	return email, err
}
