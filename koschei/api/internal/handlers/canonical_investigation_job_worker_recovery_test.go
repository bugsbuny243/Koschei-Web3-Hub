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
