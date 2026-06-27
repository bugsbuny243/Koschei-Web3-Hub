package webhooks

import (
	"bytes"
	"testing"
	"time"
)

func TestNextRetryAt(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		attempt int
		delay   time.Duration
	}{
		{1, time.Minute},
		{2, 5 * time.Minute},
		{3, 30 * time.Minute},
		{4, 2 * time.Hour},
		{5, 8 * time.Hour},
		{6, 24 * time.Hour},
		{20, 24 * time.Hour},
	}
	for _, item := range cases {
		got := nextRetryAt(now, item.attempt)
		if !got.Equal(now.Add(item.delay)) {
			t.Fatalf("attempt %d: got %s want %s", item.attempt, got, now.Add(item.delay))
		}
	}
}

func TestCompactJSON(t *testing.T) {
	raw := []byte("{\n  \"b\": 2,\n  \"a\": 1\n}")
	got := compactJSON(raw)
	if bytes.Contains(got, []byte("\n")) {
		t.Fatalf("payload was not compacted: %q", got)
	}
	if !bytes.Contains(got, []byte(`"a":1`)) || !bytes.Contains(got, []byte(`"b":2`)) {
		t.Fatalf("unexpected compact payload: %q", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abcdef", 3); got != "abc" {
		t.Fatalf("truncate = %q", got)
	}
	if got := truncate("abc", 3); got != "abc" {
		t.Fatalf("truncate exact = %q", got)
	}
}
