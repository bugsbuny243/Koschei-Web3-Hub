package services

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"koschei/api/internal/runtimehealth"

	_ "github.com/lib/pq"
)

func TestCoverageLifecycleAction(t *testing.T) {
	open := coverageEpisode{
		ID:             "episode-1",
		ComponentID:    "head",
		CoverageStatus: runtimehealth.CoverageBlindSpot,
		CoverageReason: "head_ingest_freshness_deadline_expired",
		OpenAlertID:    "alert-1",
	}
	tests := []struct {
		name    string
		entry   runtimehealth.Entry
		hasOpen bool
		open    coverageEpisode
		want    string
	}{
		{
			name: "new attention opens episode",
			entry: runtimehealth.Entry{
				Kind:              "head_ingest",
				CoverageStatus:    runtimehealth.CoverageBlindSpot,
				CoverageReason:    "head_ingest_freshness_deadline_expired",
				CoverageAttention: true,
			},
			want: "open",
		},
		{
			name: "missing alert is repaired",
			entry: runtimehealth.Entry{
				Kind:              "head_ingest",
				CoverageStatus:    runtimehealth.CoverageBlindSpot,
				CoverageReason:    "head_ingest_freshness_deadline_expired",
				CoverageAttention: true,
			},
			hasOpen: true,
			open: coverageEpisode{
				ID:             "episode-1",
				CoverageStatus: runtimehealth.CoverageBlindSpot,
				CoverageReason: "head_ingest_freshness_deadline_expired",
			},
			want: "ensure_alert",
		},
		{
			name: "escalation updates same episode",
			entry: runtimehealth.Entry{
				Kind:              "head_ingest",
				CoverageStatus:    runtimehealth.CoverageReorgGuard,
				CoverageReason:    "canonical_lineage_recheck_required",
				CoverageAttention: true,
			},
			hasOpen: true,
			open:    open,
			want:    "update",
		},
		{
			name: "same attention stays deduped",
			entry: runtimehealth.Entry{
				Kind:              "head_ingest",
				CoverageStatus:    runtimehealth.CoverageBlindSpot,
				CoverageReason:    "head_ingest_freshness_deadline_expired",
				CoverageAttention: true,
			},
			hasOpen: true,
			open:    open,
			want:    "none",
		},
		{
			name: "catching up does not close episode",
			entry: runtimehealth.Entry{
				Kind:           "head_ingest",
				CoverageStatus: runtimehealth.CoverageLagging,
				CoverageReason: "durable_cursor_is_behind_observed_head",
			},
			hasOpen: true,
			open:    open,
			want:    "none",
		},
		{
			name: "current recovers episode",
			entry: runtimehealth.Entry{
				Kind:           "head_ingest",
				CoverageStatus: runtimehealth.CoverageCurrent,
				CoverageReason: "durable_cursor_matches_observed_provider_head",
			},
			hasOpen: true,
			open:    open,
			want:    "recover",
		},
		{
			name: "non ingest is ignored",
			entry: runtimehealth.Entry{
				Kind:              "storage",
				CoverageAttention: true,
			},
			hasOpen: true,
			open:    open,
			want:    "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := coverageLifecycleAction(tt.entry, tt.hasOpen, tt.open); got != tt.want {
				t.Fatalf("action=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestCoverageAlertSeverity(t *testing.T) {
	if got := coverageAlertSeverity(runtimehealth.CoverageBlindSpot); got != "high" {
		t.Fatalf("blind spot severity=%q", got)
	}
	if got := coverageAlertSeverity(runtimehealth.CoverageReorgGuard); got != "high" {
		t.Fatalf("reorg severity=%q", got)
	}
	if got := coverageAlertSeverity(runtimehealth.CoverageLagging); got != "medium" {
		t.Fatalf("lagging severity=%q", got)
	}
	if got := coverageAlertSeverity(runtimehealth.CoverageCurrent); got != "info" {
		t.Fatalf("current severity=%q", got)
	}
}


func TestGlobalRadarCoverageAlertLifecyclePostgres17(t *testing.T) {
	databaseURL := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	t.Setenv("DISCORD_WEBHOOK_URL", "")

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	componentID := fmt.Sprintf("coverage-test-%d", time.Now().UnixNano())
	networkID := "ethereum-mainnet"
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM global_radar_coverage_episodes WHERE component_id=$1", componentID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM security_alert_events WHERE payload->>'component_id'=$1", componentID)
	})

	health := runtimehealth.New()
	health.Register(componentID, "head_ingest", networkID, true)
	state := &coverageLifecycleState{open: map[string]coverageEpisode{}}

	head, cursor := uint64(99), uint64(100)
	health.RecordIngestProgress(componentID, &head, &cursor, true, false)
	if err := reconcileGlobalRadarCoverageAlerts(ctx, db, health, state); err != nil {
		t.Fatal(err)
	}

	var firstEpisodeID, openAlertID, episodeStatus string
	if err := db.QueryRowContext(ctx, `
		SELECT id::text,COALESCE(open_alert_id::text,''),status
		FROM global_radar_coverage_episodes
		WHERE component_id=$1
		ORDER BY opened_at DESC
		LIMIT 1`, componentID).Scan(&firstEpisodeID, &openAlertID, &episodeStatus); err != nil {
		t.Fatal(err)
	}
	if firstEpisodeID == "" || openAlertID == "" || episodeStatus != "open" {
		t.Fatalf("unexpected open episode id=%q alert=%q status=%q", firstEpisodeID, openAlertID, episodeStatus)
	}

	if err := reconcileGlobalRadarCoverageAlerts(ctx, db, health, state); err != nil {
		t.Fatal(err)
	}
	var occurrenceCount int
	if err := db.QueryRowContext(ctx, "SELECT occurrence_count FROM security_alert_events WHERE id=$1", openAlertID).Scan(&occurrenceCount); err != nil {
		t.Fatal(err)
	}
	if occurrenceCount != 1 {
		t.Fatalf("same coverage observation emitted duplicate alert occurrence_count=%d", occurrenceCount)
	}

	head = 100
	cursor = 100
	health.RecordIngestProgress(componentID, &head, &cursor, false, false)
	if err := reconcileGlobalRadarCoverageAlerts(ctx, db, health, state); err != nil {
		t.Fatal(err)
	}
	var recoveredAt sql.NullTime
	var recoveryAlertID string
	if err := db.QueryRowContext(ctx, `
		SELECT status,recovered_at,COALESCE(recovery_alert_id::text,'')
		FROM global_radar_coverage_episodes
		WHERE id=$1`, firstEpisodeID).Scan(&episodeStatus, &recoveredAt, &recoveryAlertID); err != nil {
		t.Fatal(err)
	}
	if episodeStatus != "recovered" || !recoveredAt.Valid || recoveryAlertID == "" {
		t.Fatalf("episode did not recover status=%q recovered=%v alert=%q", episodeStatus, recoveredAt.Valid, recoveryAlertID)
	}

	head = 99
	health.RecordIngestProgress(componentID, &head, &cursor, true, false)
	if err := reconcileGlobalRadarCoverageAlerts(ctx, db, health, state); err != nil {
		t.Fatal(err)
	}
	var episodeCount, openCount int
	if err := db.QueryRowContext(ctx, `
		SELECT count(*),count(*) FILTER (WHERE status='open')
		FROM global_radar_coverage_episodes
		WHERE component_id=$1`, componentID).Scan(&episodeCount, &openCount); err != nil {
		t.Fatal(err)
	}
	if episodeCount != 2 || openCount != 1 {
		t.Fatalf("reopen did not create a new single open episode total=%d open=%d", episodeCount, openCount)
	}
}
