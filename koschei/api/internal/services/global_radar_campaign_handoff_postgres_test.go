package services

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"
)

type handoffTestSink struct {
	calls int
	fail  bool
	db    *sql.DB
}

func (s *handoffTestSink) InsertGlobalRadarSnapshot(ctx context.Context, snapshot GlobalRadarSnapshot) error {
	// A single-connection pool proves the publisher holds no SQL transaction.
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT 1`).Scan(&n); err != nil {
		return err
	}
	s.calls++
	if s.fail {
		return errors.New("uncertain external acknowledgement")
	}
	return nil
}

func TestGlobalRadarCampaignHandoffPostgres17(t *testing.T) {
	connection := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if connection == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", connection)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := VerifyGlobalCampaignRuntimeSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	var occupied bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM global_radar_campaign_handoffs) OR EXISTS(SELECT 1 FROM global_campaign_runtime_queue)`).Scan(&occupied); err != nil {
		t.Fatal(err)
	}
	if occupied {
		t.Fatal("handoff acceptance requires an empty isolated test database")
	}
	now := time.Now().UTC()
	a, b, bridge := testGlobalCampaignRadarBridge(t, now, IntelligenceEvidenceVerified, IntelligenceEvidenceVerified)
	snapshot, err := BuildGlobalRadarSnapshot([]GlobalRadarObservation{a, b}, []GlobalRadarRelationEdge{bridge.Relation}, []GlobalRadarBridgeLink{bridge}, nil, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	_, _, source, err := canonicalGlobalRadarHandoff(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := buildGlobalCampaignRuntimeJobs(snapshot)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("fixture: %v", err)
	}
	jobSource := globalCampaignRuntimeSourceRef(jobs[0])
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = db.ExecContext(cleanup, `DELETE FROM global_campaign_runtime_queue WHERE source_ref=$1`, jobSource)
		_, _ = db.ExecContext(cleanup, `DELETE FROM global_radar_campaign_handoffs WHERE source_ref=$1`, source)
	})
	for i := 0; i < 2; i++ {
		if err := StageGlobalRadarCampaignSnapshot(ctx, db, snapshot); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM global_radar_campaign_handoffs`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("stage dedup: %d %v", n, err)
	}
	first, found, err := claimGlobalRadarCampaignHandoff(ctx, db, "worker:first")
	if err != nil || !found {
		t.Fatalf("first claim: %v %v", found, err)
	}
	_, found, err = claimGlobalRadarCampaignHandoff(ctx, db, "worker:second")
	if err != nil || found {
		t.Fatalf("live lease stolen: %v %v", found, err)
	}
	// Crash after claim/publication: expiration transfers ownership and fences ACK.
	if _, err := db.ExecContext(ctx, `UPDATE global_radar_campaign_handoffs SET lease_until=clock_timestamp()-interval '1 second' WHERE source_ref=$1`, source); err != nil {
		t.Fatal(err)
	}
	second, found, err := claimGlobalRadarCampaignHandoff(ctx, db, "worker:second")
	if err != nil || !found || second.token <= first.token {
		t.Fatalf("restart recovery: %+v %v", second, err)
	}
	if err := completeGlobalRadarCampaignHandoff(ctx, db, first, snapshot); !errors.Is(err, ErrGlobalCampaignWorkerLeaseFenced) {
		t.Fatalf("stale ACK accepted: %v", err)
	}
	if err := retryGlobalRadarCampaignHandoff(ctx, db, first, false); !errors.Is(err, ErrGlobalCampaignWorkerLeaseFenced) {
		t.Fatalf("stale retry accepted: %v", err)
	}
	if err := retryGlobalRadarCampaignHandoff(ctx, db, second, false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE global_radar_campaign_handoffs SET next_attempt_at=clock_timestamp() WHERE source_ref=$1`, source); err != nil {
		t.Fatal(err)
	}
	sink := &handoffTestSink{db: db, fail: true}
	if worked, err := ProcessGlobalRadarCampaignHandoff(ctx, db, sink, "worker:publisher"); err == nil || !worked {
		t.Fatalf("publication failure acknowledged: %v %v", worked, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM global_campaign_runtime_queue`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("queue before publication ACK: %d %v", n, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE global_radar_campaign_handoffs SET next_attempt_at=clock_timestamp() WHERE source_ref=$1`, source); err != nil {
		t.Fatal(err)
	}
	sink.fail = false
	if worked, err := ProcessGlobalRadarCampaignHandoff(ctx, db, sink, "worker:restarted"); err != nil || !worked {
		t.Fatalf("recovered publication: %v %v", worked, err)
	}
	if sink.calls != 2 {
		t.Fatalf("replay count: %d", sink.calls)
	}
	var state string
	var compacted bool
	if err := db.QueryRowContext(ctx, `SELECT status,snapshot_payload IS NULL FROM global_radar_campaign_handoffs WHERE source_ref=$1`, source).Scan(&state, &compacted); err != nil || state != "complete" || !compacted {
		t.Fatalf("receipt: %s %v %v", state, compacted, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM global_campaign_runtime_queue`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("atomic enqueue: %d %v", n, err)
	}
	if err := StageGlobalRadarCampaignSnapshot(ctx, db, snapshot); err != nil {
		t.Fatal(err)
	}
	if worked, err := ProcessGlobalRadarCampaignHandoff(ctx, db, sink, "worker:replay"); err != nil || worked {
		t.Fatalf("completed source republished: %v %v", worked, err)
	}
	// Poisoned source is quarantined without calling the external publisher.
	if _, err := db.ExecContext(ctx, `UPDATE global_radar_campaign_handoffs SET status='pending',completed_at=NULL,snapshot_payload='{}',next_attempt_at=clock_timestamp() WHERE source_ref=$1`, source); err != nil {
		t.Fatal(err)
	}
	if worked, err := ProcessGlobalRadarCampaignHandoff(ctx, db, sink, "worker:poison"); !errors.Is(err, ErrGlobalCampaignRuntimeInput) || !worked {
		t.Fatalf("poison source: %v %v", worked, err)
	}
	if sink.calls != 2 {
		t.Fatal("poison source reached publisher")
	}
	status, err := readGlobalRadarCampaignHandoffStatus(ctx, db)
	if err != nil || status.Failed != 1 || status.Pending != 0 || status.Publishing != 0 {
		t.Fatalf("poison status: %+v %v", status, err)
	}
}
