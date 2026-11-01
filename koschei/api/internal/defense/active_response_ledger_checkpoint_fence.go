package defense

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"errors"
	"time"
)

// TrustedShadowLedgerHighWatermark is a previously authenticated,
// INDEPENDENTLY retained, complete-genesis checkpoint summary. Never load it
// from the same untrusted database as the ledger being audited.
// It is an input to verification, not an automatically published checkpoint.
type TrustedShadowLedgerHighWatermark struct {
	TenantID        string    `json:"tenant_id"`
	RecordsVerified int       `json:"records_verified"`
	ChainHash       string    `json:"chain_hash"`
	IssuedAt        time.Time `json:"issued_at"`
}

func validateShadowLedgerHighWatermark(
	checkpoint ActiveResponseSignedCheckpoint,
	fence TrustedShadowLedgerHighWatermark,
	now time.Time,
) error {
	if !validShadowLedgerID(fence.TenantID) ||
		fence.TenantID != checkpoint.TenantID ||
		fence.RecordsVerified < 1 ||
		fence.RecordsVerified > 100000 ||
		!validActiveResponseSHA256(fence.ChainHash) ||
		fence.IssuedAt.IsZero() ||
		fence.IssuedAt.After(now) {
		return errors.New("invalid or cross-tenant independently trusted high-water mark")
	}
	if checkpoint.RecordCount < fence.RecordsVerified ||
		checkpoint.IssuedAt.Before(fence.IssuedAt) {
		return errors.New("checkpoint would roll back accepted record count or issuance time")
	}
	if checkpoint.RecordCount == fence.RecordsVerified &&
		checkpoint.EndHash != fence.ChainHash {
		return errors.New("same-height checkpoint conflicts with independently trusted chain head")
	}
	if checkpoint.RecordCount > fence.RecordsVerified &&
		!checkpoint.IssuedAt.After(fence.IssuedAt) {
		return errors.New("new records require strictly later checkpoint issuance")
	}
	return nil
}

// VerifyCurrentShadowLedgerCheckpointWithHighWatermark adds anti-rollback
// anchoring to a full, signed, read-only tenant ledger verification.
// A previously accepted external high-water hash must occur at the SAME
// position in the new full chain, even if the new signer is independently
// trusted. This cannot protect against compromise of the external archive
// or signer trust registry. The caller must never synthesize the fence from
// the database being checked.
func VerifyCurrentShadowLedgerCheckpointWithHighWatermark(
	ctx context.Context,
	db *sql.DB,
	checkpoint ActiveResponseSignedCheckpoint,
	trustedSigners map[string]ed25519.PublicKey,
	fence TrustedShadowLedgerHighWatermark,
	now time.Time,
) (ActiveResponseLedgerIntegrity, error) {
	return verifyCurrentShadowLedgerCheckpoint(ctx, db, checkpoint, trustedSigners, &fence, now)
}
