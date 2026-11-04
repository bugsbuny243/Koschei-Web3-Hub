package main

import (
	"bytes"
	"log"
	"testing"
)

// Lines copied from production deploy logs on 2026-11-03.
func TestLogLevelRoutingSeparatesFailuresFromProgress(t *testing.T) {
	failures := []string{
		`solana rpc failure method=getSignaturesForAddress provider=mainnet.helius-rpc.com http_class=4xx status=429 error="http status 429"`,
		`radar retention v2: halted; before_retention_v2 archive export failed: valid KOSCHEI_RADAR_ARCHIVE_EXPORT_S3_ENDPOINT is required`,
		`CRITICAL: missing required production security env vars: OWNER_SECRET`,
		`tron flow radar cycle failed: tron_provider_http_429`,
		`watchlist monitor refresh failed id=42: tron_provider_unavailable`,
	}
	progress := []string{
		`canonical pump volume cycle: candidates=300 qualified=6 queued=1 threshold_usd=500000`,
		`actor defense correlation cycle: changed_creator_tracks=14 changed_repeat_holder_tracks=15 memory=persistent_actor_index`,
		`api listening on :8080`,
		`watchlist monitor started: interval=1m0s batch=20`,
		`migrations applied/skipped: 0/141`,
		`ENTITLEMENT_DATABASE_URL is not set: paid customer operations remain fail-closed`,
		`tron flow radar started: mode=observe_only storage=in_memory seeds=0 max_hops=4 max_tracked=20000 poll=3s`,
		`webhook delivery summary error_count=0 errors=0`,
	}
	var out, errOut bytes.Buffer
	logger := log.New(levelRoutingWriter{out: &out, errOut: &errOut}, "", 0)
	for _, line := range failures {
		out.Reset()
		errOut.Reset()
		logger.Print(line)
		if errOut.Len() == 0 || out.Len() != 0 {
			t.Fatalf("failure line went to stdout: %q", line)
		}
	}
	for _, line := range progress {
		out.Reset()
		errOut.Reset()
		logger.Print(line)
		if out.Len() == 0 || errOut.Len() != 0 {
			t.Fatalf("progress line went to stderr: %q", line)
		}
	}
}

func TestLogZeroFailureCounters(t *testing.T) {
	for _, tc := range []struct {
		line string
		failure bool
	}{
		{"worker summary processed=25 failed=0 denied=0 unavailable=0", false},
		{"worker summary error=0 failure=0 refused=0 halted=0", false},
		{"worker summary processed=25 failed=2", true},
		{"worker summary failed=0 connection refused", true},
	} {
		if got := logLineIsFailure([]byte(tc.line)); got != tc.failure {
			t.Errorf("logLineIsFailure(%q)=%v, want %v", tc.line, got, tc.failure)
		}
	}
}
