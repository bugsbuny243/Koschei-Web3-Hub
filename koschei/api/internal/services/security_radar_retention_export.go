package services

import (
	"context"
	"fmt"
	"log"

	"koschei/api/internal/services/retentionexport"
)

func (w *securityRadarRetentionWorker) exportPendingArchive(ctx context.Context, phase string) error {
	result, err := retentionexport.RunFromEnv(ctx, w.db)
	if !result.Enabled {
		// A disabled/unconfigured export sink cannot authorize source deletion.
		// PostgreSQL staging rows alone are not an independently retained
		// archive; refuse both the current v2 and the legacy rollback worker.
		// Do not include a provider URL or credentials in the error.
		return fmt.Errorf("%s archive export preflight failed: durable destination disabled", phase)
	}
	if err != nil {
		return fmt.Errorf("%s archive export failed: %w", phase, err)
	}
	if result.ConnectionVerified {
		log.Printf("radar archive sink connection verified sink=%s phase=%s restore_proof=false", result.Sink, phase)
	}
	if result.ExportedRows > 0 {
		log.Printf("radar retention export run %s phase=%s sink=%s rows=%d objects=%d bytes=%d ref=%s",
			result.RunID, phase, result.Sink, result.ExportedRows, result.ObjectCount,
			result.BytesExported, result.LastExportRef)
	}
	return nil
}
