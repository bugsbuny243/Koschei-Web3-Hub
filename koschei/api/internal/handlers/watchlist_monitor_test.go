package handlers

import (
	"testing"
	"time"
)

func TestWatchlistMonitorDefaultsToDisabled(t *testing.T) {
	t.Setenv("KOSCHEI_AUTOMATIC_SCANNING_ENABLED", "")
	t.Setenv("WATCHLIST_MONITOR_ENABLED", "")
	t.Setenv("WATCHLIST_MONITOR_INTERVAL", "")
	t.Setenv("WATCHLIST_MONITOR_BATCH_SIZE", "")
	if watchlistMonitorEnabled() {
		t.Fatal("watchlist monitor must default to disabled")
	}
	if got := watchlistMonitorInterval(); got != time.Minute {
		t.Fatalf("interval = %s, want %s", got, time.Minute)
	}
	if got := watchlistMonitorBatchSize(); got != 20 {
		t.Fatalf("batch = %d, want 20", got)
	}
}

func TestWatchlistMonitorRequiresBothExplicitSwitches(t *testing.T) {
	t.Setenv("KOSCHEI_AUTOMATIC_SCANNING_ENABLED", "true")
	t.Setenv("WATCHLIST_MONITOR_ENABLED", "true")
	if !watchlistMonitorEnabled() {
		t.Fatal("watchlist monitor should start only when both explicit switches are enabled")
	}
	t.Setenv("KOSCHEI_AUTOMATIC_SCANNING_ENABLED", "false")
	if watchlistMonitorEnabled() {
		t.Fatal("master automatic-scanning switch must override the watchlist switch")
	}
}

func TestWatchlistMonitorConfigBounds(t *testing.T) {
	t.Setenv("KOSCHEI_AUTOMATIC_SCANNING_ENABLED", "true")
	t.Setenv("WATCHLIST_MONITOR_ENABLED", "false")
	t.Setenv("WATCHLIST_MONITOR_INTERVAL", "5s")
	t.Setenv("WATCHLIST_MONITOR_BATCH_SIZE", "999")
	if watchlistMonitorEnabled() {
		t.Fatal("watchlist monitor should be disabled")
	}
	if got := watchlistMonitorInterval(); got != time.Minute {
		t.Fatalf("unsafe interval = %s, want default %s", got, time.Minute)
	}
	if got := watchlistMonitorBatchSize(); got != 20 {
		t.Fatalf("unsafe batch = %d, want default 20", got)
	}
}
