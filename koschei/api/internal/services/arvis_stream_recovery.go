package services

import (
	"context"
	"database/sql"
	"log"
	"time"
)

func StartArvisStreamRecovery(ctx context.Context, db *sql.DB) func() {
	if db == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	go arvisStreamRecoveryLoop(ctx, db)
	return cancel
}

func arvisStreamRecoveryLoop(ctx context.Context, db *sql.DB) {
	arvisRecoverStaleStreamJobs(ctx, db)
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			arvisRecoverStaleStreamJobs(ctx, db)
		}
	}
}

func arvisRecoverStaleStreamJobs(ctx context.Context, db *sql.DB) {
	if db == nil {
		return
	}
	result, err := db.ExecContext(ctx, `
		WITH stale AS (
			SELECT p.stream_event_id,
			       EXISTS (
			         SELECT 1
			         FROM security_radar_verdicts v
			         WHERE v.module_id='final_verdict_engine'
			           AND v.signed=true
			           AND COALESCE(v.signals->>'source_stream_event_id','')=p.stream_event_id::text
			       ) AS has_final
			FROM arvis_stream_processing p
			WHERE p.status='processing'
			  AND p.updated_at < now() - interval '5 minutes'
		)
		UPDATE arvis_stream_processing p
		SET status=CASE
		             WHEN stale.has_final THEN 'completed'
		             WHEN p.attempts >= 3 THEN 'exhausted'
		             ELSE 'failed'
		           END,
		    processed_at=CASE
		                   WHEN stale.has_final THEN COALESCE(p.processed_at,now())
		                   ELSE p.processed_at
		                 END,
		    last_error=CASE
		                 WHEN stale.has_final THEN 'reconciled_final_verdict_exists'
		                 ELSE 'recovered stale processing lease'
		               END,
		    updated_at=CASE
		                 WHEN stale.has_final THEN now()
		                 WHEN p.attempts >= 5 THEN now()
		                 WHEN p.attempts >= 3 THEN now() - interval '31 minutes'
		                 ELSE now() - interval '1 minute'
		               END
		FROM stale
		WHERE p.stream_event_id=stale.stream_event_id
	`)
	if err != nil {
		if !isUndefinedTableError(err) {
			log.Printf("arvis stale stream recovery failed: %v", err)
		}
		return
	}
	if rows, err := result.RowsAffected(); err == nil && rows > 0 {
		log.Printf("arvis recovered %d stale stream processing jobs", rows)
	}
}
