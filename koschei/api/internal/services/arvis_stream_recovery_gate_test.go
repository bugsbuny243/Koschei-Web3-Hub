package services

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// A real PostgreSQL stream event and stale claim prove the actual production
// watcher starts its database recovery even while costly RPC scanning is off.
// The fixture event is fresh and explicitly cleaned up; it cannot be mistaken
// for a live on-chain observation or signed canonical final.
func TestArvisStreamRecoveryIndependentOfScanAndRPCGatesPostgres17(t *testing.T) {
	url := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL not configured")
	}

	for _, tc := range []struct {
		name     string
		autoScan string
		rpcSaver string
	}{
		{name: "automatic_scanning_disabled", autoScan: "false", rpcSaver: "true"},
		{name: "rpc_saver_enabled", autoScan: "true", rpcSaver: "true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KOSCHEI_OWNER_UNLIMITED_AUTOSCAN_ENABLED", "false")
			t.Setenv("KOSCHEI_AUTOMATIC_SCANNING_ENABLED", tc.autoScan)
			t.Setenv("SOLANA_RPC_LIMIT_SAVER_ENABLED", tc.rpcSaver)
			t.Setenv("KOSCHEI_AUTO_RADAR_FORCE_BACKGROUND", "false")
			t.Setenv("RADAR_STREAM_FORCE_ENABLED", "false")
			t.Setenv("ARVIS_BACKGROUND_RPC_ENRICHMENT_ENABLED", "false")
			t.Setenv("RADAR_STREAM_GAP_HEALER_ENABLED", "false")
			t.Setenv("ARVIS_STREAM_VERDICT_DISABLED", "true")
			t.Setenv(arvisStreamTrustedKeysEnv, "")

			if tc.autoScan == "false" && AutomaticBackgroundScanningEnabled() {
				t.Fatal("test setup accidentally enabled broad scanning")
			}
			if tc.autoScan == "true" && (!AutomaticBackgroundScanningEnabled() ||
				!SolanaRPCLimitSaverEnabled() || ForceBackgroundRadarEnabled()) {
				t.Fatal("test setup did not exercise the RPC saver gate")
			}

			db, err := sql.Open("postgres", url)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(5)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := db.PingContext(ctx); err != nil {
				t.Fatal(err)
			}
			var eventID string
			const target = "So11111111111111111111111111111111111111112"
			const network = "solana-mainnet"
			const module = ModulePumpSybilRadar
			const txSignature = "ci-recovery-no-canonical-final"
			err = db.QueryRowContext(ctx, `
				INSERT INTO security_radar_stream_events
				(provider,stream_mode,network,module_id,event_type,target,target_type,signature,evidence_quality)
				VALUES('ci_fixture','test',$1,$2,'test_recovery_gating',$3,'token',$4,'fixture_only')
				RETURNING id::text
			`, network, module, target, txSignature).Scan(&eventID)
			if err != nil {
				t.Fatal(err)
			}
			// Defer cleanup BEFORE db.Close runs; t.Cleanup would execute after
			// function-level defers, leaving the fixture database connection closed.
			defer func() {
				cleanupCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
				defer done()
				_, _ = db.ExecContext(cleanupCtx, "DELETE FROM arvis_stream_processing WHERE stream_event_id=$1::uuid", eventID)
				_, _ = db.ExecContext(cleanupCtx, "DELETE FROM security_radar_stream_events WHERE id=$1::uuid", eventID)
			}()
			if _, err := db.ExecContext(ctx, `
				INSERT INTO arvis_stream_processing
				(stream_event_id,target,signature,status,attempts,updated_at)
				VALUES($1::uuid,$2,$3,'processing',1,now()-interval '10 minutes')
			`, eventID, target, txSignature); err != nil {
				t.Fatal(err)
			}

			watchCtx, stopContext := context.WithCancel(ctx)
			stop := StartSecurityRadarWatcher(watchCtx, db, nil)
			defer func() {
				stopContext()
				stop()
			}()

			ticker := time.NewTicker(25 * time.Millisecond)
			defer ticker.Stop()
			for {
				var status, reason string
				if err := db.QueryRowContext(ctx, `
					SELECT status,COALESCE(last_error,'')
					FROM arvis_stream_processing WHERE stream_event_id=$1::uuid
				`, eventID).Scan(&status, &reason); err != nil {
					t.Fatal(err)
				}
				if status == "failed" {
					if !strings.Contains(reason, "canonical final not verified") {
						t.Fatalf("recovery failed to retain honest unknown-evidence status: %q", reason)
					}
					return
				}
				if status != "processing" {
					t.Fatalf("disabled scan or RPC saver incorrectly finalized stream claim: %q", status)
				}
				select {
				case <-ctx.Done():
					t.Fatal("database-only recovery did not run under scanner/RPC saver gates")
				case <-ticker.C:
				}
			}
		})
	}
}
