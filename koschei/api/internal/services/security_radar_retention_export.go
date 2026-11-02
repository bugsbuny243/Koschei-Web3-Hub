package services

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"

	"koschei/api/internal/services/retentionexport"
)

// retentionRequireExternallyVerifiedSourceArchive protects both retention
// workers, including direct archiveAndDelete calls. At present the archive
// SQL stages a row and deletes its source in the same statement, while the
// independent object export only runs afterward. A successful S3 connection
// preflight proves no individual source row was externally retained.
//
// Intentionally NO environment flag or boolean enables this operation.
// Replacing this guard requires a reviewed two-phase implementation that
// authenticates a durable per-row external archive receipt BEFORE deletion.
func retentionRequireExternallyVerifiedSourceArchive() error {
	return fmt.Errorf("external_archive_proof_before_source_delete_not_wired")
}

// Autonomous source deletion requires an independently retained, encrypted
// object destination. A filesystem sink may write to ephemeral container
// storage, and a plain HTTP S3 endpoint may expose credentials or evidence.
// The standalone exporter/restore verifier may still use local filesystem
// fixtures; only the destructive retention path is restricted here.
func retentionDurableExportPreflight() error {
	config := retentionexport.LoadConfigFromEnv()
	mode := strings.ToLower(strings.TrimSpace(config.Sink))
	if mode == "" || mode == "disabled" {
		return fmt.Errorf("durable destination disabled")
	}
	if mode != "s3" {
		return fmt.Errorf("autonomous retention requires S3 archive destination")
	}
	endpoint, err := url.Parse(config.S3Endpoint)
	if err != nil || endpoint == nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil {
		return fmt.Errorf("autonomous retention requires secure HTTPS S3 endpoint")
	}
	return nil
}

func (w *securityRadarRetentionWorker) exportPendingArchive(ctx context.Context, phase string) error {
	if err := retentionDurableExportPreflight(); err != nil {
		return fmt.Errorf("%s archive export preflight failed: %w", phase, err)
	}
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
