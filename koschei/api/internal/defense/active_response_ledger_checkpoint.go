package defense

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// A checkpoint is signed metadata for an offline, shadow-only audit range.
// The signer key must be retrieved from a separately controlled registry,
// and a copy of the checkpoint must be retained outside the audited database.
const ActiveResponseCheckpointSchemaVersion = "koschei-active-response-checkpoint-v1"

type ActiveResponseSignedCheckpoint struct {
	SchemaVersion string    `json:"schema_version"`
	TenantID      string    `json:"tenant_id"`
	StartHash     string    `json:"start_hash"`
	EndHash       string    `json:"end_hash"`
	RecordCount   int       `json:"record_count"`
	SignerID      string    `json:"signer_id"`
	IssuedAt      time.Time `json:"issued_at"`
	Signature     []byte    `json:"signature"`
}

// ActiveResponseCheckpointSigningPayload defines the exact signed bytes.
// No private key is stored or used here, and this function does not sign.
func ActiveResponseCheckpointSigningPayload(checkpoint ActiveResponseSignedCheckpoint) ([]byte, error) {
	if checkpoint.SchemaVersion != ActiveResponseCheckpointSchemaVersion ||
		!validShadowLedgerID(checkpoint.TenantID) ||
		!validShadowLedgerID(checkpoint.SignerID) ||
		!validActiveResponseSHA256(checkpoint.StartHash) ||
		!validActiveResponseSHA256(checkpoint.EndHash) ||
		checkpoint.RecordCount < 1 || checkpoint.RecordCount > 100000 ||
		checkpoint.IssuedAt.IsZero() {
		return nil, errors.New("invalid signed shadow checkpoint boundary")
	}
	payload := struct {
		SchemaVersion string    `json:"schema_version"`
		TenantID      string    `json:"tenant_id"`
		StartHash     string    `json:"start_hash"`
		EndHash       string    `json:"end_hash"`
		RecordCount   int       `json:"record_count"`
		SignerID      string    `json:"signer_id"`
		IssuedAt      time.Time `json:"issued_at"`
	}{
		checkpoint.SchemaVersion, checkpoint.TenantID, checkpoint.StartHash,
		checkpoint.EndHash, checkpoint.RecordCount, checkpoint.SignerID,
		checkpoint.IssuedAt.UTC(),
	}
	return json.Marshal(payload)
}

// VerifySignedShadowLedgerCheckpoint validates an externally retained and
// independently signed range descriptor, then checks every chain link.
// It does not fetch the signer registry or the entries, and cannot attest
// to omitted checkpoints, live actions or the state of external providers.
func VerifySignedShadowLedgerCheckpoint(
	checkpoint ActiveResponseSignedCheckpoint,
	entries []ActiveResponseLedgerEntry,
	trustedSigners map[string]ed25519.PublicKey,
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
	if now.IsZero() || checkpoint.IssuedAt.After(now) {
		return rejected, errors.New("trusted clock is required and checkpoint cannot be future-dated")
	}
	if len(entries) != checkpoint.RecordCount {
		return rejected, errors.New("checkpoint record count does not match supplied audit interval")
	}
	signedBytes, err := ActiveResponseCheckpointSigningPayload(checkpoint)
	if err != nil {
		return rejected, err
	}
	key, ok := trustedSigners[checkpoint.SignerID]
	if !ok || len(key) != ed25519.PublicKeySize ||
		len(checkpoint.Signature) != ed25519.SignatureSize ||
		!ed25519.Verify(key, signedBytes, checkpoint.Signature) {
		return rejected, errors.New("signed shadow checkpoint is not authenticated by the trusted registry")
	}
	verified, err := VerifyShadowLedgerSnapshot(
		checkpoint.TenantID, entries, checkpoint.StartHash, checkpoint.EndHash,
	)
	if err != nil {
		return rejected, fmt.Errorf("checkpoint audit range rejected: %w", err)
	}
	return verified, nil
}
