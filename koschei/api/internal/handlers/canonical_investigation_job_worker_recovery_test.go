package handlers

import (
	"testing"
	"time"
)

func TestCanonicalStaleRecoveryInterval(t *testing.T) {
	t.Setenv("KOSCHEI_WORKER_RECOVERY_CEILING_SECONDS", "900")
	for _, tc := range []struct {
		name       string
		staleAfter time.Duration
		want       time.Duration
	}{
		{"default long lease", 2 * time.Hour, 15 * time.Minute},
		{"short lease", 5 * time.Minute, 150 * time.Second},
		{"unspecified lease", 0, 15 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalStaleRecoveryInterval(tc.staleAfter); got != tc.want {
				t.Fatalf("recovery interval=%s want=%s", got, tc.want)
			}
		})
	}
	t.Setenv("KOSCHEI_WORKER_RECOVERY_CEILING_SECONDS", "60")
	if got := canonicalStaleRecoveryInterval(5 * time.Minute); got != time.Minute {
		t.Fatalf("minimum recovery ceiling ignored: got %s", got)
	}
}

func TestCanonicalSafeHeartbeatStaysBelowStaleJobLease(t *testing.T) {
	for _, tc := range []struct {
		name      string
		stale     time.Duration
		requested time.Duration
		want      time.Duration
	}{
		{"default worker", 2 * time.Hour, 30 * time.Second, 30 * time.Second},
		{"heartbeat equals stale lease", 5 * time.Minute, 5 * time.Minute, 100 * time.Second},
		{"heartbeat nearly as long as stale lease", 5 * time.Minute, 290 * time.Second, 100 * time.Second},
		{"already safe heartbeat", 5 * time.Minute, 30 * time.Second, 30 * time.Second},
		{"zero heartbeat uses safe fallback", 5 * time.Minute, 0, 30 * time.Second},
		{"missing lease preserves heartbeat", 0, 45 * time.Second, 45 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := canonicalSafeHeartbeat(tc.stale, tc.requested)
			if got != tc.want {
				t.Fatalf("heartbeat=%s want %s", got, tc.want)
			}
			if tc.stale > 0 && got >= tc.stale/2 {
				t.Fatalf("heartbeat=%s is too close to recovery lease=%s", got, tc.stale)
			}
		})
	}
}
