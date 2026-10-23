package jobs

import (
	"strings"
	"testing"
	"time"
)

func TestStaleRequeueScopesToCanonicalJobTypes(t *testing.T) {
	query, args, err := staleRequeueQuery(2*time.Hour, "canonical_investigation", "token_scan", "token_scan", " ")
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"WHERE status='running'",
		"updated_at < now() - ($1 * interval '1 second')",
		"AND job_type IN ($2,$3)",
		"CASE WHEN attempts < max_attempts",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("stale recovery query missing %q", fragment)
		}
	}
	if len(args) != 3 || args[0] != int64(7200) || args[1] != "canonical_investigation" || args[2] != "token_scan" {
		t.Fatalf("unexpected scoped stale requeue args: %#v", args)
	}
}

func TestStaleRequeueRejectsUnscopedRecovery(t *testing.T) {
	for _, input := range [][]string{nil, {"", "  "}} {
		query, args, err := staleRequeueQuery(30*time.Minute, input...)
		if err == nil || query != "" || args != nil {
			t.Fatalf("unscoped recovery must be rejected: query=%q args=%v err=%v", query, args, err)
		}
	}
}
