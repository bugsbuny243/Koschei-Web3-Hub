package runtimehealth

import (
	"encoding/json"
	"errors"
	"strings"
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

func TestPeriodicHealthExpiresWithoutNewFailuresAndRecovers(t *testing.T) {
	now := time.Date(2026, 9, 29, 17, 0, 0, 0, time.UTC)
	r := New()
	r.now = func() time.Time { return now }
	r.RegisterPeriodic("head", "worker", "", true, time.Minute)
	r.Success("head", 0) // A successful empty cycle is still a heartbeat.
	now = now.Add(time.Minute)
	r.Register("head", "worker", "", true) // Registration is not a heartbeat.
	s := r.Snapshot()
	e := s.Entries[0]
	if e.State != StateDegraded || e.Freshness != "stale" || s.Counts[StateLive] != 0 || e.FailedCycles != 0 {
		t.Fatalf("stalled worker must not remain live or invent failures: %#v", s)
	}
	if r.entries["head"].State != StateLive {
		t.Fatal("snapshot must not mutate recorded state")
	}
	r.Success("head", 3)
	e = r.Snapshot().Entries[0]
	if e.State != StateLive || e.Freshness != "fresh" || e.Observations != 3 {
		t.Fatalf("successful cycle must recover freshness: %#v", e)
	}
}

func TestPeriodicHealthStartupFailureAndInactiveStates(t *testing.T) {
	now := time.Date(2026, 9, 29, 17, 0, 0, 0, time.UTC)
	r := New()
	r.now = func() time.Time { return now }
	r.RegisterPeriodic("head", "worker", "", true, time.Minute)
	if e := r.Snapshot().Entries[0]; e.Freshness != "awaiting_success" {
		t.Fatalf("startup is not live evidence: %#v", e)
	}
	now = now.Add(time.Minute)
	r.RegisterPeriodic("head", "worker", "", true, time.Minute)
	if e := r.Snapshot().Entries[0]; e.Freshness != "stale" || e.State != StateDegraded {
		t.Fatalf("repeated registration must not hide startup stall: %#v", e)
	}
	r.Failure("head", errors.New("provider unavailable"))
	if e := r.Snapshot().Entries[0]; e.State != StateUnavailable || e.Freshness != "stale" {
		t.Fatalf("expiry must preserve unavailable status: %#v", e)
	}
	r.Stop("head")
	if e := r.Snapshot().Entries[0]; e.State != StateStopped || e.Freshness != "inactive" {
		t.Fatalf("stopped worker: %#v", e)
	}
	r.RegisterPeriodic("head", "worker", "", false, time.Minute)
	if e := r.Snapshot().Entries[0]; e.State != StateDisabled || e.Freshness != "inactive" {
		t.Fatalf("disabled worker: %#v", e)
	}
	r.RegisterPeriodic("head", "worker", "", true, time.Minute)
	if e := r.Snapshot().Entries[0]; e.State != StateConfigured || e.Freshness != "awaiting_success" {
		t.Fatalf("restart must get a new startup deadline: %#v", e)
	}
}

func TestUnmonitoredHealthAndSnapshotTimestampIsolation(t *testing.T) {
	r := New()
	r.Register("database", "storage", "", true)
	r.Success("database", 0)
	s := r.Snapshot()
	if s.Entries[0].Freshness != "not_monitored" || s.Entries[0].FreshUntil != nil {
		t.Fatal("one-time check must not claim continuously monitored freshness")
	}
	original := *s.Entries[0].LastSuccessAt
	*s.Entries[0].LastSuccessAt = time.Time{}
	if !r.Snapshot().Entries[0].LastSuccessAt.Equal(original) {
		t.Fatal("snapshot caller changed registry timestamp")
	}
}

func TestIngestProgressPreservesUnknownZeroAndProviderRegression(t *testing.T) {
	r := New()
	r.Register("head", "head_ingest", "ethereum-mainnet", true)
	head, cursor := uint64(105), uint64(100)
	r.RecordIngestProgress("head", &head, &cursor, false, false)
	p := r.Snapshot().Entries[0].Ingest
	if p.Status != "catching_up" || *p.PendingBlocks != 5 {
		t.Fatalf("backlog: %#v", p)
	}
	*p.DurableCursor = 999
	head = 100
	if *r.Snapshot().Entries[0].Ingest.DurableCursor != 100 || *r.Snapshot().Entries[0].Ingest.ObservedHead != 105 {
		t.Fatal("progress aliases caller memory")
	}
	r.RecordIngestProgress("head", &head, &cursor, false, false)
	raw, _ := json.Marshal(r.Snapshot())
	if !strings.Contains(string(raw), `"pending_blocks":"0"`) {
		t.Fatalf("zero must remain present and exact: %s", raw)
	}
	head = 99
	r.RecordIngestProgress("head", &head, &cursor, true, false)
	p = r.Snapshot().Entries[0].Ingest
	if p.Status != "provider_behind_cursor" || p.PendingBlocks != nil {
		t.Fatalf("provider regression underflow: %#v", p)
	}
	r.RecordIngestProgress("head", nil, nil, true, true)
	p = r.Snapshot().Entries[0].Ingest
	if p.Status != "reorg_recheck_required" || p.PendingBlocks != nil || p.DurableCursor != nil {
		t.Fatalf("reorg must remain unverified: %#v", p)
	}
}
