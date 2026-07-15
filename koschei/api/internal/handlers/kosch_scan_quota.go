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

type koschQuotaStatus struct {
	Tier      string    `json:"tier"`
	Daily     int       `json:"quota_daily"`
	Used      int       `json:"quota_used_today"`
	Remaining int       `json:"quota_remaining_today"`
	ResetsAt  time.Time `json:"quota_resets_at"`
}

type koschQuotaReservation struct {
	ID          string
	AuthSubject string
	QuotaDate   time.Time
	Tier        string
	Limit       int
	Reason      string
}

type quotaResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *quotaResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *quotaResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func configuredKOSCHDailyQuota(tier string) int {
	tier = strings.ToLower(strings.TrimSpace(tier))
	name := ""
	fallback := 0
	switch tier {
	case "enterprise":
		name, fallback = "KOSCHEI_QUOTA_ENTERPRISE_DAILY", 1000
	case "pro":
		name, fallback = "KOSCHEI_QUOTA_PRO_DAILY", 100
	case "basic":
		name, fallback = "KOSCHEI_QUOTA_BASIC_DAILY", 5
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

func utcQuotaWindow(now time.Time) (time.Time, time.Time) {
	now = now.UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return start, start.Add(24 * time.Hour)
}

func quotaAuthSubject(ctx context.Context) string {
	if claims, ok := userFromContext(ctx); ok {
		return strings.TrimSpace(claims.Sub)
	}
	if principal, ok := apiPrincipalFromContext(ctx); ok {
		return strings.TrimSpace(principal.AuthSubject)
	}
	return ""
}

func (h *Handler) currentQuotaTier(ctx context.Context, authSubject string) (string, error) {
	if h == nil || h.DB == nil {
		return "", errors.New("database unavailable")
	}
	authSubject = strings.TrimSpace(authSubject)
	if authSubject == "" {
		return "", errors.New("auth subject required")
	}
	var tier string
	err := h.DB.QueryRowContext(ctx, `
		SELECT tier
		FROM token_access_snapshots
		WHERE auth_subject=$1 AND gate_enabled=true AND expires_at > now()
		ORDER BY checked_at DESC
		LIMIT 1`, authSubject).Scan(&tier)
	if err == nil {
		return strings.ToLower(strings.TrimSpace(tier)), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	evaluation, err := h.evaluateTokenAccess(ctx, authSubject)
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(evaluation.Tier)), nil
}

func (h *Handler) reserveKOSCHScanQuota(ctx context.Context, authSubject, tier, reason string, now time.Time) (koschQuotaReservation, koschQuotaStatus, error) {
	if h == nil || h.DB == nil {
		return koschQuotaReservation{}, koschQuotaStatus{}, errors.New("database unavailable")
	}
	authSubject = strings.TrimSpace(authSubject)
	tier = strings.ToLower(strings.TrimSpace(tier))
	limit := configuredKOSCHDailyQuota(tier)
	start, reset := utcQuotaWindow(now)
	status := koschQuotaStatus{Tier: tier, Daily: limit, ResetsAt: reset}
	if authSubject == "" || limit <= 0 {
		return koschQuotaReservation{}, status, errors.New("eligible KOSCH tier required")
	}
	if reason = strings.TrimSpace(reason); reason == "" {
		reason = "scan"
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return koschQuotaReservation{}, status, err
	}
	defer tx.Rollback()

	var used int
	err = tx.QueryRowContext(ctx, `
		INSERT INTO kosch_daily_quota_usage
			(auth_subject,quota_date,tier,quota_limit,used_count,created_at,updated_at)
		VALUES ($1,$2,$3,$4,1,now(),now())
		ON CONFLICT (auth_subject,quota_date) DO UPDATE SET
			tier=EXCLUDED.tier,
			quota_limit=EXCLUDED.quota_limit,
			used_count=kosch_daily_quota_usage.used_count+1,
			updated_at=now()
		WHERE kosch_daily_quota_usage.used_count < EXCLUDED.quota_limit
		RETURNING used_count`, authSubject, start, tier, limit).Scan(&used)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.QueryRowContext(ctx, `SELECT used_count FROM kosch_daily_quota_usage WHERE auth_subject=$1 AND quota_date=$2`, authSubject, start).Scan(&used)
		status.Used = used
		status.Remaining = maxQuotaInt(limit-used, 0)
		return koschQuotaReservation{}, status, tokenAccessError{Status: http.StatusTooManyRequests, Code: "quota_exceeded"}
	}
	if err != nil {
		return koschQuotaReservation{}, status, err
	}

	var reservationID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO kosch_daily_quota_reservations
			(auth_subject,quota_date,tier,quota_limit,reason,status)
		VALUES ($1,$2,$3,$4,$5,'reserved')
		RETURNING id::text`, authSubject, start, tier, limit, reason).Scan(&reservationID)
	if err != nil {
		return koschQuotaReservation{}, status, err
	}
	if err := tx.Commit(); err != nil {
		return koschQuotaReservation{}, status, err
	}
	status.Used = used
	status.Remaining = maxQuotaInt(limit-used, 0)
	return koschQuotaReservation{ID: reservationID, AuthSubject: authSubject, QuotaDate: start, Tier: tier, Limit: limit, Reason: reason}, status, nil
}

func (h *Handler) finalizeKOSCHScanQuota(ctx context.Context, reservation koschQuotaReservation) error {
	if h == nil || h.DB == nil || strings.TrimSpace(reservation.ID) == "" {
		return nil
	}
	_, err := h.DB.ExecContext(ctx, `
		UPDATE kosch_daily_quota_reservations
		SET status='consumed',finalized_at=now()
		WHERE id=$1::uuid AND status='reserved'`, reservation.ID)
	return err
}

func (h *Handler) refundKOSCHScanQuota(ctx context.Context, reservation koschQuotaReservation, reason string) error {
	if h == nil || h.DB == nil || strings.TrimSpace(reservation.ID) == "" {
		return nil
	}
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
		UPDATE kosch_daily_quota_reservations
		SET status='refunded',reason=CASE WHEN $2='' THEN reason ELSE $2 END,finalized_at=now()
		WHERE id=$1::uuid AND status='reserved'`, reservation.ID, strings.TrimSpace(reason))
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil || rows == 0 {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE kosch_daily_quota_usage
		SET used_count=GREATEST(used_count-1,0),updated_at=now()
		WHERE auth_subject=$1 AND quota_date=$2`, reservation.AuthSubject, reservation.QuotaDate)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (h *Handler) KOSCHQuotaStatus(ctx context.Context, authSubject, tier string, now time.Time) (koschQuotaStatus, error) {
	start, reset := utcQuotaWindow(now)
	limit := configuredKOSCHDailyQuota(tier)
	status := koschQuotaStatus{Tier: tier, Daily: limit, ResetsAt: reset}
	if h == nil || h.DB == nil || strings.TrimSpace(authSubject) == "" || limit <= 0 {
		return status, nil
	}
	var used int
	err := h.DB.QueryRowContext(ctx, `SELECT used_count FROM kosch_daily_quota_usage WHERE auth_subject=$1 AND quota_date=$2`, authSubject, start).Scan(&used)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return status, err
	}
	status.Used = used
	status.Remaining = maxQuotaInt(limit-used, 0)
	return status, nil
}

func (h *Handler) EnforceScanQuota(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authSubject := quotaAuthSubject(r.Context())
		if authSubject == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		tier, err := h.currentQuotaTier(r.Context(), authSubject)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "quota_unavailable"})
			return
		}
		reason := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		reservation, status, err := h.reserveKOSCHScanQuota(r.Context(), authSubject, tier, reason, time.Now().UTC())
		if accessErr, ok := err.(tokenAccessError); ok && accessErr.Code == "quota_exceeded" {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error": "quota_exceeded", "tier": status.Tier, "limit": status.Daily,
				"used": status.Used, "resets_at": status.ResetsAt,
			})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "quota_unavailable"})
			return
		}

		writer := &quotaResponseWriter{ResponseWriter: w}
		defer func() {
			if recovered := recover(); recovered != nil {
				_ = h.refundKOSCHScanQuota(context.Background(), reservation, "handler_panic_refund")
				panic(recovered)
			}
			statusCode := writer.status
			if statusCode == 0 {
				statusCode = http.StatusOK
			}
			if statusCode >= 400 {
				_ = h.refundKOSCHScanQuota(context.Background(), reservation, fmt.Sprintf("http_%d_refund", statusCode))
			} else {
				_ = h.finalizeKOSCHScanQuota(context.Background(), reservation)
			}
		}()
		next(writer, r)
	}
}

func (h *Handler) RequireAPIKeyTokenTier(required string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := apiPrincipalFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		evaluation, err := h.evaluateTokenAccess(r.Context(), principal.AuthSubject)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosch_access_unavailable"})
			return
		}
		if tokenTierRank(evaluation.Tier) < tokenTierRank(required) {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error": "token_tier_required", "required_tier": required, "current_tier": evaluation.Tier,
			})
			return
		}
		next(w, r)
	}
}

func maxQuotaInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
