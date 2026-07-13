package services

import (
	"context"
	"database/sql"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Radar retention worker.
//
// The stream pipeline intentionally stores an evidence trail. Hot tokens can
// therefore produce thousands of rows per day. Cleanup runs independently of
// RPC background workers so quota-saver mode cannot disable database hygiene.
//
// Config:
//
//	KOSCHEI_RADAR_RETENTION_DISABLED=1        worker off
//	KOSCHEI_RADAR_RETENTION_DAYS=30           window (min 7, max 365)
//	KOSCHEI_RADAR_RETENTION_INTERVAL_HOURS=12 frequency (min 1, max 48)
const radarRetentionBatchSize = 5000
const radarRetentionMaxBatchesPerTable = 40 // hard cap: 200k rows/table/run

func StartSecurityRadarRetentionWorker(ctx context.Context, db *sql.DB) func() {
	if db == nil || envBool("KOSCHEI_RADAR_RETENTION_DISABLED") {
		return func() {}
	}
	workerCtx, cancel := context.WithCancel(ctx)
	go (&securityRadarRetentionWorker{
		db:       db,
		days:     radarRetentionDays(),
		interval: radarRetentionInterval(),
	}).start(workerCtx)
	return cancel
}

type securityRadarRetentionWorker struct {
	db       *sql.DB
	days     int
	interval time.Duration
}

func (w *securityRadarRetentionWorker) start(ctx context.Context) {
	if w == nil || w.db == nil {
		return
	}
	log.Printf("radar retention worker started window=%dd interval=%s", w.days, w.interval)
	w.runOnce(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("radar retention worker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *securityRadarRetentionWorker) runOnce(ctx context.Context) {
	cutoff := time.Now().UTC().AddDate(0, 0, -w.days)

	// Verdicts go first. Events are removed only after no verdict references
	// them. arvis_stream_processing rows cascade when stream events are deleted.
	verdicts := w.deleteBatched(ctx, "security_radar_verdicts", `
		DELETE FROM security_radar_verdicts
		WHERE id IN (
			SELECT id FROM security_radar_verdicts
			WHERE created_at < $1
			ORDER BY created_at ASC
			LIMIT $2
		)`, cutoff)

	events := w.deleteBatched(ctx, "security_radar_events", `
		DELETE FROM security_radar_events
		WHERE id IN (
			SELECT e.id FROM security_radar_events e
			WHERE e.created_at < $1
			  AND NOT EXISTS (
				SELECT 1 FROM security_radar_verdicts v WHERE v.event_id = e.id
			  )
			ORDER BY e.created_at ASC
			LIMIT $2
		)`, cutoff)

	seen := w.deleteBatched(ctx, "security_radar_seen_signatures", `
		DELETE FROM security_radar_seen_signatures
		WHERE id IN (
			SELECT id FROM security_radar_seen_signatures
			WHERE created_at < $1
			ORDER BY created_at ASC
			LIMIT $2
		)`, cutoff)

	stream := w.deleteBatched(ctx, "security_radar_stream_events", `
		DELETE FROM security_radar_stream_events
		WHERE id IN (
			SELECT id FROM security_radar_stream_events
			WHERE created_at < $1
			ORDER BY created_at ASC
			LIMIT $2
		)`, cutoff)

	trades := w.deleteBatched(ctx, "token_trade_events", `
		DELETE FROM token_trade_events
		WHERE id IN (
			SELECT t.id FROM token_trade_events t
			WHERE COALESCE(t.block_time,t.created_at) < now()-interval '72 hours'
			  AND NOT EXISTS (
				SELECT 1 FROM watchlist_targets w
				WHERE w.status='active' AND w.target=t.mint
			  )
			  AND NOT EXISTS (
				SELECT 1 FROM security_radar_verdicts v
				WHERE v.target=t.mint AND v.created_at >= $1
			  )
			ORDER BY COALESCE(t.block_time,t.created_at) ASC
			LIMIT $2
		)`, cutoff)

	// token_structural_signals is deliberately retained. It is one row per
	// token and the floor reader already ignores stale holder/authority clocks.
	if verdicts+events+seen+stream+trades > 0 {
		log.Printf("radar retention: removed verdicts=%d events=%d seen_signatures=%d stream_events=%d trade_events=%d (scan cutoff %s; unprotected trades also require age >72h)",
			verdicts, events, seen, stream, trades, cutoff.Format(time.RFC3339))
	}
}

// deleteBatched deletes fixed-size chunks until clean or the per-run cap is
// reached. Missing tables and query failures never crash the service.
func (w *securityRadarRetentionWorker) deleteBatched(ctx context.Context, table, query string, cutoff time.Time) int64 {
	var total int64
	for batch := 0; batch < radarRetentionMaxBatchesPerTable; batch++ {
		if ctx.Err() != nil {
			return total
		}
		stepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		result, err := w.db.ExecContext(stepCtx, query, cutoff, radarRetentionBatchSize)
		cancel()
		if err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "does not exist") {
				log.Printf("radar retention: %s cleanup error: %v", table, err)
			}
			return total
		}
		affected, _ := result.RowsAffected()
		total += affected
		if affected < radarRetentionBatchSize {
			return total
		}
	}
	log.Printf("radar retention: %s hit per-run batch cap; remaining rows will be cleaned next run", table)
	return total
}

func radarRetentionDays() int {
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_RADAR_RETENTION_DAYS")); raw != "" {
		if days, err := strconv.Atoi(raw); err == nil && days >= 7 && days <= 365 {
			return days
		}
	}
	return 30
}

func radarRetentionInterval() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_RADAR_RETENTION_INTERVAL_HOURS")); raw != "" {
		if hours, err := strconv.Atoi(raw); err == nil && hours >= 1 && hours <= 48 {
			return time.Duration(hours) * time.Hour
		}
	}
	return 12 * time.Hour
}
