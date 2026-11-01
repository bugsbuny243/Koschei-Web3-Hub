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

// Opt-in only: concurrency acceptance writes append-only test records to an
// isolated, disposable database named exactly koschei_shadow_test.
// It never installs schema, drops tables or accesses provider integrations.
func TestActiveResponsePostgresStagingLedgerConcurrency(t *testing.T) {
	dsn := os.Getenv("KOSCHEI_ACTIVE_RESPONSE_STAGING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("dedicated staging PostgreSQL acceptance database not configured")
	}
	if err := validateIsolatedShadowPostgresDSN(dsn); err != nil {
		t.Fatalf("refusing unsafe shadow staging database DSN: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(8)

	var dbName string
	if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&dbName); err != nil {
		t.Fatal(err)
	}
	if dbName != "koschei_shadow_test" {
		t.Fatalf("refusing concurrent shadow ledger test in database %q", dbName)
	}
	for _, table := range []string{"koschei_active_response_shadow_plans", "koschei_active_response_shadow_audit"} {
		var exists sql.NullString
		if err := db.QueryRowContext(ctx, "SELECT to_regclass($1)::text", "public."+table).Scan(&exists); err != nil || !exists.Valid {
			t.Fatalf("staging schema missing for %s: %v", table, err)
		}
	}

	base, now := trustedShadowLedgerFixture(t)
	tenant := fmt.Sprintf("shadow-concurrency-%d", time.Now().UnixNano())
	base.Policy.TenantID = tenant
	base.Request.TenantID = tenant
	base.Asset.TenantID = tenant
	base.Provider.TenantID = tenant
	base.Recovery.TenantID = tenant

	const distinctActions = 4
	inputs := make([]TrustedShadowLedgerInputs, distinctActions)
	for i := range inputs {
		inputs[i] = base
		inputs[i].Request = base.Request
		inputs[i].Request.ActionID = fmt.Sprintf("concurrent-action-%d", i)
		resignShadowLedgerFixture(t, &inputs[i], now)
	}

	type result struct {
		value ActiveResponseLedgerResult
		err   error
	}
	gate := make(chan struct{})
	outcomes := make(chan result, distinctActions)
	for _, input := range inputs {
		go func(v TrustedShadowLedgerInputs) {
			<-gate
			record, err := RecordVerifiedShadowLedger(ctx, db, v, now)
			outcomes <- result{value: record, err: err}
		}(input)
	}
	close(gate)
	for i := 0; i < distinctActions; i++ {
		outcome := <-outcomes
		if outcome.err != nil || outcome.value.Status != ActiveResponseLedgerRecorded ||
			outcome.value.ExecutionPerformed {
			t.Fatalf("parallel shadow append was not safely serialized: %+v, %v", outcome.value, outcome.err)
		}
	}

	// The first caller records one action; concurrent identical callers must
	// return only duplicate outcomes without appending extra audit rows.
	duplicate := base
	duplicate.Request = base.Request
	duplicate.Request.ActionID = "replayed-concurrent-action"
	resignShadowLedgerFixture(t, &duplicate, now)
	const duplicateWriters = 4
	gate = make(chan struct{})
	duplicates := make(chan result, duplicateWriters)
	for i := 0; i < duplicateWriters; i++ {
		go func() {
			<-gate
			record, err := RecordVerifiedShadowLedger(ctx, db, duplicate, now)
			duplicates <- result{value: record, err: err}
		}()
	}
	close(gate)
	recorded, repeated := 0, 0
	for i := 0; i < duplicateWriters; i++ {
		outcome := <-duplicates
		if outcome.err != nil || outcome.value.ExecutionPerformed {
			t.Fatalf("concurrent duplicate rejected with unsafe outcome: %+v, %v", outcome.value, outcome.err)
		}
		switch outcome.value.Status {
		case ActiveResponseLedgerRecorded:
			recorded++
		case ActiveResponseLedgerDuplicate:
			repeated++
		default:
			t.Fatalf("unexpected duplicate status: %q", outcome.value.Status)
		}
	}
	if recorded != 1 || repeated != duplicateWriters-1 {
		t.Fatalf("incorrect duplicate write split: first=%d repeats=%d", recorded, repeated)
	}

	rows, err := db.QueryContext(ctx,
		`SELECT p.action_id, p.target_id, p.provider_id, p.plan_digest,
		        a.previous_hash, a.chain_hash, a.recorded_at
		   FROM koschei_active_response_shadow_audit a
		   JOIN koschei_active_response_shadow_plans p
		     ON a.tenant_id=p.tenant_id AND a.action_id=p.action_id
		  WHERE a.tenant_id=$1 ORDER BY a.sequence_id`, tenant)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var entries []ActiveResponseLedgerEntry
	for rows.Next() {
		entry := ActiveResponseLedgerEntry{SchemaVersion: ActiveResponseLedgerSchemaVersion, TenantID: tenant}
		if err := rows.Scan(&entry.ActionID, &entry.TargetID, &entry.ProviderID,
			&entry.PlanDigest, &entry.PreviousHash, &entry.ChainHash, &entry.RecordedAt); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(entries) != distinctActions+1 {
		t.Fatalf("expected %d append-only audit events; found %d", distinctActions+1, len(entries))
	}
	root := "sha256:" + fmt.Sprintf("%064d", 0)
	check, err := VerifyShadowLedgerSnapshot(tenant, entries, root, entries[len(entries)-1].ChainHash)
	if err != nil || check.Status != "verified" || check.RecordsVerified != len(entries) ||
		check.ExecutionPerformed {
		t.Fatalf("parallel transaction chain does not verify: %+v %v", check, err)
	}
}
