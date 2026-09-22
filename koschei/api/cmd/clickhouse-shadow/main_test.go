package main

import (
	"strings"
	"testing"
	"time"
)

func TestLoadConfigAtUsesExplicitDeterministicWindow(t *testing.T) {
	t.Setenv("DATABASE_READ_URL", "postgres://shadow-source.example/koschei")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("KOSCHEI_CLICKHOUSE_SHADOW_SINCE", "2026-09-21T01:00:00Z")
	t.Setenv("KOSCHEI_CLICKHOUSE_SHADOW_UNTIL", "2026-09-21T03:00:00Z")

	now := time.Date(2026, 9, 22, 4, 30, 0, 0, time.UTC)
	cfg, err := loadConfigAt(now)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.since.Format(time.RFC3339), "2026-09-21T01:00:00Z"; got != want {
		t.Fatalf("since=%s want=%s", got, want)
	}
	if got, want := cfg.until.Format(time.RFC3339), "2026-09-21T03:00:00Z"; got != want {
		t.Fatalf("until=%s want=%s", got, want)
	}
}

func TestLoadConfigAtDefaultsSinceRelativeToExplicitUntil(t *testing.T) {
	t.Setenv("DATABASE_READ_URL", "postgres://shadow-source.example/koschei")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("KOSCHEI_CLICKHOUSE_SHADOW_SINCE", "")
	t.Setenv("KOSCHEI_CLICKHOUSE_SHADOW_UNTIL", "2026-09-21T03:00:00Z")

	now := time.Date(2026, 9, 22, 4, 30, 0, 0, time.UTC)
	cfg, err := loadConfigAt(now)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.since.Format(time.RFC3339), "2026-09-20T03:00:00Z"; got != want {
		t.Fatalf("since=%s want=%s", got, want)
	}
	if got, want := cfg.until.Format(time.RFC3339), "2026-09-21T03:00:00Z"; got != want {
		t.Fatalf("until=%s want=%s", got, want)
	}
}

func TestLoadConfigAtRejectsFutureUntil(t *testing.T) {
	t.Setenv("DATABASE_READ_URL", "postgres://shadow-source.example/koschei")
	t.Setenv("KOSCHEI_CLICKHOUSE_SHADOW_UNTIL", "2026-09-22T05:00:00Z")

	now := time.Date(2026, 9, 22, 4, 30, 0, 0, time.UTC)
	_, err := loadConfigAt(now)
	if err == nil || !strings.Contains(err.Error(), "must not be in the future") {
		t.Fatalf("expected future-until error, got %v", err)
	}
}

func TestLoadConfigAtRejectsNonIncreasingWindow(t *testing.T) {
	t.Setenv("DATABASE_READ_URL", "postgres://shadow-source.example/koschei")
	t.Setenv("KOSCHEI_CLICKHOUSE_SHADOW_SINCE", "2026-09-21T03:00:00Z")
	t.Setenv("KOSCHEI_CLICKHOUSE_SHADOW_UNTIL", "2026-09-21T03:00:00Z")

	now := time.Date(2026, 9, 22, 4, 30, 0, 0, time.UTC)
	_, err := loadConfigAt(now)
	if err == nil || !strings.Contains(err.Error(), "must be before") {
		t.Fatalf("expected invalid-window error, got %v", err)
	}
}
