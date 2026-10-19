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
	// A configured, explicitly trusted Ed25519 public-key registry permits
	// recovery ONLY from a source-bound, authenticated canonical final receipt.
	// This never infers completion from a legacy arm verdict's signed=true.
	registry, configured, trustErr := arvisStreamRecoveryTrustRegistry()
	if trustErr != nil {
		log.Printf("arvis stale stream recovery trusted registry unavailable: %v", trustErr)
	} else if configured {
		arvisReconcileStaleCanonicalReceipts(ctx, db, registry)
	}
	// Any remaining processing lease has no accepted canonical final. Keep it
	// retryable/failed (or exhausted); never silently declare completion.
	result, err := db.ExecContext(ctx, `
		UPDATE arvis_stream_processing
		SET status=CASE
		             WHEN attempts >= 3 THEN 'exhausted'
		             ELSE 'failed'
		           END,
		    processed_at=NULL,
		    last_error='recovered stale processing lease; canonical final not verified',
		    updated_at=CASE
		                 WHEN attempts >= 5 THEN now()
		                 WHEN attempts >= 3 THEN now() - interval '31 minutes'
		                 ELSE now() - interval '1 minute'
		               END
		WHERE status='processing'
		  AND updated_at < now() - interval '5 minutes'
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
