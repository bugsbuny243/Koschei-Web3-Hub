package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestConfiguredKOSCHDailyQuotaDefaultsAndOverrides(t *testing.T) {
	t.Setenv("KOSCHEI_QUOTA_BASIC_DAILY", "")
	t.Setenv("KOSCHEI_QUOTA_PRO_DAILY", "")
	t.Setenv("KOSCHEI_QUOTA_ENTERPRISE_DAILY", "")
	if got := configuredKOSCHDailyQuota("basic"); got != 5 {
		t.Fatalf("basic default = %d", got)
	}
	if got := configuredKOSCHDailyQuota("pro"); got != 100 {
		t.Fatalf("pro default = %d", got)
	}
	if got := configuredKOSCHDailyQuota("enterprise"); got != 1000 {
		t.Fatalf("enterprise default = %d", got)
	}
	t.Setenv("KOSCHEI_QUOTA_BASIC_DAILY", "17")
	if got := configuredKOSCHDailyQuota("basic"); got != 17 {
		t.Fatalf("basic override = %d", got)
	}
}

func TestQuotaUTCWindowResetsAtNextUTCMidnight(t *testing.T) {
	start, reset := quotaUTCWindow(time.Date(2026, 7, 15, 23, 59, 0, 0, time.FixedZone("other", 3*60*60)))
	if start.Location() != time.UTC || reset.Location() != time.UTC {
		t.Fatal("quota window must be UTC")
	}
	if !reset.Equal(start.Add(24 * time.Hour)) || start.Hour() != 0 || start.Minute() != 0 {
		t.Fatalf("unexpected UTC window: %s - %s", start, reset)
	}
}

func TestBasicTierCannotSatisfyProRequirement(t *testing.T) {
	if tokenTierRank("basic") >= tokenTierRank("pro") {
		t.Fatal("basic tier must not satisfy pro route requirement")
	}
	if tokenTierRank("enterprise") < tokenTierRank("pro") {
		t.Fatal("enterprise tier must satisfy pro route requirement")
	}
}

func TestQuotaExceededReturns429(t *testing.T) {
	reset := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	identity := func(*http.Request) (string, string, error) { return "subject", "basic", nil }
	reserve := func(context.Context, string, string, string) (premiumOutputReservation, koschQuotaStatus, error) {
		status := koschQuotaStatus{Tier: "basic", DailyLimit: 5, UsedToday: 5, Remaining: 0, ResetsAt: reset}
		return premiumOutputReservation{}, status, koschQuotaExceededError{Status: status}
	}
	refunded := 0
	refund := func(context.Context, premiumOutputReservation, string) error { refunded++; return nil }
	called := false
	handler := enforceScanQuotaWith(identity, reserve, refund, func(http.ResponseWriter, *http.Request) { called = true })

	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/radar/check", nil))
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if called || refunded != 0 {
		t.Fatalf("called=%t refunded=%d", called, refunded)
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["error"] != "quota_exceeded" || payload["tier"] != "basic" || int(payload["limit"].(float64)) != 5 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestFailedProtectedWorkRefundsReservation(t *testing.T) {
	identity := func(*http.Request) (string, string, error) { return "subject", "pro", nil }
	reservation := premiumOutputReservation{EntitlementID: "id", Email: "user@example.com", Reason: "scan"}
	reserve := func(context.Context, string, string, string) (premiumOutputReservation, koschQuotaStatus, error) {
		return reservation, koschQuotaStatus{Tier: "pro", DailyLimit: 100, UsedToday: 1, Remaining: 99}, nil
	}
	refunded := 0
	refund := func(_ context.Context, got premiumOutputReservation, reason string) error {
		if got.EntitlementID != reservation.EntitlementID || !strings.Contains(reason, "refund") {
			return errors.New("unexpected refund")
		}
		refunded++
		return nil
	}
	handler := enforceScanQuotaWith(identity, reserve, refund, func(w http.ResponseWriter, r *http.Request) {
		if status, ok := koschQuotaFromContext(r.Context()); !ok || status.Remaining != 99 {
			t.Fatal("quota status missing from request context")
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "rpc_failed"})
	})

	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/radar/check", nil))
	if recorder.Code != http.StatusServiceUnavailable || refunded != 1 {
		t.Fatalf("status=%d refunded=%d", recorder.Code, refunded)
	}
}

func TestSuccessfulProtectedWorkKeepsReservation(t *testing.T) {
	identity := func(*http.Request) (string, string, error) { return "subject", "enterprise", nil }
	reserve := func(context.Context, string, string, string) (premiumOutputReservation, koschQuotaStatus, error) {
		return premiumOutputReservation{EntitlementID: "id"}, koschQuotaStatus{Tier: "enterprise", DailyLimit: 1000, UsedToday: 1, Remaining: 999}, nil
	}
	refunded := 0
	refund := func(context.Context, premiumOutputReservation, string) error { refunded++; return nil }
	handler := enforceScanQuotaWith(identity, reserve, refund, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/scan/token", nil))
	if recorder.Code != http.StatusOK || refunded != 0 {
		t.Fatalf("status=%d refunded=%d", recorder.Code, refunded)
	}
}
