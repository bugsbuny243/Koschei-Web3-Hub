package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"koschei/api/internal/alerts"
	"koschei/api/internal/runtimehealth"
)

const (
	globalRadarCoverageAlertInterval = 30 * time.Second
	globalRadarCoverageAlertHealthID = "worker.global-radar-coverage-alert-lifecycle"
)

type coverageEpisode struct {
	ID             string
	ComponentID    string
	NetworkID      string
	CoverageStatus string
	CoverageReason string
	OpenAlertID    string
}

type coverageLifecycleState struct {
	open   map[string]coverageEpisode
	loaded bool
}

func StartGlobalRadarCoverageAlertLifecycle(parent context.Context, db *sql.DB, health *runtimehealth.Registry) func() {
	if db == nil || health == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(parent)
	health.RegisterPeriodic(globalRadarCoverageAlertHealthID, "worker", "", true, 2*globalRadarCoverageAlertInterval+15*time.Second)
	var once sync.Once
	go func() {
		state := &coverageLifecycleState{open: map[string]coverageEpisode{}}
		run := func() {
			if err := reconcileGlobalRadarCoverageAlerts(ctx, db, health, state); err != nil {
				health.Failure(globalRadarCoverageAlertHealthID, err)
				log.Printf("global radar coverage alert lifecycle: %v", err)
				return
			}
			health.Success(globalRadarCoverageAlertHealthID, 0)
		}
		run()
		ticker := time.NewTicker(globalRadarCoverageAlertInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				health.Stop(globalRadarCoverageAlertHealthID)
				return
			case <-ticker.C:
				run()
			}
		}
	}()
	return func() { once.Do(cancel) }
}

func reconcileGlobalRadarCoverageAlerts(ctx context.Context, db *sql.DB, health *runtimehealth.Registry, state *coverageLifecycleState) error {
	if state == nil {
		return fmt.Errorf("coverage lifecycle state is required")
	}
	if !state.loaded {
		open, err := loadOpenCoverageEpisodes(ctx, db)
		if err != nil {
			return fmt.Errorf("load open coverage episodes: %w", err)
		}
		state.open = open
		state.loaded = true
	}

	for _, entry := range health.Snapshot().Entries {
		if entry.Kind != "head_ingest" {
			continue
		}
		current, hasOpen := state.open[entry.ID]
		action := coverageLifecycleAction(entry, hasOpen, current)
		switch action {
		case "open":
			episode, err := ensureOpenCoverageEpisode(ctx, db, entry)
			if err != nil {
				return err
			}
			if err := ensureCoverageAttentionAlert(ctx, db, entry, &episode, true); err != nil {
				return err
			}
			state.open[entry.ID] = episode
		case "update":
			episode := current
			if _, err := db.ExecContext(ctx, `
				UPDATE global_radar_coverage_episodes
				SET coverage_status=$1,coverage_reason=$2,updated_at=now()
				WHERE id=$3 AND status='open'`,
				entry.CoverageStatus, entry.CoverageReason, episode.ID); err != nil {
				return fmt.Errorf("update coverage episode: %w", err)
			}
			episode.CoverageStatus = entry.CoverageStatus
			episode.CoverageReason = entry.CoverageReason
			if err := ensureCoverageAttentionAlert(ctx, db, entry, &episode, true); err != nil {
				return err
			}
			state.open[entry.ID] = episode
		case "ensure_alert":
			episode := current
			if err := ensureCoverageAttentionAlert(ctx, db, entry, &episode, false); err != nil {
				return err
			}
			state.open[entry.ID] = episode
		case "recover":
			if err := recoverCoverageEpisode(ctx, db, entry, current); err != nil {
				return err
			}
			delete(state.open, entry.ID)
		}
	}
	return nil
}

func coverageLifecycleAction(entry runtimehealth.Entry, hasOpen bool, open coverageEpisode) string {
	if entry.Kind != "head_ingest" {
		return "none"
	}
	if entry.CoverageAttention {
		if !hasOpen {
			return "open"
		}
		if open.OpenAlertID == "" {
			return "ensure_alert"
		}
		if open.CoverageStatus != entry.CoverageStatus || open.CoverageReason != entry.CoverageReason {
			return "update"
		}
		return "none"
	}
	if hasOpen && entry.CoverageStatus == runtimehealth.CoverageCurrent {
		return "recover"
	}
	return "none"
}

func loadOpenCoverageEpisodes(ctx context.Context, db *sql.DB) (map[string]coverageEpisode, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id::text,component_id,network_id,coverage_status,coverage_reason,COALESCE(open_alert_id::text,'')
		FROM global_radar_coverage_episodes
		WHERE status='open'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]coverageEpisode{}
	for rows.Next() {
		var item coverageEpisode
		if err := rows.Scan(&item.ID, &item.ComponentID, &item.NetworkID, &item.CoverageStatus, &item.CoverageReason, &item.OpenAlertID); err != nil {
			return nil, err
		}
		out[item.ComponentID] = item
	}
	return out, rows.Err()
}

func ensureOpenCoverageEpisode(ctx context.Context, db *sql.DB, entry runtimehealth.Entry) (coverageEpisode, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return coverageEpisode{}, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, entry.ID); err != nil {
		return coverageEpisode{}, err
	}

	var item coverageEpisode
	err = tx.QueryRowContext(ctx, `
		SELECT id::text,component_id,network_id,coverage_status,coverage_reason,COALESCE(open_alert_id::text,'')
		FROM global_radar_coverage_episodes
		WHERE component_id=$1 AND status='open'
		LIMIT 1`, entry.ID).Scan(
		&item.ID, &item.ComponentID, &item.NetworkID, &item.CoverageStatus, &item.CoverageReason, &item.OpenAlertID,
	)
	if err == sql.ErrNoRows {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO global_radar_coverage_episodes
				(component_id,network_id,status,coverage_status,coverage_reason)
			VALUES ($1,$2,'open',$3,$4)
			RETURNING id::text,component_id,network_id,coverage_status,coverage_reason,COALESCE(open_alert_id::text,'')`,
			entry.ID, entry.NetworkID, entry.CoverageStatus, entry.CoverageReason,
		).Scan(&item.ID, &item.ComponentID, &item.NetworkID, &item.CoverageStatus, &item.CoverageReason, &item.OpenAlertID)
	}
	if err != nil {
		return coverageEpisode{}, err
	}
	if err := tx.Commit(); err != nil {
		return coverageEpisode{}, err
	}
	return item, nil
}

func ensureCoverageAttentionAlert(ctx context.Context, db *sql.DB, entry runtimehealth.Entry, episode *coverageEpisode, repeat bool) error {
	if episode == nil || strings.TrimSpace(episode.ID) == "" {
		return fmt.Errorf("coverage episode is required")
	}
	if episode.OpenAlertID != "" && !repeat {
		return nil
	}
	severity := coverageAlertSeverity(entry.CoverageStatus)
	alertID, err := alerts.Emit(ctx, db, alerts.Event{
		Source:    "global_radar_runtime",
		EventType: "global_radar.coverage.attention",
		Severity:  severity,
		Target:    entry.NetworkID,
		Title:     "Global Radar coverage attention required",
		Message: fmt.Sprintf(
			"Operational coverage status=%s reason=%s. This is a monitoring-confidence signal, not proof of historical chain-data loss.",
			entry.CoverageStatus, entry.CoverageReason,
		),
		DedupeKey: "global-radar-coverage-open:" + episode.ID,
		Payload: map[string]any{
			"component_id":    entry.ID,
			"network_id":      entry.NetworkID,
			"coverage_status": entry.CoverageStatus,
			"coverage_reason": entry.CoverageReason,
			"truth_boundary":  "operational_monitoring_signal_not_historical_data_loss_proof",
		},
	})
	if err != nil {
		return fmt.Errorf("emit coverage attention alert: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE global_radar_coverage_episodes
		SET open_alert_id=$1,updated_at=now()
		WHERE id=$2 AND status='open'`, alertID, episode.ID); err != nil {
		return fmt.Errorf("link coverage attention alert: %w", err)
	}
	episode.OpenAlertID = alertID
	return nil
}

func recoverCoverageEpisode(ctx context.Context, db *sql.DB, entry runtimehealth.Entry, episode coverageEpisode) error {
	alertID, err := alerts.Emit(ctx, db, alerts.Event{
		Source:    "global_radar_runtime",
		EventType: "global_radar.coverage.recovered",
		Severity:  "low",
		Target:    entry.NetworkID,
		Title:     "Global Radar coverage returned to current",
		Message:   "The durable cursor again matches the observed provider head. This recovery does not establish historical completeness or finality.",
		DedupeKey: "global-radar-coverage-recovered:" + episode.ID,
		Payload: map[string]any{
			"component_id":    entry.ID,
			"network_id":      entry.NetworkID,
			"coverage_status": entry.CoverageStatus,
			"coverage_reason": entry.CoverageReason,
			"truth_boundary":  "current_head_alignment_not_historical_completeness_proof",
		},
	})
	if err != nil {
		return fmt.Errorf("emit coverage recovery alert: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, entry.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE global_radar_coverage_episodes
		SET status='recovered',coverage_status='current',coverage_reason=$1,
		    recovery_alert_id=$2,recovered_at=now(),updated_at=now()
		WHERE id=$3 AND status='open'`,
		entry.CoverageReason, alertID, episode.ID); err != nil {
		return err
	}
	return tx.Commit()
}

func coverageAlertSeverity(status string) string {
	switch status {
	case runtimehealth.CoverageBlindSpot, runtimehealth.CoverageReorgGuard:
		return "high"
	case runtimehealth.CoverageLagging:
		return "medium"
	default:
		return "info"
	}
}
