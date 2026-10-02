package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestGlobalCampaignStorePostgres17(t *testing.T) {
	databaseURL := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(4)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	campaignRef := fmt.Sprintf("KCAM1-CI-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM global_campaign_current WHERE campaign_ref=$1`, campaignRef)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM global_campaign_revisions WHERE campaign_ref=$1`, campaignRef)
	})

	observedAt := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	first := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:         campaignRef,
		Revision:            1,
		State:               GlobalCampaignEmerging,
		FirstObservedAt:     observedAt,
		LastObservedAt:      observedAt,
		Networks:            []string{"ethereum", "solana"},
		Subjects:            []string{"subject:alpha"},
		ObservationRefs:     []string{"observation:alpha"},
		ObservedAnchorCount: 1,
		RulesetVersion:      "campaign-ci.v1",
	})
	if err := ValidateGlobalCampaign(first); err != nil {
		t.Fatalf("fixture invalid: %v", err)
	}

	created, err := PersistGlobalCampaignRevision(ctx, db, first)
	if err != nil {
		t.Fatalf("persist revision 1: %v", err)
	}
	if !created.Inserted || created.Idempotent {
		t.Fatalf("unexpected initial result: %+v", created)
	}

	replay, err := PersistGlobalCampaignRevision(ctx, db, first)
	if err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	if replay.Inserted || !replay.Idempotent {
		t.Fatalf("unexpected replay result: %+v", replay)
	}

	divergent := first
	divergent.ObservationRefs = append(divergent.ObservationRefs, "observation:divergent")
	divergent.ObservedAnchorCount++
	divergent = NewGlobalCampaign(divergent)
	if _, err := PersistGlobalCampaignRevision(ctx, db, divergent); !errors.Is(err, ErrGlobalCampaignRevisionConflict) {
		t.Fatalf("same-revision divergent payload error=%v want conflict", err)
	}

	gap := first
	gap.Revision = 3
	gap.LastObservedAt = observedAt.Add(2 * time.Minute)
	gap.ObservationRefs = append(gap.ObservationRefs, "observation:gap")
	gap = NewGlobalCampaign(gap)
	if _, err := PersistGlobalCampaignRevision(ctx, db, gap); !errors.Is(err, ErrGlobalCampaignRevisionGap) {
		t.Fatalf("revision gap error=%v want gap", err)
	}

	next, err := ApplyGlobalCampaignTransition(first, GlobalCampaignTransition{
		From:         GlobalCampaignEmerging,
		To:           GlobalCampaignActive,
		ReasonCodes:  []string{"verified_relation_growth"},
		EvidenceRefs: []string{"relation:alpha-beta"},
	})
	if err != nil {
		t.Fatalf("build revision 2 transition: %v", err)
	}
	next.LastObservedAt = observedAt.Add(time.Minute)
	next.RelationRefs = append(next.RelationRefs, "relation:alpha-beta")
	next.VerifiedAnchorCount = 1
	next = NewGlobalCampaign(next)
	persisted, err := PersistGlobalCampaignRevision(ctx, db, next)
	if err != nil {
		t.Fatalf("persist revision 2: %v", err)
	}
	if !persisted.Inserted || persisted.Idempotent {
		t.Fatalf("unexpected revision 2 result: %+v", persisted)
	}

	if _, err := PersistGlobalCampaignRevision(ctx, db, first); !errors.Is(err, ErrGlobalCampaignRevisionStale) {
		t.Fatalf("stale revision error=%v want stale", err)
	}

	invalidTransition := next
	invalidTransition.Revision = 3
	invalidTransition.State = GlobalCampaignReopened
	invalidTransition.LastObservedAt = observedAt.Add(2 * time.Minute)
	invalidTransition.TransitionReasonCodes = []string{"invalid_direct_reopen"}
	invalidTransition.TransitionEvidenceRefs = []string{"observation:alpha"}
	invalidTransition = NewGlobalCampaign(invalidTransition)
	if _, err := PersistGlobalCampaignRevision(ctx, db, invalidTransition); err == nil {
		t.Fatal("invalid active -> reopened transition unexpectedly persisted")
	}

	loaded, found, err := LoadCurrentGlobalCampaign(ctx, db, campaignRef)
	if err != nil {
		t.Fatalf("load current campaign: %v", err)
	}
	if !found {
		t.Fatal("current campaign not found")
	}
	if loaded.Revision != 2 || loaded.State != GlobalCampaignActive || loaded.EvidenceHashSHA256 != next.EvidenceHashSHA256 {
		t.Fatalf("unexpected current campaign: %+v", loaded)
	}

	var revisionCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM global_campaign_revisions WHERE campaign_ref=$1`, campaignRef,
	).Scan(&revisionCount); err != nil {
		t.Fatal(err)
	}
	if revisionCount != 2 {
		t.Fatalf("revision count=%d want=2", revisionCount)
	}

	if _, err := db.ExecContext(ctx, `
		UPDATE global_campaign_current
		SET evidence_hash_sha256=$2
		WHERE campaign_ref=$1`,
		campaignRef,
		"sha256:0000000000000000000000000000000000000000000000000000000000000000",
	); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadCurrentGlobalCampaign(ctx, db, campaignRef); !errors.Is(err, ErrGlobalCampaignStoredCorrupt) {
		t.Fatalf("corrupt projection error=%v want fail-closed corruption", err)
	}
}
