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

func TestGlobalCampaignIncidentLinkPostgres17(t *testing.T) {
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
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	campaignRef := fmt.Sprintf("KCAM1-INC-%d", time.Now().UnixNano())
	observedAt := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	first := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:         campaignRef,
		Revision:            1,
		State:               GlobalCampaignEmerging,
		FirstObservedAt:     observedAt,
		LastObservedAt:      observedAt,
		Networks:            []string{"solana"},
		Subjects:            []string{"subject:incident"},
		ObservationRefs:     []string{"observation:incident"},
		ObservedAnchorCount: 1,
		RulesetVersion:      "campaign-incident-ci.v1",
	})
	if _, err := PersistGlobalCampaignRevision(ctx, db, first); err != nil {
		t.Fatalf("persist campaign revision 1: %v", err)
	}

	var incidentID, incidentRef string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO security_incident_cases (title,severity,status,network,target,summary)
		VALUES ($1,'high','open','solana',$2,'campaign linkage acceptance')
		RETURNING id::text, incident_ref`,
		"Campaign incident acceptance", "subject:incident",
	).Scan(&incidentID, &incidentRef); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM security_incident_cases WHERE id=$1::uuid`, incidentID)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM global_campaign_current WHERE campaign_ref=$1`, campaignRef)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM global_campaign_revisions WHERE campaign_ref=$1`, campaignRef)
	})

	linked, err := LinkGlobalCampaignToIncident(ctx, db, incidentID, campaignRef, "owner")
	if err != nil {
		t.Fatalf("link revision 1: %v", err)
	}
	if !linked.Inserted || linked.Idempotent || linked.CampaignRevision != 1 || linked.EvidenceHashSHA256 != first.EvidenceHashSHA256 || linked.IncidentRef != incidentRef {
		t.Fatalf("unexpected first link: %+v", linked)
	}
	if linked.VerdictAuthority || linked.ContainmentAuthority {
		t.Fatalf("link crossed authority boundary: %+v", linked)
	}

	replay, err := LinkGlobalCampaignToIncident(ctx, db, incidentID, campaignRef, "owner")
	if err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	if replay.Inserted || !replay.Idempotent || replay.CampaignRevision != 1 {
		t.Fatalf("unexpected replay: %+v", replay)
	}

	next, err := ApplyGlobalCampaignTransition(first, GlobalCampaignTransition{
		From:         GlobalCampaignEmerging,
		To:           GlobalCampaignActive,
		ReasonCodes:  []string{"incident_link_growth"},
		EvidenceRefs: []string{"relation:incident"},
	})
	if err != nil {
		t.Fatal(err)
	}
	next.LastObservedAt = observedAt.Add(time.Minute)
	next.RelationRefs = append(next.RelationRefs, "relation:incident")
	next.VerifiedAnchorCount = 1
	next = NewGlobalCampaign(next)
	if _, err := PersistGlobalCampaignRevision(ctx, db, next); err != nil {
		t.Fatalf("persist campaign revision 2: %v", err)
	}

	linkedNext, err := LinkGlobalCampaignToIncident(ctx, db, incidentID, campaignRef, "owner")
	if err != nil {
		t.Fatalf("link revision 2: %v", err)
	}
	if !linkedNext.Inserted || linkedNext.Idempotent || linkedNext.CampaignRevision != 2 || linkedNext.EvidenceHashSHA256 != next.EvidenceHashSHA256 {
		t.Fatalf("unexpected revision 2 link: %+v", linkedNext)
	}

	links, err := LoadGlobalCampaignIncidentLinks(ctx, db, incidentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 2 || links[0].CampaignRevision != 1 || links[1].CampaignRevision != 2 {
		t.Fatalf("links=%+v", links)
	}

	var actionCount int
	if err := db.QueryRowContext(ctx, `
		SELECT count(*) FROM security_incident_actions
		WHERE incident_id=$1::uuid AND action_type='link_campaign'`, incidentID,
	).Scan(&actionCount); err != nil {
		t.Fatal(err)
	}
	if actionCount != 2 {
		t.Fatalf("link_campaign actions=%d want=2", actionCount)
	}

	if _, err := LinkGlobalCampaignToIncident(ctx, db, incidentID, campaignRef+"-missing", "owner"); !errors.Is(err, ErrGlobalCampaignIncidentCampaignNotFound) {
		t.Fatalf("missing campaign error=%v", err)
	}
}
