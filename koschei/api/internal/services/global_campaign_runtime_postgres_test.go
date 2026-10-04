package services

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"
)

func TestGlobalCampaignRuntimePostgres17(t *testing.T) {
	url := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := VerifyGlobalCampaignRuntimeSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	// Never clear arbitrary data: require an empty, isolated test queue.
	var occupied bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM global_campaign_runtime_queue)`).Scan(&occupied); err != nil {
		t.Fatal(err)
	}
	if occupied {
		t.Fatal("campaign runtime acceptance requires an empty isolated test database")
	}
	base := time.Now().UTC()
	a, b, bridge := testGlobalCampaignRadarBridge(t, base, IntelligenceEvidenceVerified, IntelligenceEvidenceVerified)
	snapshot, err := BuildGlobalRadarSnapshot([]GlobalRadarObservation{a, b}, []GlobalRadarRelationEdge{bridge.Relation}, []GlobalRadarBridgeLink{bridge}, nil, base.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := buildGlobalCampaignRuntimeJobs(snapshot)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("fixture: %v", err)
	}
	sourceRef := globalCampaignRuntimeSourceRef(jobs[0])
	campaignRef := MaterializeGlobalCampaign(jobs[0].Input).CampaignRef
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM global_campaign_runtime_queue WHERE source_ref=$1`, sourceRef)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM global_campaign_current WHERE campaign_ref=$1`, campaignRef)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM global_campaign_revisions WHERE campaign_ref=$1`, campaignRef)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM global_campaign_worker_leases WHERE lease_key=$1 AND lease_owner='runtime-acceptance'`, globalCampaignRuntimeLeaseKey)
	})
	for i, want := range []int{1, 0} {
		n, err := EnqueueGlobalCampaignRadarSnapshot(ctx, db, snapshot)
		if err != nil || n != want {
			t.Fatalf("enqueue %d: inserted=%d error=%v", i, n, err)
		}
	}
	var now time.Time
	if err := db.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		t.Fatal(err)
	}
	lease, err := AcquireGlobalCampaignWorkerLease(ctx, db, globalCampaignRuntimeLeaseKey, "runtime-acceptance", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	worked, err := ProcessGlobalCampaignRuntimeJob(ctx, db, lease)
	if err != nil || !worked {
		t.Fatalf("materialize/ack: %v %v", worked, err)
	}
	worked, err = ProcessGlobalCampaignRuntimeJob(ctx, db, lease)
	if err != nil || worked {
		t.Fatalf("replay processed twice: %v %v", worked, err)
	}
	status, results, err := ReadGlobalCampaignRuntime(ctx, db, true)
	if err != nil || status.Processed != 1 || status.Pending != 0 || len(results) != 1 {
		t.Fatalf("read: %+v results=%d error=%v", status, len(results), err)
	}
	result := results[0]
	if result.Command.ContainmentVerified || result.Command.ResponseState != GlobalCampaignCommandCenterMonitoring || result.Fabric.ContainmentAuthority {
		t.Fatal("runtime created response authority")
	}
	if result.Campaign.Revision != 1 || len(result.Radar.CrossNetworkContinuity) != 1 {
		t.Fatal("canonical revision or continuity missing")
	}
	if err := ReleaseGlobalCampaignWorkerLease(ctx, db, lease, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	next, err := AcquireGlobalCampaignWorkerLease(ctx, db, globalCampaignRuntimeLeaseKey, lease.LeaseOwner, time.Now().UTC(), time.Minute)
	if err != nil || next.FencingToken <= lease.FencingToken {
		t.Fatalf("same owner generation reused: %+v %v", next, err)
	}
	if _, err := ProcessGlobalCampaignRuntimeJob(ctx, db, lease); !errors.Is(err, ErrGlobalCampaignWorkerLeaseFenced) {
		t.Fatalf("stale worker permitted: %v", err)
	}
	// Reject a corrupted durable handoff before any campaign writes.
	if _, err := db.ExecContext(ctx, `UPDATE global_campaign_runtime_queue SET status='pending',command_payload=NULL,reference_payload='{}'::jsonb WHERE source_ref=$1`, sourceRef); err != nil {
		t.Fatal(err)
	}
	if _, err := ProcessGlobalCampaignRuntimeJob(ctx, db, next); !errors.Is(err, ErrGlobalCampaignRuntimeInput) {
		t.Fatalf("corrupt input accepted: %v", err)
	}
	status, _, err = ReadGlobalCampaignRuntime(ctx, db, true)
	if err != nil || status.Failed != 1 || status.Pending != 0 {
		t.Fatalf("poison job was not quarantined: %+v %v", status, err)
	}
	var revisions int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM global_campaign_revisions WHERE campaign_ref=$1`, result.Campaign.CampaignRef).Scan(&revisions); err != nil || revisions != 1 {
		t.Fatalf("corruption wrote a revision: %d %v", revisions, err)
	}
}
