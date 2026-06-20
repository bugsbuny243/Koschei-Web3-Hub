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
		UPDATE arvis_stream_processing
		SET status='failed',
		    last_error='recovered stale processing lease',
		    updated_at=now()
		WHERE status='processing'
		  AND updated_at < now() - interval '5 minutes'
		  AND attempts < 3
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
