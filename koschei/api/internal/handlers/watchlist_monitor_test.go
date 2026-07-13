package handlers

import (
	"testing"
	"time"
)

func TestWatchlistMonitorConfigDefaults(t *testing.T) {
	t.Setenv("WATCHLIST_MONITOR_ENABLED", "")
	t.Setenv("WATCHLIST_MONITOR_INTERVAL", "")
	t.Setenv("WATCHLIST_MONITOR_BATCH_SIZE", "")
	if !watchlistMonitorEnabled() {
		t.Fatal("watchlist monitor should default to enabled")
	}
	if got := watchlistMonitorInterval(); got != time.Minute {
		t.Fatalf("interval = %s, want %s", got, time.Minute)
	}
	if got := watchlistMonitorBatchSize(); got != 20 {
		t.Fatalf("batch = %d, want 20", got)
	}
}

func TestWatchlistMonitorConfigBounds(t *testing.T) {
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
