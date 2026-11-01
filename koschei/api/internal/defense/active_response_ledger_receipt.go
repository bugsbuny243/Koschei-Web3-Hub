package defense

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// These are read-only observations. An absent result is deliberately not a
// grant to retry a write, and no result proves a provider action executed.
const (
	ActiveResponseReceiptPresent = "shadow_receipt_consistent"
	ActiveResponseReceiptAbsent  = "shadow_receipt_absent_unconfirmed"
	ActiveResponseReceiptUnknown = "shadow_receipt_unverified"
)

type ActiveResponseShadowReceipt struct {
	Status             string `json:"status"`
	TenantID           string `json:"tenant_id"`
	ActionID           string `json:"action_id"`
	PlanDigest         string `json:"plan_digest"`
	ChainHash          string `json:"chain_hash,omitempty"`
	ExecutionPerformed bool   `json:"execution_performed"`
}

// InspectShadowLedgerReceipt performs one read-only PostgreSQL statement on
// the configured database. The caller must authenticate the tenant and
// source the expected provider/target/digest from its own trusted records.
// Only an authoritative PRIMARY database read can meaningfully reconcile
// an ambiguous commit; a missing row may still be unconfirmed (e.g. if the
// database is stale or unavailable), and NEVER authorizes automatic replay.
func InspectShadowLedgerReceipt(
	ctx context.Context,
	db *sql.DB,
	tenantID, actionID, targetID, providerID, expectedDigest string,
) (ActiveResponseShadowReceipt, error) {
	out := ActiveResponseShadowReceipt{
		Status:     ActiveResponseReceiptUnknown,
		TenantID:   tenantID,
		ActionID:   actionID,
		PlanDigest: expectedDigest,
	}
	if !validShadowLedgerID(tenantID) || !validShadowLedgerID(actionID) ||
		!validShadowLedgerID(targetID) || !validShadowLedgerID(providerID) ||
		!validActiveResponseSHA256(expectedDigest) {
		return out, errors.New("invalid trusted receipt scope or expected plan digest")
	}
	if db == nil {
		return out, errors.New("shadow receipt database unavailable")
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	var savedTarget, savedProvider, savedDigest, planChain sql.NullString
	var previousHash, auditChain sql.NullString
	var recordedAt sql.NullTime
	err := db.QueryRowContext(ctx, `
		SELECT p.target_id, p.provider_id, p.plan_digest, p.chain_hash,
		       a.previous_hash, a.chain_hash, a.recorded_at
		  FROM koschei_active_response_shadow_plans AS p
		  FULL OUTER JOIN koschei_active_response_shadow_audit AS a
		    ON a.tenant_id = p.tenant_id AND a.action_id = p.action_id
		 WHERE COALESCE(p.tenant_id, a.tenant_id) = $1
		   AND COALESCE(p.action_id, a.action_id) = $2
	`, tenantID, actionID).Scan(
		&savedTarget, &savedProvider, &savedDigest, &planChain,
		&previousHash, &auditChain, &recordedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		out.Status = ActiveResponseReceiptAbsent
		return out, nil
	}
	if err != nil {
		return out, fmt.Errorf("read shadow audit receipt: %w", err)
	}
	if !savedTarget.Valid || !savedProvider.Valid || !savedDigest.Valid ||
		!planChain.Valid || !previousHash.Valid || !auditChain.Valid ||
		!recordedAt.Valid ||
		savedTarget.String != targetID ||
		savedProvider.String != providerID ||
		savedDigest.String != expectedDigest ||
		planChain.String != auditChain.String ||
		!validActiveResponseSHA256(auditChain.String) ||
		!validActiveResponseSHA256(previousHash.String) ||
		recordedAt.Time.IsZero() {
		return out, errors.New("shadow receipt incomplete, conflicting or malformed")
	}
	entry := ActiveResponseLedgerEntry{
		SchemaVersion: ActiveResponseLedgerSchemaVersion,
		TenantID:      tenantID,
		ActionID:      actionID,
		TargetID:      savedTarget.String,
		ProviderID:    savedProvider.String,
		PlanDigest:    savedDigest.String,
		PreviousHash:  previousHash.String,
		ChainHash:     auditChain.String,
		RecordedAt:    recordedAt.Time.UTC().Truncate(time.Microsecond),
	}
	calculated, err := shadowLedgerEntryHash(entry)
	if err != nil || calculated != entry.ChainHash {
		return out, errors.New("shadow receipt chain digest mismatch")
	}
	out.Status = ActiveResponseReceiptPresent
	out.ChainHash = entry.ChainHash
	return out, nil
}
