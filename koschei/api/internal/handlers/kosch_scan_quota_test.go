package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func openQuotaTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("KOSCHEI_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = "postgres://postgres:postgres@127.0.0.1:5432/koschei_ci?sslmode=disable"
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("postgres unavailable: %v", err)
	}
	return db
}

func cleanupQuotaSubject(t *testing.T, db *sql.DB, subject string) {
	t.Helper()
	_, _ = db.Exec(`DELETE FROM kosch_daily_quota_reservations WHERE auth_subject=$1`, subject)
	_, _ = db.Exec(`DELETE FROM kosch_daily_quota_usage WHERE auth_subject=$1`, subject)
}

func TestKOSCHQuotaExhaustionAndFailedWorkRefund(t *testing.T) {
	db := openQuotaTestDB(t)
	defer db.Close()
	t.Setenv("KOSCHEI_QUOTA_BASIC_DAILY", "1")
	subject := "quota-test-refund"
	cleanupQuotaSubject(t, db, subject)
	defer cleanupQuotaSubject(t, db, subject)

	h := &Handler{DB: db}
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	reservation, first, err := h.reserveKOSCHScanQuota(context.Background(), subject, "basic", "test", now)
	if err != nil || first.Used != 1 || first.Remaining != 0 {
		t.Fatalf("first reservation: status=%#v err=%v", first, err)
	}
	_, exhausted, err := h.reserveKOSCHScanQuota(context.Background(), subject, "basic", "test", now)
	var accessErr tokenAccessError
	if !errors.As(err, &accessErr) || accessErr.Status != http.StatusTooManyRequests || accessErr.Code != "quota_exceeded" {
		t.Fatalf("expected quota_exceeded, status=%#v err=%v", exhausted, err)
	}
	if err := h.refundKOSCHScanQuota(context.Background(), reservation, "failed_work_refund"); err != nil {
		t.Fatal(err)
	}
	second, restored, err := h.reserveKOSCHScanQuota(context.Background(), subject, "basic", "retry", now)
	if err != nil || restored.Used != 1 {
		t.Fatalf("quota was not restored: status=%#v err=%v", restored, err)
	}
	if err := h.finalizeKOSCHScanQuota(context.Background(), second); err != nil {
		t.Fatal(err)
	}
}

func TestKOSCHQuotaResetsAtNextUTCDate(t *testing.T) {
	db := openQuotaTestDB(t)
	defer db.Close()
	t.Setenv("KOSCHEI_QUOTA_BASIC_DAILY", "1")
	subject := "quota-test-utc-reset"
	cleanupQuotaSubject(t, db, subject)
	defer cleanupQuotaSubject(t, db, subject)

	h := &Handler{DB: db}
	dayOne := time.Date(2026, 7, 15, 23, 59, 0, 0, time.UTC)
	dayTwo := dayOne.Add(2 * time.Minute)
	first, _, err := h.reserveKOSCHScanQuota(context.Background(), subject, "basic", "day-one", dayOne)
	if err != nil {
		t.Fatal(err)
	}
	defer h.finalizeKOSCHScanQuota(context.Background(), first)
	second, status, err := h.reserveKOSCHScanQuota(context.Background(), subject, "basic", "day-two", dayTwo)
	if err != nil {
		t.Fatalf("new UTC day must restore quota: %v", err)
	}
	defer h.finalizeKOSCHScanQuota(context.Background(), second)
	if status.Used != 1 || !status.ResetsAt.Equal(time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected day-two status: %#v", status)
	}
}

func TestKOSCHQuotaDefaultsFollowTierLadder(t *testing.T) {
	t.Setenv("KOSCHEI_QUOTA_BASIC_DAILY", "")
	t.Setenv("KOSCHEI_QUOTA_PRO_DAILY", "")
	t.Setenv("KOSCHEI_QUOTA_ENTERPRISE_DAILY", "")
	if got := configuredKOSCHDailyQuota("basic"); got != 5 {
		t.Fatalf("basic quota=%d", got)
	}
	if got := configuredKOSCHDailyQuota("pro"); got != 100 {
		t.Fatalf("pro quota=%d", got)
	}
	if got := configuredKOSCHDailyQuota("enterprise"); got != 1000 {
		t.Fatalf("enterprise quota=%d", got)
	}
	if tokenTierRank("basic") >= tokenTierRank("pro") {
		t.Fatal("basic/dust holder must not satisfy a pro route")
	}
}
