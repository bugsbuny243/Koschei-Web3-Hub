package defense

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// VerifyCurrentShadowLedgerCheckpoint validates an independently signed,
// full-from-genesis checkpoint against a single PostgreSQL read-only,
// repeatable-read snapshot. It does not issue a ledger write, execution
// permission, provider command, or external effect assertion.
//
// Checkpoints for historical prefixes deliberately fail if more audit events
// are present in the snapshot. Callers must use the authoritative primary
// and protect the signed checkpoint and trusted signer registry independently.
func VerifyCurrentShadowLedgerCheckpoint(
	ctx context.Context,
	db *sql.DB,
	checkpoint ActiveResponseSignedCheckpoint,
	trustedSigners map[string]ed25519.PublicKey,
	now time.Time,
) (ActiveResponseLedgerIntegrity, error) {
	return verifyCurrentShadowLedgerCheckpoint(ctx, db, checkpoint, trustedSigners, nil, now)
}

// verifyCurrentShadowLedgerCheckpoint keeps optional independent high-water
// verification inside the SAME read-only MVCC transaction as all hash checks.
func verifyCurrentShadowLedgerCheckpoint(
	ctx context.Context,
	db *sql.DB,
	checkpoint ActiveResponseSignedCheckpoint,
	trustedSigners map[string]ed25519.PublicKey,
	fence *TrustedShadowLedgerHighWatermark,
	now time.Time,
) (ActiveResponseLedgerIntegrity, error) {
	rejected := ActiveResponseLedgerIntegrity{
		SchemaVersion:      ActiveResponseLedgerIntegritySchemaVersion,
		TenantID:           checkpoint.TenantID,
		CheckpointRoot:     checkpoint.StartHash,
		CheckpointHead:     checkpoint.EndHash,
		Status:             "rejected",
		ExecutionPerformed: false,
	}
	if err := ctx.Err(); err != nil {
		return rejected, err
	}
	if now.IsZero() || checkpoint.IssuedAt.After(now) {
		return rejected, errors.New("trusted checkpoint verification clock is required")
	}
	if checkpoint.StartHash != "sha256:"+strings.Repeat("0", 64) {
		return rejected, errors.New("full shadow ledger verification requires genesis checkpoint")
	}
	payload, err := ActiveResponseCheckpointSigningPayload(checkpoint)
	if err != nil {
		return rejected, err
	}
	publicKey, exists := trustedSigners[checkpoint.SignerID]
	if !exists || len(publicKey) != ed25519.PublicKeySize ||
		len(checkpoint.Signature) != ed25519.SignatureSize ||
		!ed25519.Verify(publicKey, payload, checkpoint.Signature) {
		return rejected, errors.New("shadow ledger checkpoint signature not trusted")
	}
	if fence != nil {
		if err := validateShadowLedgerHighWatermark(checkpoint, *fence, now); err != nil {
			return rejected, fmt.Errorf("trusted shadow high-water mark rejected: %w", err)
		}
	}
	if db == nil {
		return rejected, errors.New("shadow checkpoint database unavailable")
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	})
	if err != nil {
		return rejected, fmt.Errorf("begin read-only checkpoint verification: %w", err)
	}
	defer tx.Rollback()

	// The FULL JOIN detects orphaned plan rows as well as orphaned audit
	// rows; LIMIT count+1 fails closed if this checkpoint is no longer the
	// full observed tenant history. All reads use one MVCC snapshot.
	rows, err := tx.QueryContext(ctx, `
		SELECT COALESCE(a.action_id, p.action_id),
		       p.target_id, p.provider_id, p.plan_digest, p.chain_hash, p.recorded_at,
		       a.plan_digest, a.previous_hash, a.chain_hash, a.recorded_at
		  FROM koschei_active_response_shadow_audit AS a
		  FULL OUTER JOIN koschei_active_response_shadow_plans AS p
		    ON p.tenant_id = a.tenant_id AND p.action_id = a.action_id
		 WHERE COALESCE(a.tenant_id, p.tenant_id) = $1
		 ORDER BY a.sequence_id ASC NULLS LAST
		 LIMIT $2`,
		checkpoint.TenantID, checkpoint.RecordCount+1)
	if err != nil {
		return rejected, fmt.Errorf("read shadow checkpoint snapshot: %w", err)
	}
	entries := make([]ActiveResponseLedgerEntry, 0, checkpoint.RecordCount)
	for rows.Next() {
		var actionID, targetID, providerID, planDigest, planChain sql.NullString
		var planRecordedAt sql.NullTime
		var auditDigest, previousHash, auditChain sql.NullString
		var auditRecordedAt sql.NullTime
		if err = rows.Scan(
			&actionID, &targetID, &providerID, &planDigest, &planChain, &planRecordedAt,
			&auditDigest, &previousHash, &auditChain, &auditRecordedAt,
		); err != nil {
			break
		}
		if !actionID.Valid || !targetID.Valid || !providerID.Valid ||
			!planDigest.Valid || !planChain.Valid || !planRecordedAt.Valid ||
			!auditDigest.Valid || !previousHash.Valid || !auditChain.Valid ||
			!auditRecordedAt.Valid || planDigest.String != auditDigest.String ||
			planChain.String != auditChain.String ||
			!planRecordedAt.Time.Equal(auditRecordedAt.Time) {
			err = errors.New("missing or mismatched shadow plan/audit row in checkpoint")
			break
		}
		entries = append(entries, ActiveResponseLedgerEntry{
			SchemaVersion: ActiveResponseLedgerSchemaVersion,
			TenantID:      checkpoint.TenantID,
			ActionID:      actionID.String,
			TargetID:      targetID.String,
			ProviderID:    providerID.String,
			PlanDigest:    planDigest.String,
			PreviousHash:  previousHash.String,
			ChainHash:     auditChain.String,
			RecordedAt:    auditRecordedAt.Time.UTC(),
		})
		if len(entries) > checkpoint.RecordCount {
			err = errors.New("signed checkpoint is not the current complete tenant ledger")
			break
		}
		if fence != nil && len(entries) == fence.RecordsVerified &&
			entries[len(entries)-1].ChainHash != fence.ChainHash {
			err = errors.New("trusted high-water chain anchor missing from checkpoint history")
			break
		}
	}
	if err == nil {
		err = rows.Err()
	}
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return rejected, fmt.Errorf("shadow ledger checkpoint snapshot rejected: %w", err)
	}
	if len(entries) != checkpoint.RecordCount {
		return rejected, errors.New("signed checkpoint record count differs from database snapshot")
	}
	verified, err := VerifySignedShadowLedgerCheckpoint(checkpoint, entries, trustedSigners, now)
	if err != nil {
		return rejected, fmt.Errorf("signed database checkpoint rejected: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return rejected, fmt.Errorf("finish read-only checkpoint verification: %w", err)
	}
	return verified, nil
}
