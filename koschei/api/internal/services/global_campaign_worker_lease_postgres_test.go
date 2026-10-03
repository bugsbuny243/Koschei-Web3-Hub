package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestGlobalCampaignWorkerLeasePostgres17(t *testing.T) {
	databaseURL := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(16)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	leaseKey := fmt.Sprintf("campaign-worker-%d", time.Now().UnixNano())
	contentionKey := leaseKey + "-contention"
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM global_campaign_worker_leases WHERE lease_key=$1`, leaseKey)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM global_campaign_worker_leases WHERE lease_key=$1`, contentionKey)
	})

	t0 := time.Date(2026, 10, 3, 1, 0, 0, 0, time.UTC)
	a, err := AcquireGlobalCampaignWorkerLease(ctx, db, leaseKey, "worker:a", t0, 2*time.Minute)
	if err != nil {
		t.Fatalf("worker A acquire: %v", err)
	}
	if a.FencingToken != 1 {
		t.Fatalf("first token=%d want 1", a.FencingToken)
	}

	if _, err := AcquireGlobalCampaignWorkerLease(ctx, db, leaseKey, "worker:b", t0.Add(30*time.Second), 2*time.Minute); !errors.Is(err, ErrGlobalCampaignWorkerLeaseBusy) {
		t.Fatalf("worker B before expiry error=%v want busy", err)
	}
	if err := AssertGlobalCampaignWorkerLease(ctx, db, a, t0.Add(time.Minute)); err != nil {
		t.Fatalf("worker A should still own lease: %v", err)
	}

	b, err := AcquireGlobalCampaignWorkerLease(ctx, db, leaseKey, "worker:b", t0.Add(3*time.Minute), 2*time.Minute)
	if err != nil {
		t.Fatalf("worker B takeover: %v", err)
	}
	if b.FencingToken != 2 {
		t.Fatalf("takeover token=%d want 2", b.FencingToken)
	}
	if err := AssertGlobalCampaignWorkerLease(ctx, db, a, t0.Add(3*time.Minute)); !errors.Is(err, ErrGlobalCampaignWorkerLeaseFenced) {
		t.Fatalf("stale worker A assert error=%v want fenced", err)
	}
	if _, err := RenewGlobalCampaignWorkerLease(ctx, db, a, t0.Add(3*time.Minute), time.Minute); !errors.Is(err, ErrGlobalCampaignWorkerLeaseFenced) {
		t.Fatalf("stale worker A renew error=%v want fenced", err)
	}
	if err := ReleaseGlobalCampaignWorkerLease(ctx, db, a, t0.Add(3*time.Minute)); !errors.Is(err, ErrGlobalCampaignWorkerLeaseFenced) {
		t.Fatalf("stale worker A release error=%v want fenced", err)
	}

	b, err = RenewGlobalCampaignWorkerLease(ctx, db, b, t0.Add(3*time.Minute+30*time.Second), 2*time.Minute)
	if err != nil {
		t.Fatalf("worker B renew: %v", err)
	}
	if err := AssertGlobalCampaignWorkerLease(ctx, db, b, t0.Add(4*time.Minute)); err != nil {
		t.Fatalf("worker B assert after renew: %v", err)
	}
	if err := ReleaseGlobalCampaignWorkerLease(ctx, db, b, t0.Add(4*time.Minute)); err != nil {
		t.Fatalf("worker B release: %v", err)
	}

	const contenders = 8
	var winners atomic.Int32
	var busy atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			lease, err := AcquireGlobalCampaignWorkerLease(ctx, db, contentionKey, fmt.Sprintf("worker:race:%d", i), t0, 5*time.Minute)
			if err == nil {
				if lease.FencingToken != 1 {
					t.Errorf("winner token=%d want 1", lease.FencingToken)
				}
				winners.Add(1)
				return
			}
			if errors.Is(err, ErrGlobalCampaignWorkerLeaseBusy) {
				busy.Add(1)
				return
			}
			t.Errorf("contender %d unexpected error: %v", i, err)
		}(i)
	}
	close(start)
	wg.Wait()
	if winners.Load() != 1 || busy.Load() != contenders-1 {
		t.Fatalf("contention winners=%d busy=%d want 1/%d", winners.Load(), busy.Load(), contenders-1)
	}
}
