package services

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"koschei/api/internal/services/retentionexport"

	_ "github.com/lib/pq"
)

func TestRetentionArchiveExportPostgres17(t *testing.T) {
	databaseURL := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(4)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	const sourceTable = "security_radar_seen_signatures"
	stamp := time.Now().UTC().Format("20060102150405.000000000")
	var firstID, secondID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO security_radar_seen_signatures
			(signature,module_id,source_address,network,seen_at,source_target,slot,created_at)
		VALUES ($1,'retention-export-ci','restore-source','solana-mainnet',now(),'restore-target',22345,now())
		RETURNING id::text
	`, "retention-export-a-"+stamp).Scan(&firstID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `
		INSERT INTO security_radar_seen_signatures
			(signature,module_id,source_address,network,seen_at,source_target,slot,created_at)
		VALUES ($1,'retention-export-ci','restore-source','solana-mainnet',now(),'restore-target',22346,now())
		RETURNING id::text
	`, "retention-export-b-"+stamp).Scan(&secondID); err != nil {
		t.Fatal(err)
	}

	var retentionRunID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO radar_retention_runs (id,cutoff,status,detail)
		VALUES (gen_random_uuid(),now()-interval '30 days','completed','{"ci_test":"retention_archive_export"}'::jsonb)
		RETURNING id::text`).Scan(&retentionRunID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM radar_retention_archive
			WHERE source_table=$1 AND source_id IN ($2,$3)
		`, sourceTable, firstID, secondID)
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM security_radar_seen_signatures
			WHERE id IN ($1::uuid,$2::uuid)
		`, firstID, secondID)
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM radar_retention_export_runs
			WHERE detail->>'ci_test'='retention_archive_export'
		`)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM radar_retention_runs WHERE id=$1::uuid`, retentionRunID)
	}()

	if _, err := db.ExecContext(ctx, `
		INSERT INTO radar_retention_archive
			(run_id,source_table,source_id,row_checksum,payload,archived_at)
		SELECT $1::uuid,$2,s.id::text,
		       encode(sha256(convert_to(to_jsonb(s)::text,'UTF8')),'hex'),
		       to_jsonb(s),now()-interval '10 days'
		FROM security_radar_seen_signatures s
		WHERE s.id IN ($3::uuid,$4::uuid)
	`, retentionRunID, sourceTable, firstID, secondID); err != nil {
		t.Fatal(err)
	}

	exportRoot := t.TempDir()
	sink, err := retentionexport.NewFilesystemSink(exportRoot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := (retentionexport.Exporter{
		Repository: retentionexport.NewSQLRepository(db),
		Sink:       sink,
		Config: retentionexport.Config{
			Sink: "filesystem", BatchSize: 100, MaxBatches: 2, Prefix: "ci-retention",
		},
	}).Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExportedRows != 2 || result.ObjectCount != 1 || !strings.Contains(result.LastExportRef, "#sha256=") {
		t.Fatalf("unexpected export result: %+v", result)
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE radar_retention_export_runs
		SET detail=jsonb_set(detail,'{ci_test}','"retention_archive_export"'::jsonb,true)
		WHERE id=$1::uuid`, result.RunID); err != nil {
		t.Fatal(err)
	}
	var exported int
	if err := db.QueryRowContext(ctx, `
		SELECT count(*) FROM radar_retention_archive
		WHERE source_table=$1
		  AND source_id IN ($2,$3)
		  AND exported_at IS NOT NULL
		  AND export_ref LIKE '%#sha256=%'
	`, sourceTable, firstID, secondID).Scan(&exported); err != nil {
		t.Fatal(err)
	}
	if exported != 2 {
		t.Fatalf("exported rows=%d want=2", exported)
	}

	if _, err := db.ExecContext(ctx, `
		UPDATE radar_retention_archive SET exported_at=now()-interval '8 days'
		WHERE source_table=$1 AND source_id IN ($2,$3)
	`, sourceTable, firstID, secondID); err != nil {
		t.Fatal(err)
	}
	worker := &securityRadarRetentionWorker{db: db, days: 30, interval: 12 * time.Hour}
	if pruned := worker.pruneExportedArchive(ctx); pruned < 2 {
		t.Fatalf("pruned rows=%d want at least 2", pruned)
	}
	var remaining int
	if err := db.QueryRowContext(ctx, `
		SELECT count(*) FROM radar_retention_archive
		WHERE source_table=$1 AND source_id IN ($2,$3)
	`, sourceTable, firstID, secondID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("archive rows remain after verified export prune: %d", remaining)
	}

	parts := strings.Split(result.LastExportRef, "#sha256=")
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "filesystem://") || len(parts[1]) != 64 {
		t.Fatalf("invalid export ref for restore acceptance: %q", result.LastExportRef)
	}
	exportedPath := strings.TrimPrefix(parts[0], "filesystem://")
	exportedObject, err := os.ReadFile(exportedPath)
	if err != nil {
		t.Fatal(err)
	}

	// Prove restore verification depends only on the exported bytes plus the
	// current source-table schema, not on hot or staging rows still existing.
	if _, err := db.ExecContext(ctx, `
		DELETE FROM security_radar_seen_signatures
		WHERE id IN ($1::uuid,$2::uuid)
	`, firstID, secondID); err != nil {
		t.Fatal(err)
	}

	restoreResult, err := retentionexport.VerifyRestoreAcceptance(ctx, db, exportedObject, parts[1])
	if err != nil {
		t.Fatal(err)
	}
	if restoreResult.Rows != 2 ||
		restoreResult.TypedRows != 2 ||
		restoreResult.TypedSourceTables != 1 ||
		restoreResult.SourceIDMismatches != 0 ||
		restoreResult.ChecksumMismatches != 0 ||
		restoreResult.SourceTables != 1 {
		t.Fatalf("unexpected post-prune restore acceptance: %+v", restoreResult)
	}
}
