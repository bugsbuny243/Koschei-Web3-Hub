package http

import (
	"context"
	"testing"
	"time"
)

func TestStatelessSensitiveLimitUsesBoundedMemoryWithoutLegacyAuthFlag(t *testing.T) {
	t.Setenv("KOSCHEI_NEON_AUTH_ONLY", "")
	key := sensitiveBucketKeyHash("203.0.113.77", "/api/auth/login")
	rule := sensitiveLimitRule{Limit: 2, Window: time.Minute}

	first, err := consumeSharedSensitiveLimit(context.Background(), nil, key, "/api/auth/login", rule)
	if err != nil {
		t.Fatalf("first stateless rate-limit consume failed: %v", err)
	}
	if !first.Allowed || first.Count != 1 || first.Remaining != 1 {
		t.Fatalf("unexpected first decision: %+v", first)
	}

	second, err := consumeSharedSensitiveLimit(context.Background(), nil, key, "/api/auth/login", rule)
	if err != nil {
		t.Fatalf("second stateless rate-limit consume failed: %v", err)
	}
	if !second.Allowed || second.Count != 2 || second.Remaining != 0 {
		t.Fatalf("unexpected second decision: %+v", second)
	}

	third, err := consumeSharedSensitiveLimit(context.Background(), nil, key, "/api/auth/login", rule)
	if err != nil {
		t.Fatalf("third stateless rate-limit consume failed: %v", err)
	}
	if third.Allowed || third.Count != 3 {
		t.Fatalf("stateless memory limiter did not enforce limit: %+v", third)
	}
}

func TestStatelessSensitiveLimitStorageNameIsExplicit(t *testing.T) {
	t.Setenv("KOSCHEI_NEON_AUTH_ONLY", "")
	if got := sensitiveRateLimitStorageName(nil); got != "bounded_memory" {
		t.Fatalf("storage name = %q; want bounded_memory", got)
	}
}
