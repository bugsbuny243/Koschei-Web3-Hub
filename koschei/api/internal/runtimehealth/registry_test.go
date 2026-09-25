package runtimehealth

import (
	"errors"
	"testing"
	"time"
)

func TestRegistryTracksConfiguredLiveDegradedAndStopped(t *testing.T) {
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	registry := New()
	registry.now = func() time.Time { return now }

	registry.Register("worker.head.ethereum", "head_ingest", "ethereum-mainnet", true)
	snapshot := registry.Snapshot()
	if len(snapshot.Entries) != 1 || snapshot.Entries[0].State != StateConfigured {
		t.Fatalf("unexpected configured snapshot: %#v", snapshot)
	}

	now = now.Add(time.Minute)
	registry.Success("worker.head.ethereum", 2)
	snapshot = registry.Snapshot()
	entry := snapshot.Entries[0]
	if entry.State != StateLive || entry.SuccessfulCycles != 1 || entry.Observations != 2 || entry.LastSuccessAt == nil {
		t.Fatalf("unexpected live entry: %#v", entry)
	}

	now = now.Add(time.Minute)
	registry.Failure("worker.head.ethereum", errors.New("rpc_status_429"))
	entry = registry.Snapshot().Entries[0]
	if entry.State != StateDegraded || entry.FailedCycles != 1 || entry.ConsecutiveFailures != 1 || entry.LastError != "rpc_status_429" {
		t.Fatalf("unexpected degraded entry: %#v", entry)
	}

	now = now.Add(time.Minute)
	registry.Stop("worker.head.ethereum")
	entry = registry.Snapshot().Entries[0]
	if entry.State != StateStopped {
		t.Fatalf("unexpected stopped entry: %#v", entry)
	}
}

func TestRegistryMarksFirstFailureUnavailable(t *testing.T) {
	registry := New()
	registry.Register("clickhouse.events", "storage", "", true)
	registry.Failure("clickhouse.events", errors.New("connection refused"))
	entry := registry.Snapshot().Entries[0]
	if entry.State != StateUnavailable {
		t.Fatalf("state=%q", entry.State)
	}
}

func TestRegistryKeepsDisabledComponentsExplicit(t *testing.T) {
	registry := New()
	registry.Register("worker.optional", "worker", "", false)
	entry := registry.Snapshot().Entries[0]
	if entry.State != StateDisabled || entry.Configured {
		t.Fatalf("unexpected disabled entry: %#v", entry)
	}
}
