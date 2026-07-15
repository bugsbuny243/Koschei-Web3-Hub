package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var errScanQuotaExceeded = errors.New("scan quota exceeded")

type tokenAccessRequestContext struct {
	Evaluation  tokenAccessEvaluation
	AuthSubject string
	Email       string
}

type tokenAccessRequestContextKey struct{}

type scanQuotaStatus struct {
	Tier      string    `json:"tier"`
	Limit     int       `json:"limit"`
	Used      int       `json:"used"`
	Remaining int       `json:"remaining"`
	ResetsAt  time.Time `json:"resets_at"`
}

type scanQuotaReservation struct {
	Email       string
	DayKey      string
	EventReason string
	Tier        string
	Limit       int
	ResetsAt    time.Time
}

type scanQuotaLedger interface {
	Reserve(context.Context, string, string, int, time.Time) (scanQuotaReservation, scanQuotaStatus, error)
	Refund(context.Context, scanQuotaReservation) error
	Status(context.Context, string, string, int, time.Time) (scanQuotaStatus, error)
}

type postgresScanQuotaLedger struct {
	DB *sql.DB
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
	if err != nil || value < 1 || value > 1_000_000 {
		return fallback
	}
	return value
}

func utcQuotaWindow(now time.Time) (time.Time, time.Time, string) {
	now = now.UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	reset := start.Add(24 * time.Hour)
	return start, reset, "kosch_daily_scan:" + start.Format("2006-01-02")
}

func withTokenAccessRequestContext(ctx context.Context, value tokenAccessRequestContext) context.Context {
	return context.WithValue(ctx, tokenAccessRequestContextKey{}, value)
}

func tokenAccessRequestFromContext(ctx context.Context) (tokenAccessRequestContext, bool) {
	value, ok := ctx.Value(tokenAccessRequestContextKey{}).(tokenAccessRequestContext)
	return value, ok
}

func (h *Handler) EnforceScanQuota(next http.HandlerFunc) http.HandlerFunc {
	return enforceScanQuota(postgresScanQuotaLedger{DB: h.DB}, next)
}

func enforceScanQuota(ledger scanQuotaLedger, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		access, ok := tokenAccessRequestFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "quota_context_unavailable"})
			return
		}
		tier := strings.ToLower(strings.TrimSpace(access.Evaluation.Tier))
		limit := configuredKOSCHDailyQuota(tier)
		if limit <= 0 {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error": "token_tier_required", "required_tier": "basic", "current_tier": tier,
			})
			return
		}
		email := strings.ToLower(strings.TrimSpace(access.Email))
		if email == "" {
			email = entitlementEmailFromSubject(access.AuthSubject)
		}
		if email == "" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "quota_identity_unavailable"})
			return
		}

		reservation, status, err := ledger.Reserve(r.Context(), email, tier, limit, time.Now().UTC())
		if errors.Is(err, errScanQuotaExceeded) {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error": "quota_exceeded", "tier": tier, "limit": status.Limit,
				"used": status.Used, "remaining": 0, "resets_at": status.ResetsAt,
			})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "quota_unavailable"})
			return
		}
		w.Header().Set("X-Koschei-Quota-Limit", strconv.Itoa(status.Limit))
		w.Header().Set("X-Koschei-Quota-Remaining", strconv.Itoa(status.Remaining))
		w.Header().Set("X-Koschei-Quota-Reset", status.ResetsAt.Format(time.RFC3339))

		tracker := &quotaResponseWriter{ResponseWriter: w, status: http.StatusOK}
		succeeded := false
		defer func() {
			if recovered := recover(); recovered != nil {
				refundQuotaDetached(ledger, reservation)
				panic(recovered)
			}
			if !succeeded {
				refundQuotaDetached(ledger, reservation)
			}
		}()
		next(tracker, r)
		succeeded = tracker.status >= 200 && tracker.status < 400
	}
}

type quotaResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *quotaResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *quotaResponseWriter) Write(body []byte) (int, error) {
	return w.ResponseWriter.Write(body)
}

func refundQuotaDetached(ledger scanQuotaLedger, reservation scanQuotaReservation) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = ledger.Refund(ctx, reservation)
}

func (p postgresScanQuotaLedger) Reserve(ctx context.Context, email, tier string, limit int, now time.Time) (scanQuotaReservation, scanQuotaStatus, error) {
	status := newScanQuotaStatus(tier, limit, now)
	if p.DB == nil {
		return scanQuotaReservation{}, status, errors.New("database unavailable")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || limit <= 0 {
		return scanQuotaReservation{}, status, errors.New("invalid quota reservation")
	}
	start, reset, dayKey := utcQuotaWindow(now)
	status.ResetsAt = reset
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return scanQuotaReservation{}, status, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(lower($1)||':'||$2,0))`, email, dayKey); err != nil {
		return scanQuotaReservation{}, status, err
	}
	used, err := quotaUsedTx(ctx, tx, email, dayKey, start, reset)
	if err != nil {
		return scanQuotaReservation{}, status, err
	}
	status.Used = used
	status.Remaining = maxInt(limit-used, 0)
	if used >= limit {
		return scanQuotaReservation{}, status, errScanQuotaExceeded
	}
	id, err := quotaReservationID()
	if err != nil {
		return scanQuotaReservation{}, status, err
	}
	reason := dayKey + ":" + id
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO credit_events(email,amount,reason,event_type)
		VALUES(lower($1),-1,$2,'kosch_quota_reserve')`, email, reason); err != nil {
		return scanQuotaReservation{}, status, err
	}
	if err := tx.Commit(); err != nil {
		return scanQuotaReservation{}, status, err
	}
	status.Used++
	status.Remaining = maxInt(limit-status.Used, 0)
	return scanQuotaReservation{Email: email, DayKey: dayKey, EventReason: reason, Tier: tier, Limit: limit, ResetsAt: reset}, status, nil
}

func (p postgresScanQuotaLedger) Refund(ctx context.Context, reservation scanQuotaReservation) error {
	if p.DB == nil || strings.TrimSpace(reservation.EventReason) == "" || strings.TrimSpace(reservation.Email) == "" {
		return nil
	}
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(lower($1)||':'||$2,0))`, reservation.Email, reservation.DayKey); err != nil {
		return err
	}
	var refunded bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM credit_events
			WHERE lower(email)=lower($1) AND reason=$2 AND event_type='kosch_quota_refund'
		)`, reservation.Email, reservation.EventReason).Scan(&refunded); err != nil {
		return err
	}
	if refunded {
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO credit_events(email,amount,reason,event_type)
		VALUES(lower($1),1,$2,'kosch_quota_refund')`, reservation.Email, reservation.EventReason); err != nil {
		return err
	}
	return tx.Commit()
}

func (p postgresScanQuotaLedger) Status(ctx context.Context, email, tier string, limit int, now time.Time) (scanQuotaStatus, error) {
	status := newScanQuotaStatus(tier, limit, now)
	if p.DB == nil || strings.TrimSpace(email) == "" || limit <= 0 {
		return status, nil
	}
	start, reset, dayKey := utcQuotaWindow(now)
	status.ResetsAt = reset
	var used int
	err := p.DB.QueryRowContext(ctx, `
		SELECT GREATEST(COALESCE(-SUM(amount),0),0)::int
		FROM credit_events
		WHERE lower(email)=lower($1)
		  AND reason LIKE $2 || ':%'
		  AND created_at >= $3 AND created_at < $4
		  AND event_type IN ('kosch_quota_reserve','kosch_quota_refund')`, email, dayKey, start, reset).Scan(&used)
	if err != nil {
		return status, err
	}
	status.Used = used
	status.Remaining = maxInt(limit-used, 0)
	return status, nil
}

func quotaUsedTx(ctx context.Context, tx *sql.Tx, email, dayKey string, start, reset time.Time) (int, error) {
	var used int
	err := tx.QueryRowContext(ctx, `
		SELECT GREATEST(COALESCE(-SUM(amount),0),0)::int
		FROM credit_events
		WHERE lower(email)=lower($1)
		  AND reason LIKE $2 || ':%'
		  AND created_at >= $3 AND created_at < $4
		  AND event_type IN ('kosch_quota_reserve','kosch_quota_refund')`, email, dayKey, start, reset).Scan(&used)
	return used, err
}

func newScanQuotaStatus(tier string, limit int, now time.Time) scanQuotaStatus {
	_, reset, _ := utcQuotaWindow(now)
	return scanQuotaStatus{Tier: tier, Limit: limit, Remaining: maxInt(limit, 0), ResetsAt: reset}
}

func quotaReservationID() (string, error) {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("quota reservation id: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}
