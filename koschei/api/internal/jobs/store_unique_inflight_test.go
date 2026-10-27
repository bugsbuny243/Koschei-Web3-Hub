package jobs

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// A customer pressing "investigate" twice must join the running job, but a
// finished investigation must never block a fresh one.
func TestCreateUniqueInFlightJoinsOnlyQueuedOrRunningJobsPostgres17(t *testing.T) {
	url := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL not configured")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	run := time.Now().UTC().Format("20060102150405.000000000")
	target := "ci-fixture-inflight-target-" + run
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM web3_jobs WHERE target=$1`, target) })

	store := &Store{DB: db}
	input := func(user string) CreateInput {
		return CreateInput{UserID: user, Type: "canonical_investigation", Network: "solana-mainnet", Target: target,
			Request: map[string]any{"mint": target, "mode": "customer_canonical_job"}}
	}
	first, created, err := store.CreateUniqueInFlight(ctx, input("ci-fixture-user-a"), "customer|ci-fixture-user-a")
	if err != nil || !created {
		t.Fatalf("first request: created=%v err=%v", created, err)
	}
	again, created, err := store.CreateUniqueInFlight(ctx, input("ci-fixture-user-a"), "customer|ci-fixture-user-a")
	if err != nil || created || again.ID != first.ID {
		t.Fatalf("a repeated request must join queued job %s: got %s created=%v err=%v", first.ID, again.ID, created, err)
	}
	other, created, err := store.CreateUniqueInFlight(ctx, input("ci-fixture-user-b"), "customer|ci-fixture-user-b")
	if err != nil || !created || other.ID == first.ID {
		t.Fatalf("another account must get its own job: got %s created=%v err=%v", other.ID, created, err)
	}

	if err := store.Complete(ctx, first.ID, map[string]any{"ok": true}); err != nil {
		t.Fatal(err)
	}
	fresh, created, err := store.CreateUniqueInFlight(ctx, input("ci-fixture-user-a"), "customer|ci-fixture-user-a")
	if err != nil || !created || fresh.ID == first.ID {
		t.Fatalf("a completed job must not block a new investigation: got %s created=%v err=%v", fresh.ID, created, err)
	}
	if reused, created, err := store.CreateUniqueActive(ctx, input("ci-fixture-user-a"), "customer|ci-fixture-user-a"); err != nil || created || reused.ID != fresh.ID {
		t.Fatalf("CreateUniqueActive must keep matching existing jobs: got %s created=%v err=%v", reused.ID, created, err)
	}
}
