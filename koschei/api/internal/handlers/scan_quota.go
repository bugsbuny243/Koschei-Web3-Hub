package handlers

import (
	"context"
	"crypto/rand"
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

type scanQuotaLedger interface {
	Reserve(context.Context, string, string, string, int, time.Time) (premiumOutputReservation, scanQuotaStatus, error)
	Refund(context.Context, premiumOutputReservation) error
	Status(context.Context, string, string, int, time.Time) (scanQuotaStatus, error)
}

type handlerScanQuotaLedger struct {
	Handler *Handler
}

func (l handlerScanQuotaLedger) Reserve(ctx context.Context, authSubject, email, tier string, limit int, now time.Time) (premiumOutputReservation, scanQuotaStatus, error) {
	if l.Handler == nil {
		return premiumOutputReservation{}, newScanQuotaStatus(tier, limit, now), errors.New("handler unavailable")
	}
	return l.Handler.reserveKOSCHScanQuota(ctx, authSubject, email, tier, limit, now)
}

func (l handlerScanQuotaLedger) Refund(ctx context.Context, reservation premiumOutputReservation) error {
	if l.Handler == nil {
		return nil
	}
	return l.Handler.refundPremiumOutputReservation(ctx, reservation, "kosch_daily_scan_refund")
}

func (l handlerScanQuotaLedger) Status(ctx context.Context, email, tier string, limit int, now time.Time) (scanQuotaStatus, error) {
	if l.Handler == nil {
		return newScanQuotaStatus(tier, limit, now), nil
	}
	return l.Handler.koschScanQuotaStatus(ctx, email, tier, limit, now)
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
	return enforceScanQuota(handlerScanQuotaLedger{Handler: h}, next)
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

		reservation, status, err := ledger.Reserve(r.Context(), access.AuthSubject, email, tier, limit, time.Now().UTC())
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

func refundQuotaDetached(ledger scanQuotaLedger, reservation premiumOutputReservation) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = ledger.Refund(ctx, reservation)
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
