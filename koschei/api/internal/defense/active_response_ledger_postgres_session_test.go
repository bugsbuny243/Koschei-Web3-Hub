package defense

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// This test terminates ONLY a backend belonging to the disposable CI test
// database and reconnects to verify a previously committed shadow receipt.
// It does NOT simulate PostgreSQL failover, lost quorum or an ambiguous COMMIT.
func TestActiveResponsePostgresStagingLedgerSessionLoss(t *testing.T) {
	dsn := os.Getenv("KOSCHEI_ACTIVE_RESPONSE_STAGING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated shadow PostgreSQL staging DSN not configured")
	}
	if err := validateIsolatedShadowPostgresDSN(dsn); err != nil {
		t.Fatalf("unsafe staging connection: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	var databaseName string
	if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&databaseName); err != nil {
		t.Fatal(err)
	}
	if databaseName != "koschei_shadow_test" {
		t.Fatalf("refusing test connection termination on database %q", databaseName)
	}

	input, now := trustedShadowLedgerFixture(t)
	tenant := fmt.Sprintf("shadow-session-%d", time.Now().UnixNano())
	input.Policy.TenantID = tenant
	input.Request.TenantID = tenant
	input.Asset.TenantID = tenant
	input.Provider.TenantID = tenant
	input.Recovery.TenantID = tenant
	resignShadowLedgerFixture(t, &input, now)
	recorded, err := RecordVerifiedShadowLedger(ctx, db, input, now)
	if err != nil || recorded.Status != ActiveResponseLedgerRecorded {
		t.Fatalf("could not create shadow receipt before session loss: %+v %v", recorded, err)
	}

	session, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	var backendPID int
	if err := session.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&backendPID); err != nil {
		t.Fatal(err)
	}
	killer, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var terminated bool
	if err := killer.QueryRowContext(ctx,
		"SELECT pg_terminate_backend($1)", backendPID).Scan(&terminated); err != nil {
		killer.Close()
		t.Fatal(err)
	}
	if err := killer.Close(); err != nil {
		t.Fatal(err)
	}
	if !terminated {
		t.Fatal("failed to terminate the disposable staging backend")
	}
	var one int
	if err := session.QueryRowContext(ctx, "SELECT 1").Scan(&one); err == nil {
		t.Fatal("terminated database session unexpectedly remained usable")
	}
	// Use a new connection for read-only reconciliation, NEVER an automatic
	// replay of the write that might already have committed.
	receipt, err := InspectShadowLedgerReceipt(ctx, db, tenant,
		input.Request.ActionID, input.Request.TargetID,
		input.Provider.ProviderID, recorded.PlanDigest)
	if err != nil || receipt.Status != ActiveResponseReceiptPresent ||
		receipt.ChainHash != recorded.ChainHash || receipt.ExecutionPerformed {
		t.Fatalf("committed audit receipt not recovered after session loss: %+v %v", receipt, err)
	}
}
