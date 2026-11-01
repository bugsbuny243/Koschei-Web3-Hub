package defense

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// This is an opt-in destructive-to-test-data acceptance test for a dedicated
// disposable PostgreSQL database named exactly koschei_shadow_test. The SQL
// schema must be preinstalled manually. No production database is accepted.
func TestActiveResponsePostgresStagingLedgerAcceptance(t *testing.T) {
	dsn := os.Getenv("KOSCHEI_ACTIVE_RESPONSE_STAGING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("dedicated staging PostgreSQL acceptance requires explicit database URL")
	}
	if err := validateIsolatedShadowPostgresDSN(dsn); err != nil {
		t.Fatalf("refusing unsafe shadow staging database DSN: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var dbName string
	if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&dbName); err != nil {
		t.Fatal(err)
	}
	if dbName != "koschei_shadow_test" {
		t.Fatalf("refusing to run shadow ledger acceptance on database %q", dbName)
	}
	for _, table := range []string{
		"koschei_active_response_shadow_plans",
		"koschei_active_response_shadow_audit",
	} {
		var exists sql.NullString
		if err := db.QueryRowContext(ctx, "SELECT to_regclass($1)::text", "public."+table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists.Valid {
			t.Fatalf("staging schema not installed: missing %s", table)
		}
	}

	input, now := trustedShadowLedgerFixture(t)
	tenant := fmt.Sprintf("shadow-acceptance-%d", time.Now().UnixNano())
	input.Policy.TenantID = tenant
	input.Request.TenantID = tenant
	input.Asset.TenantID = tenant
	input.Provider.TenantID = tenant
	input.Recovery.TenantID = tenant
	resignShadowLedgerFixture(t, &input, now)

	first, err := RecordVerifiedShadowLedger(ctx, db, input, now)
	if err != nil || first.Status != ActiveResponseLedgerRecorded || first.ExecutionPerformed {
		t.Fatalf("failed first shadow append: %+v %v", first, err)
	}
	again, err := RecordVerifiedShadowLedger(ctx, db, input, now)
	if err != nil || again.Status != ActiveResponseLedgerDuplicate ||
		again.ChainHash != first.ChainHash || again.ExecutionPerformed {
		t.Fatalf("idempotent shadow replay was not detected: %+v %v", again, err)
	}

	// The action ID must not be reused for a different reviewed payload.
	collision := input
	collision.Request = input.Request
	collision.Request.Parameters = map[string]string{"different_plan": "yes"}
	resignShadowLedgerFixture(t, &collision, now)
	if _, err := RecordVerifiedShadowLedger(ctx, db, collision, now); err == nil {
		t.Fatal("different digest accepted under existing tenant/action ID")
	}

	var countPlans, countAudit int
	if err := db.QueryRowContext(ctx,
		"SELECT count(*) FROM koschei_active_response_shadow_plans WHERE tenant_id=$1 AND action_id=$2",
		tenant, input.Request.ActionID).Scan(&countPlans); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx,
		"SELECT count(*) FROM koschei_active_response_shadow_audit WHERE tenant_id=$1 AND action_id=$2",
		tenant, input.Request.ActionID).Scan(&countAudit); err != nil {
		t.Fatal(err)
	}
	if countPlans != 1 || countAudit != 1 {
		t.Fatalf("duplicate shadow audit rows: plans=%d, audit=%d", countPlans, countAudit)
	}
	// The manually provisioned staging trigger must prohibit modifications,
	// including a no-op UPDATE and a DELETE of an existing audit row.
	if _, err := db.ExecContext(ctx,
		"UPDATE koschei_active_response_shadow_audit SET chain_hash=chain_hash WHERE tenant_id=$1 AND action_id=$2",
		tenant, input.Request.ActionID); err == nil {
		t.Fatal("append-only audit row accepted an UPDATE")
	}
	if _, err := db.ExecContext(ctx,
		"DELETE FROM koschei_active_response_shadow_audit WHERE tenant_id=$1 AND action_id=$2",
		tenant, input.Request.ActionID); err == nil {
		t.Fatal("append-only audit row accepted a DELETE")
	}

	var entry ActiveResponseLedgerEntry
	entry.SchemaVersion = ActiveResponseLedgerSchemaVersion
	entry.TenantID = tenant
	row := db.QueryRowContext(ctx,
		`SELECT p.action_id,p.target_id,p.provider_id,p.plan_digest,a.previous_hash,
			a.chain_hash,a.recorded_at
		  FROM koschei_active_response_shadow_plans p
		  JOIN koschei_active_response_shadow_audit a
		    ON a.tenant_id=p.tenant_id AND a.action_id=p.action_id
		  WHERE p.tenant_id=$1 AND p.action_id=$2`, tenant, input.Request.ActionID)
	if err := row.Scan(&entry.ActionID, &entry.TargetID, &entry.ProviderID,
		&entry.PlanDigest, &entry.PreviousHash, &entry.ChainHash, &entry.RecordedAt); err != nil {
		t.Fatal(err)
	}
	check, err := VerifyShadowLedgerSnapshot(tenant, []ActiveResponseLedgerEntry{entry}, entry.PreviousHash, first.ChainHash)
	if err != nil || check.Status != "verified" || check.ExecutionPerformed {
		t.Fatalf("persisted shadow ledger chain invalid: %+v %v", check, err)
	}
	receipt, err := InspectShadowLedgerReceipt(ctx, db,
		tenant, input.Request.ActionID, input.Request.TargetID,
		input.Provider.ProviderID, first.PlanDigest)
	if err != nil || receipt.Status != ActiveResponseReceiptPresent ||
		receipt.ChainHash != first.ChainHash || receipt.ExecutionPerformed {
		t.Fatalf("consistent committed shadow receipt not found: %+v %v", receipt, err)
	}
	absent, err := InspectShadowLedgerReceipt(ctx, db,
		tenant, "missing-shadow-action", input.Request.TargetID,
		input.Provider.ProviderID, first.PlanDigest)
	if err != nil || absent.Status != ActiveResponseReceiptAbsent || absent.ExecutionPerformed {
		t.Fatalf("absent shadow record must remain unconfirmed: %+v %v", absent, err)
	}
	conflict, err := InspectShadowLedgerReceipt(ctx, db,
		tenant, input.Request.ActionID, input.Request.TargetID,
		"wrong-provider", first.PlanDigest)
	if err == nil || conflict.Status != ActiveResponseReceiptUnknown {
		t.Fatalf("conflicting shadow receipt accepted: %+v %v", conflict, err)
	}

	// An independently signed genesis-to-tip checkpoint must agree with the
	// complete, repeatable-read PostgreSQL tenant snapshot.
	priv := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{79}, ed25519.SeedSize))
	cp := ActiveResponseSignedCheckpoint{
		SchemaVersion: ActiveResponseCheckpointSchemaVersion,
		TenantID:      tenant,
		StartHash:     entry.PreviousHash,
		EndHash:       first.ChainHash,
		RecordCount:   1,
		SignerID:      "staging-audit-archive",
		IssuedAt:      now,
	}
	payload, err := ActiveResponseCheckpointSigningPayload(cp)
	if err != nil {
		t.Fatal(err)
	}
	cp.Signature = ed25519.Sign(priv, payload)
	keys := map[string]ed25519.PublicKey{
		cp.SignerID: priv.Public().(ed25519.PublicKey),
	}
	verified, err := VerifyCurrentShadowLedgerCheckpoint(ctx, db, cp, keys, now.Add(time.Minute))
	if err != nil || verified.Status != "verified" || verified.RecordsVerified != 1 ||
		verified.ExecutionPerformed {
		t.Fatalf("database checkpoint verification failed: %+v %v", verified, err)
	}
	badCheckpoint := cp
	badCheckpoint.EndHash = cp.StartHash
	if result, err := VerifyCurrentShadowLedgerCheckpoint(ctx, db, badCheckpoint, keys, now.Add(time.Minute)); err == nil ||
		result.Status != "rejected" {
		t.Fatalf("tampered signed checkpoint accepted: %+v %v", result, err)
	}

	// A signed historical one-row checkpoint must not pass as the CURRENT
	// full tenant ledger after an additional separately reviewed append.
	second := input
	second.Request = input.Request
	second.Request.ActionID = input.Request.ActionID + "-second"
	resignShadowLedgerFixture(t, &second, now)
	recordTwo, err := RecordVerifiedShadowLedger(ctx, db, second, now)
	if err != nil || recordTwo.Status != ActiveResponseLedgerRecorded {
		t.Fatalf("second shadow append failed: %+v %v", recordTwo, err)
	}
	if result, err := VerifyCurrentShadowLedgerCheckpoint(ctx, db, cp, keys, now.Add(time.Minute)); err == nil ||
		result.Status != "rejected" {
		t.Fatalf("stale signed ledger head accepted: %+v %v", result, err)
	}

	// Independently retained, previously authenticated high-water state
	// prevents rollback or a re-signed fork of the first observed event.
	fence := TrustedShadowLedgerHighWatermark{
		TenantID:        tenant,
		RecordsVerified: 1,
		ChainHash:       first.ChainHash,
		IssuedAt:        cp.IssuedAt,
	}
	advancedCP := cp
	advancedCP.RecordCount = 2
	advancedCP.EndHash = recordTwo.ChainHash
	advancedCP.IssuedAt = cp.IssuedAt.Add(time.Second)
	signed, err := ActiveResponseCheckpointSigningPayload(advancedCP)
	if err != nil {
		t.Fatal(err)
	}
	advancedCP.Signature = ed25519.Sign(priv, signed)
	advanced, err := VerifyCurrentShadowLedgerCheckpointWithHighWatermark(
		ctx, db, advancedCP, keys, fence, now.Add(time.Minute),
	)
	if err != nil || advanced.Status != "verified" ||
		advanced.RecordsVerified != 2 || advanced.ExecutionPerformed {
		t.Fatalf("independently fenced ledger advancement failed: %+v %v", advanced, err)
	}
	forkedFence := fence
	forkedFence.ChainHash = "sha256:" + fmt.Sprintf("%064x", 1)
	forked, err := VerifyCurrentShadowLedgerCheckpointWithHighWatermark(
		ctx, db, advancedCP, keys, forkedFence, now.Add(time.Minute),
	)
	if err == nil || forked.Status != "rejected" {
		t.Fatalf("signed ledger fork accepted against external high-water mark: %+v %v", forked, err)
	}
	rolledBack := TrustedShadowLedgerHighWatermark{
		TenantID:        tenant,
		RecordsVerified: 2,
		ChainHash:       advancedCP.EndHash,
		IssuedAt:        advancedCP.IssuedAt,
	}
	rollback, err := VerifyCurrentShadowLedgerCheckpointWithHighWatermark(
		ctx, db, cp, keys, rolledBack, now.Add(time.Minute),
	)
	if err == nil || rollback.Status != "rejected" {
		t.Fatalf("older valid signature accepted despite higher trusted checkpoint: %+v %v", rollback, err)
	}
}

func resignShadowLedgerFixture(t *testing.T, input *TrustedShadowLedgerInputs, now time.Time) {
	t.Helper()
	plan, err := ProposeActiveResponse(
		input.Policy, input.Finding, input.Request, input.KnownEvidence, input.Asset,
	)
	if err != nil || plan.Status != ActiveResponseShadowProposed {
		t.Fatalf("cannot rebind staging fixture: %+v %v", plan, err)
	}
	digest, err := ActiveResponsePlanDigest(
		input.Policy, input.Finding, input.Request, input.Asset, plan,
	)
	if err != nil {
		t.Fatal(err)
	}
	one, keyOne := signActiveResponseApproval(t, 51, "staging-reviewer-one", digest, now)
	two, keyTwo := signActiveResponseApproval(t, 52, "staging-reviewer-two", digest, now)
	input.Approvals = []ActiveResponseApproval{one, two}
	input.TrustedApprovers = map[string]ed25519.PublicKey{
		"staging-reviewer-one": keyOne,
		"staging-reviewer-two": keyTwo,
	}
}
