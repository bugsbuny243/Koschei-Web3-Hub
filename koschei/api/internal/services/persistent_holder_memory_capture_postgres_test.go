package services

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestPersistentHolderMemorySurvivesCurrentShareDecayPostgres17(t *testing.T) {
	databaseURL := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	suffix := time.Now().UTC().Format("20060102150405.000000000")
	owner := "ci-holder-memory-owner-" + suffix
	mintA := "ci-holder-memory-a-" + suffix
	mintB := "ci-holder-memory-b-" + suffix
	mintCurrent := "ci-holder-memory-current-" + suffix
	cleanup := func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM security_actor_evidence WHERE network='solana-mainnet' AND actor_wallet=$1`, owner)
	}
	cleanup()
	defer cleanup()

	observedA := time.Date(2026, 7, 13, 18, 38, 12, 0, time.UTC)
	observedB := time.Date(2026, 7, 16, 19, 12, 44, 0, time.UTC)
	for _, input := range []struct {
		mint  string
		share float64
		when  time.Time
	}{
		{mintA, 81.85, observedA},
		{mintB, 36.03, observedB},
	} {
		holder := HolderIntelligence{
			Available: true,
			Rows: []HolderIntelligenceRow{{
				Rank: 1, OwnerWallet: owner, OwnerResolved: true, RiskBearing: true,
				CirculatingPercentage: input.share,
			}},
		}
		count, err := CapturePersistentDominantHolderMemory(ctx, db, "solana-mainnet", input.mint, holder, input.when)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("persisted=%d for %s", count, input.mint)
		}
	}

	currentHolder := HolderIntelligence{
		Available: true,
		Rows: []HolderIntelligenceRow{{
			Rank: 2, OwnerWallet: owner, OwnerResolved: true, RiskBearing: true,
			CirculatingPercentage: 9.25,
		}},
	}
	store := NewSecurityRadarStore(db)
	repeat, err := store.PersistentRepeatDominantHolders(ctx, currentHolder, mintCurrent, "solana-mainnet")
	if err != nil {
		t.Fatal(err)
	}
	if len(repeat) != 1 {
		t.Fatalf("repeat evidence=%#v", repeat)
	}
	if repeat[0].TokenCount != 2 {
		t.Fatalf("historical token count=%d, want 2", repeat[0].TokenCount)
	}
	if repeat[0].CurrentPercentage != 9.25 {
		t.Fatalf("current percentage=%.4f", repeat[0].CurrentPercentage)
	}
}

func TestCapturePersistentDominantHolderMemoryRejectsNonDominantRowsPostgres17(t *testing.T) {
	databaseURL := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	suffix := time.Now().UTC().Format("20060102150405.000000000")
	owner := "ci-holder-memory-reject-" + suffix
	mint := "ci-holder-memory-reject-mint-" + suffix
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM security_actor_evidence WHERE network='solana-mainnet' AND actor_wallet=$1`, owner)
	}()

	holder := HolderIntelligence{
		Available: true,
		Rows: []HolderIntelligenceRow{
			{Rank: 1, OwnerWallet: owner, OwnerResolved: true, RiskBearing: true, CirculatingPercentage: 19.99},
			{Rank: 2, OwnerWallet: owner + "-infra", OwnerResolved: true, RiskBearing: true, ExcludedFromHolderRisk: true, CirculatingPercentage: 88},
			{Rank: 6, OwnerWallet: owner + "-rank6", OwnerResolved: true, RiskBearing: true, CirculatingPercentage: 55},
		},
	}
	count, err := CapturePersistentDominantHolderMemory(ctx, db, "solana-mainnet", mint, holder, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("persisted non-dominant rows=%d", count)
	}
}
