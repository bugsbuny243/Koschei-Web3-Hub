package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestGlobalCampaignLookupMergePostgres17(t *testing.T) {
	databaseURL := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(8)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	prefix := fmt.Sprintf("campaign-lookup-ci-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		rows, queryErr := db.QueryContext(cleanupCtx, `
			SELECT DISTINCT campaign_ref
			FROM global_campaign_revisions
			WHERE canonical_payload::text LIKE $1`, "%"+prefix+"%")
		if queryErr != nil {
			return
		}
		var refs []string
		for rows.Next() {
			var ref string
			if scanErr := rows.Scan(&ref); scanErr == nil {
				refs = append(refs, ref)
			}
		}
		_ = rows.Close()
		for _, ref := range refs {
			_, _ = db.ExecContext(cleanupCtx, `DELETE FROM global_campaign_current WHERE campaign_ref=$1`, ref)
			_, _ = db.ExecContext(cleanupCtx, `DELETE FROM global_campaign_revisions WHERE campaign_ref=$1`, ref)
		}
	})

	t0 := time.Date(2026, 10, 2, 2, 0, 0, 0, time.UTC)
	sharedObservation := prefix + ":observation:shared"
	relationA := prefix + ":relation:a"
	relationB := prefix + ":relation:b"

	firstInput := GlobalCampaignMaterializerInput{
		ObservedAt:          t0,
		Networks:            []string{"ethereum-mainnet", "solana-mainnet"},
		Subjects:            []string{prefix + ":subject:alpha"},
		ObservationRefs:     []string{sharedObservation},
		RelationRefs:        []string{relationA},
		VerifiedAnchorCount: 1,
		RulesetVersion:      "campaign-ci.v1",
	}
	first, err := MaterializeAndPersistGlobalCampaign(ctx, db, firstInput)
	if err != nil {
		t.Fatalf("persist initial partial window: %v", err)
	}
	if !first.Inserted || first.Idempotent || first.Campaign.Revision != 1 || first.Campaign.CampaignRef == "" {
		t.Fatalf("unexpected initial result: %+v", first)
	}
	campaignRef := first.Campaign.CampaignRef

	secondInput := GlobalCampaignMaterializerInput{
		ObservedAt:          t0.Add(time.Minute),
		Networks:            []string{"ethereum-mainnet"},
		ObservationRefs:     []string{sharedObservation, prefix + ":observation:b"},
		RelationRefs:        []string{relationB},
		VerifiedAnchorCount: 2,
		RulesetVersion:      "campaign-ci.v1",
	}
	second, err := MaterializeAndPersistGlobalCampaign(ctx, db, secondInput)
	if err != nil {
		t.Fatalf("merge second partial window: %v", err)
	}
	if !second.Inserted || second.Campaign.CampaignRef != campaignRef || second.Campaign.Revision != 2 {
		t.Fatalf("second window did not converge onto campaign: %+v", second)
	}
	if !containsGlobalCampaignString(second.Campaign.RelationRefs, relationA) || !containsGlobalCampaignString(second.Campaign.RelationRefs, relationB) {
		t.Fatalf("second revision lost relation evidence: %#v", second.Campaign.RelationRefs)
	}

	thirdInput := GlobalCampaignMaterializerInput{
		ObservedAt:      t0.Add(2 * time.Minute),
		ObservationRefs: []string{prefix + ":observation:c"},
		RelationRefs:    []string{relationB},
		RulesetVersion:  "campaign-ci.v1",
	}
	third, err := MaterializeAndPersistGlobalCampaign(ctx, db, thirdInput)
	if err != nil {
		t.Fatalf("merge third partial window through newly indexed relation: %v", err)
	}
	if third.Campaign.CampaignRef != campaignRef || third.Campaign.Revision != 3 {
		t.Fatalf("third window did not preserve identity/revision: %+v", third)
	}

	replay, err := MaterializeAndPersistGlobalCampaign(ctx, db, thirdInput)
	if err != nil {
		t.Fatalf("replay third partial window: %v", err)
	}
	if !replay.Idempotent || replay.Inserted || replay.Campaign.Revision != 3 {
		t.Fatalf("exact partial-window replay was not idempotent: %+v", replay)
	}

	candidates, err := FindGlobalCampaignCandidates(ctx, db, GlobalCampaignMaterializerInput{
		RelationRefs: []string{relationB},
	})
	if err != nil {
		t.Fatalf("lookup by current anchor: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Campaign.CampaignRef != campaignRef || candidates[0].SharedAnchorCount < 1 {
		t.Fatalf("unexpected candidate lookup: %+v", candidates)
	}

	separateRelation := prefix + ":relation:separate"
	separate, err := MaterializeAndPersistGlobalCampaign(ctx, db, GlobalCampaignMaterializerInput{
		ObservedAt:      t0.Add(3 * time.Minute),
		ObservationRefs: []string{prefix + ":observation:separate"},
		RelationRefs:    []string{separateRelation},
		RulesetVersion:  "campaign-ci.v1",
	})
	if err != nil {
		t.Fatalf("create separate campaign: %v", err)
	}
	if separate.Campaign.CampaignRef == campaignRef {
		t.Fatal("separate evidence unexpectedly collapsed into existing campaign")
	}

	beforeAmbiguousRevision := replay.Campaign.Revision
	if _, err := MaterializeAndPersistGlobalCampaign(ctx, db, GlobalCampaignMaterializerInput{
		ObservedAt:     t0.Add(4 * time.Minute),
		RelationRefs:   []string{relationB, separateRelation},
		RulesetVersion: "campaign-ci.v1",
	}); !errors.Is(err, ErrGlobalCampaignAmbiguousCandidates) {
		t.Fatalf("ambiguous candidate error=%v want=%v", err, ErrGlobalCampaignAmbiguousCandidates)
	}
	currentAfterAmbiguous, found, err := LoadCurrentGlobalCampaign(ctx, db, campaignRef)
	if err != nil || !found {
		t.Fatalf("load campaign after ambiguous input: found=%v err=%v", found, err)
	}
	if currentAfterAmbiguous.Revision != beforeAmbiguousRevision {
		t.Fatalf("ambiguous lookup mutated campaign: revision=%d want=%d", currentAfterAmbiguous.Revision, beforeAmbiguousRevision)
	}

	if _, err := MaterializeAndPersistGlobalCampaign(ctx, db, GlobalCampaignMaterializerInput{
		ObservedAt:     t0.Add(5 * time.Minute),
		RelationRefs:   []string{relationB},
		RulesetVersion: "campaign-ci.v2",
	}); !errors.Is(err, ErrGlobalCampaignRulesetConflict) {
		t.Fatalf("ruleset conflict error=%v want=%v", err, ErrGlobalCampaignRulesetConflict)
	}

	if _, err := MaterializeAndPersistGlobalCampaign(ctx, db, GlobalCampaignMaterializerInput{
		ObservedAt:          t0.Add(5 * time.Minute),
		Subjects:            []string{prefix + ":subject:entity-only"},
		VerifiedAnchorCount: 1,
	}); !errors.Is(err, ErrGlobalCampaignLookupEvidenceRequired) {
		t.Fatalf("entity-only candidate error=%v want=%v", err, ErrGlobalCampaignLookupEvidenceRequired)
	}

	concurrentRelationX := prefix + ":relation:concurrent-x"
	concurrentRelationY := prefix + ":relation:concurrent-y"
	concurrentInputs := []GlobalCampaignMaterializerInput{
		{
			ObservedAt:     t0.Add(6 * time.Minute),
			RelationRefs:   []string{relationA, concurrentRelationX},
			RulesetVersion: "campaign-ci.v1",
		},
		{
			ObservedAt:      t0.Add(7 * time.Minute),
			ObservationRefs: []string{sharedObservation},
			RelationRefs:    []string{concurrentRelationY},
			RulesetVersion:  "campaign-ci.v1",
		},
	}

	start := make(chan struct{})
	results := make(chan GlobalCampaignPersistResult, len(concurrentInputs))
	errs := make(chan error, len(concurrentInputs))
	var wg sync.WaitGroup
	for _, input := range concurrentInputs {
		input := input
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, mergeErr := MaterializeAndPersistGlobalCampaign(ctx, db, input)
			if mergeErr != nil {
				errs <- mergeErr
				return
			}
			results <- result
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for mergeErr := range errs {
		t.Fatalf("concurrent disjoint-anchor merge failed: %v", mergeErr)
	}
	var concurrentResults []GlobalCampaignPersistResult
	for result := range results {
		concurrentResults = append(concurrentResults, result)
	}
	if len(concurrentResults) != 2 {
		t.Fatalf("concurrent result count=%d want=2", len(concurrentResults))
	}
	for _, result := range concurrentResults {
		if !result.Inserted || result.Campaign.CampaignRef != campaignRef {
			t.Fatalf("concurrent merge did not converge: %+v", result)
		}
	}

	finalCampaign, found, err := LoadCurrentGlobalCampaign(ctx, db, campaignRef)
	if err != nil || !found {
		t.Fatalf("load final campaign: found=%v err=%v", found, err)
	}
	if finalCampaign.Revision != beforeAmbiguousRevision+2 {
		t.Fatalf("concurrent merges revision=%d want=%d", finalCampaign.Revision, beforeAmbiguousRevision+2)
	}
	if !containsGlobalCampaignString(finalCampaign.RelationRefs, concurrentRelationX) || !containsGlobalCampaignString(finalCampaign.RelationRefs, concurrentRelationY) {
		t.Fatalf("concurrent evidence did not converge: %#v", finalCampaign.RelationRefs)
	}

	var staleAnchorRows int
	if err := db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM global_campaign_anchor_index AS anchor
		JOIN global_campaign_current AS current_campaign
		  ON current_campaign.campaign_ref=anchor.campaign_ref
		WHERE anchor.campaign_ref=$1
		  AND anchor.revision<>current_campaign.revision`, campaignRef,
	).Scan(&staleAnchorRows); err != nil {
		t.Fatal(err)
	}
	if staleAnchorRows != 0 {
		t.Fatalf("anchor index has %d stale revision rows", staleAnchorRows)
	}
}

func containsGlobalCampaignString(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}
