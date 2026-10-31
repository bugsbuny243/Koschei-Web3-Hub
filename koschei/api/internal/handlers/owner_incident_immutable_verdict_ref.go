package handlers

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"koschei/api/internal/services"
)

const immutableSOCVerdictRefPrefix = "unified-verdict-ledger:"
const immutableSOCVerdictRefSource = "security_unified_verdict_receipts:ed25519_trusted_registry_verified"

// resolveImmutableSOCVerdictReference binds an immutable, signed decision to
// the exact case subject. Unlike legacy mutable fingerprints, a later rescan
// cannot overwrite the signed canonical bytes. Missing trust fails closed.
func resolveImmutableSOCVerdictReference(ctx context.Context, tx *sql.Tx, ref, network, target string) (string, error) {
	if tx == nil || strings.TrimSpace(network) == "" || strings.TrimSpace(target) == "" {
		return "", nil
	}
	raw := strings.TrimPrefix(ref, immutableSOCVerdictRefPrefix)
	uuid, signature, ok := strings.Cut(raw, ":")
	if !ok || !unifiedSOCVerdictRefUUID.MatchString(uuid) ||
		!unifiedSOCVerdictRefSignature.MatchString(signature) {
		return "", nil
	}
	var payload []byte
	var storedSignature, keyID, payloadHash string
	err := tx.QueryRowContext(ctx, `
        SELECT signed_payload,signature,key_id,payload_hash
        FROM security_unified_verdict_receipts
        WHERE id=$1::uuid AND network=$2 AND target_kind='token' AND target_id=$3
          AND signature=$4 AND signature_algorithm='ed25519'
    `, uuid, network, target, signature).Scan(&payload, &storedSignature, &keyID, &payloadHash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	registry, err := services.SOCUnifiedVerdictTrustRegistry()
	if err != nil {
		// Missing or malformed independently configured trust material is not a
		// valid evidence receipt; never silently fall back to stored metadata.
		return "", nil
	}
	identity, err := services.VerifyUnifiedRadarSignedReceipt(payload, storedSignature, payloadHash, keyID, registry)
	if err != nil || identity.Network != network || identity.Target != target {
		return "", nil
	}
	return immutableSOCVerdictRefSource, nil
}
