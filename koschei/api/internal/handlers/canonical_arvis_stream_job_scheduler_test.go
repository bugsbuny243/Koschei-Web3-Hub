package handlers

import (
	"context"
	"database/sql"
	"testing"

	"koschei/api/internal/runtimehealth"
)

func findRuntimeHealthEntry(t *testing.T, health *runtimehealth.Registry, id string) runtimehealth.Entry {
	t.Helper()
	for _, entry := range health.Snapshot().Entries {
		if entry.ID == id {
			return entry
		}
	}
	t.Fatalf("runtime health entry %q not registered", id)
	return runtimehealth.Entry{}
}

func TestCanonicalArvisStreamSchedulerWithheldUntilReady(t *testing.T) {
	t.Setenv("KOSCHEI_CANONICAL_JOB_WORKER_ENABLED", "true")
	t.Setenv("ARVIS_STREAM_CANONICAL_JOBS_ENABLED", "")
	// A non-nil handle that is never opened: a withheld scheduler must not
	// touch the database at all.
	db := &sql.DB{}
	health := runtimehealth.New()
	stop := StartCanonicalArvisStreamJobScheduler(context.Background(), db, nil, health)
	if stop == nil {
		t.Fatal("withheld scheduler must still return a stop function")
	}
	stop()
	entry := findRuntimeHealthEntry(t, health, CanonicalArvisStreamHealthID)
	if entry.Configured {
		t.Fatalf("withheld scheduler must be visible as unconfigured: %#v", entry)
	}

	scheduler := &canonicalArvisStreamJobScheduler{DB: db, MaxJobsPerCycle: 1}
	released, queued, err := scheduler.RunOnce(context.Background())
	if err != nil || released != 0 || queued != 0 {
		t.Fatalf("not-ready cycle must be a no-op: released=%d queued=%d err=%v", released, queued, err)
	}
}

func TestCanonicalArvisStreamSchedulerRequiresCanonicalWorker(t *testing.T) {
	t.Setenv("KOSCHEI_CANONICAL_JOB_WORKER_ENABLED", "false")
	t.Setenv("ARVIS_STREAM_CANONICAL_JOBS_ENABLED", "true")
	stop := StartCanonicalArvisStreamJobScheduler(context.Background(), &sql.DB{}, nil, nil)
	stop()
}
