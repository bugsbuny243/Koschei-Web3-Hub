package services

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// UnifiedRadarSignedReceiptIdentity is returned only after independent Ed25519
// verification of exact immutable canonical bytes against a trusted key.
type UnifiedRadarSignedReceiptIdentity struct {
	Network string
	Target  string
}

// VerifyUnifiedRadarSignedReceipt verifies historical canonical payload bytes.
// Public keys come ONLY from a separately configured operator trust registry.
func VerifyUnifiedRadarSignedReceipt(payload []byte, signature, payloadHash, keyID string, registry GlobalRadarTrustedVerdictRegistry) (UnifiedRadarSignedReceiptIdentity, error) {
	var signed unifiedVerdictSigningPayloadV1
	if err := json.Unmarshal(payload, &signed); err != nil {
		return UnifiedRadarSignedReceiptIdentity{}, fmt.Errorf("decode signed receipt: %w", err)
	}
	if signed.SchemaVersion != UnifiedVerdictSignatureDomainV1 ||
		strings.TrimSpace(signed.Target) == "" || strings.TrimSpace(signed.Network) == "" ||
		strings.TrimSpace(signed.Verdict) == "" || strings.TrimSpace(signed.RuleVersion) == "" ||
		len(signed.Evidence) == 0 {
		return UnifiedRadarSignedReceiptIdentity{}, errors.New("signed receipt has invalid contract fields")
	}
	canonical, err := json.Marshal(signed)
	if err != nil || !bytes.Equal(canonical, payload) {
		return UnifiedRadarSignedReceiptIdentity{}, errors.New("signed receipt is not the exact canonical byte sequence")
	}
	at, err := time.Parse(time.RFC3339Nano, signed.CreatedAt)
	if err != nil {
		return UnifiedRadarSignedReceiptIdentity{}, fmt.Errorf("invalid signed receipt timestamp: %w", err)
	}
	sum := sha256.Sum256(payload)
	expectedHash := "sha256:" + hex.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(payloadHash), []byte(expectedHash)) != 1 {
		return UnifiedRadarSignedReceiptIdentity{}, errors.New("signed receipt payload hash mismatch")
	}
	publicKey, err := registry.Resolve(keyID, at)
	if err != nil {
		return UnifiedRadarSignedReceiptIdentity{}, err
	}
	decoded, err := decodeCanonicalGlobalRadarEd25519(signature, ed25519.SignatureSize)
	if err != nil {
		return UnifiedRadarSignedReceiptIdentity{}, err
	}
	if !ed25519.Verify(publicKey, payload, decoded) {
		return UnifiedRadarSignedReceiptIdentity{}, errors.New("historical verdict Ed25519 verification failed")
	}
	return UnifiedRadarSignedReceiptIdentity{Network: signed.Network, Target: signed.Target}, nil
}

// SOCUnifiedVerdictTrustRegistry uses the existing out-of-band ARVIS public-key
// registry, including expired historical keys valid at the original signed_at.
// It MUST NOT infer trust from keys or metadata stored in the receipt table.
func SOCUnifiedVerdictTrustRegistry() (GlobalRadarTrustedVerdictRegistry, error) {
	registry, configured, err := arvisStreamRecoveryTrustRegistry()
	if err != nil {
		return GlobalRadarTrustedVerdictRegistry{}, err
	}
	if !configured {
		return GlobalRadarTrustedVerdictRegistry{}, errors.New("SOC signed verdict public-key trust registry is not configured")
	}
	return registry, nil
}

// insertUnifiedSignedReceiptTx stores each distinct authenticated signature
// exactly once; a fingerprint rescan is allowed to update only the projection.
// The caller commits or rolls back both projection and immutable receipt.
func insertUnifiedSignedReceiptTx(ctx context.Context, tx *sql.Tx, record UnifiedRadarVerdictHistoryRecord, payload []byte) (string, error) {
	if tx == nil || !record.Signed || len(payload) == 0 {
		return "", errors.New("immutable signed receipt requires transaction and canonical payload")
	}
	var receiptID string
	err := tx.QueryRowContext(ctx, `
        INSERT INTO security_unified_verdict_receipts (
            verdict_id,network,target_kind,target_id,signature,signature_algorithm,
            key_id,payload_hash,signed_payload
        ) VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9)
        ON CONFLICT (signature) DO NOTHING
        RETURNING id::text
    `, record.ID, record.Network, record.TargetKind, record.TargetID, record.Signature,
		record.SignatureAlgorithm, record.KeyID, record.PayloadHash, payload).Scan(&receiptID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("append signed verdict receipt: %w", err)
	}
	// A duplicate signature is idempotent ONLY if every authenticated byte and
	// subject still matches. A distinct payload under that signature fails closed.
	var storedPayload []byte
	var storedNetwork, storedTarget, storedKind, storedKey, storedHash string
	err = tx.QueryRowContext(ctx, `
        SELECT id::text,network,target_kind,target_id,key_id,payload_hash,signed_payload
        FROM security_unified_verdict_receipts
        WHERE signature=$1
    `, record.Signature).Scan(&receiptID, &storedNetwork, &storedKind, &storedTarget,
		&storedKey, &storedHash, &storedPayload)
	if err != nil {
		return "", fmt.Errorf("read immutable signed receipt: %w", err)
	}
	if storedNetwork != record.Network || storedKind != record.TargetKind ||
		storedTarget != record.TargetID || storedKey != record.KeyID ||
		storedHash != record.PayloadHash || !bytes.Equal(storedPayload, payload) {
		return "", errors.New("signed verdict receipt identity collision")
	}
	return receiptID, nil
}
