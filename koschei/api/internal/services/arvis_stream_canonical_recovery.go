package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const arvisStreamTrustedKeysEnv = "KOSCHEI_ARVIS_STREAM_TRUSTED_VERDICT_KEYS_JSON"

// arvisStreamRecoveryTrustRegistry never infers trust from the verdict row,
// an arm signature, or an environment signing *private* key. The optional
// operator-controlled public-key registry is an explicit read-only trust root.
func arvisStreamRecoveryTrustRegistry() (GlobalRadarTrustedVerdictRegistry, bool, error) {
	raw := strings.TrimSpace(os.Getenv(arvisStreamTrustedKeysEnv))
	if raw == "" {
		return GlobalRadarTrustedVerdictRegistry{}, false, nil
	}
	var keys []GlobalRadarTrustedVerdictKey
	if err := json.Unmarshal([]byte(raw), &keys); err != nil {
		return GlobalRadarTrustedVerdictRegistry{}, true, fmt.Errorf("decode trusted ARVIS recovery keys: %w", err)
	}
	if len(keys) == 0 {
		return GlobalRadarTrustedVerdictRegistry{}, true, errors.New("ARVIS recovery trust registry must contain at least one public key")
	}
	registry, err := NewGlobalRadarTrustedVerdictRegistry(keys)
	if err != nil {
		return GlobalRadarTrustedVerdictRegistry{}, true, fmt.Errorf("invalid ARVIS recovery trust registry: %w", err)
	}
	return registry, true, nil
}

// reconcileArvisStreamCanonicalFinalTx verifies a signed, immutable receipt
// against its ORIGINAL source event and a configured trusted Ed25519 registry,
// then transitions a stale claimed stream job to completed atomically. It never
// inspects legacy arm signed flags. The caller owns commit/rollback.
func reconcileArvisStreamCanonicalFinalTx(
	ctx context.Context, tx *sql.Tx, eventID string, registry GlobalRadarTrustedVerdictRegistry,
) (bool, error) {
	if tx == nil || strings.TrimSpace(eventID) == "" {
		return false, errors.New("ARVIS canonical recovery requires transaction and event id")
	}

	var event arvisStreamTarget
	var storedTarget, storedNetwork, storedSignature, storedModule, storedHash, storedKey string
	var claimTarget, claimSignature, targetType, status string
	var receipt []byte
	err := tx.QueryRowContext(ctx, `
		SELECT s.id::text, COALESCE(s.target,''), s.network,
		       COALESCE(s.signature,''), s.module_id, s.target_type,
		       p.status, p.target, p.signature,
		       f.target, f.network, f.source_signature, f.source_module,
		       f.payload_hash, f.key_id, f.verdict_payload
		FROM arvis_stream_processing p
		JOIN security_radar_stream_events s ON s.id=p.stream_event_id
		JOIN arvis_stream_canonical_finals f ON f.stream_event_id=p.stream_event_id
		WHERE p.stream_event_id=$1::uuid
		  AND p.status='processing'
		  AND p.updated_at < now()-interval '5 minutes'
		FOR UPDATE OF p,s
	`, eventID).Scan(
		&event.ID, &event.Target, &event.Network, &event.Signature, &event.ModuleID, &targetType,
		&status, &claimTarget, &claimSignature,
		&storedTarget, &storedNetwork, &storedSignature, &storedModule, &storedHash, &storedKey, &receipt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read stale ARVIS canonical receipt and source: %w", err)
	}
	if targetType != "token" || status != "processing" ||
		claimTarget != event.Target || claimSignature != event.Signature ||
		storedTarget != event.Target || storedNetwork != event.Network ||
		storedSignature != event.Signature || storedModule != event.ModuleID {
		return false, errors.New("ARVIS recovery canonical receipt source identity mismatch")
	}

	var envelope arvisStreamCanonicalFinalEnvelope
	if err := json.Unmarshal(receipt, &envelope); err != nil {
		return false, fmt.Errorf("decode stored ARVIS recovery receipt: %w", err)
	}
	verdict := UnifiedRadarVerdict(envelope.Verdict)
	if envelope.Target != event.Target || envelope.Network != event.Network ||
		envelope.PayloadHash != storedHash || envelope.KeyID != storedKey ||
		envelope.SignatureAlgorithm != "ed25519" ||
		verdict.Target != envelope.Target || verdict.Network != envelope.Network ||
		verdict.Digest != envelope.Digest || verdict.PayloadHash != envelope.PayloadHash ||
		verdict.KeyID != envelope.KeyID || verdict.SignatureAlgorithm != envelope.SignatureAlgorithm {
		return false, errors.New("ARVIS recovery canonical signed envelope binding mismatch")
	}
	if err := validateArvisStreamCanonicalVerdict(event, verdict, registry); err != nil {
		return false, fmt.Errorf("ARVIS recovery canonical verdict authentication failed: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE arvis_stream_processing
		SET status='completed',last_error='',processed_at=now(),updated_at=now()
		WHERE stream_event_id=$1::uuid
		  AND status='processing'
		  AND updated_at < now()-interval '5 minutes'
	`, event.ID)
	if err != nil {
		return false, fmt.Errorf("complete authenticated ARVIS stale lease: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return false, errors.New("ARVIS canonical recovery could not transition the stale lease")
	}
	return true, nil
}

func reconcileArvisStreamCanonicalFinal(ctx context.Context, db *sql.DB, eventID string, registry GlobalRadarTrustedVerdictRegistry) (bool, error) {
	if db == nil {
		return false, errors.New("ARVIS canonical recovery database is unavailable")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	ok, err := reconcileArvisStreamCanonicalFinalTx(ctx, tx, eventID, registry)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit authenticated ARVIS stale lease recovery: %w", err)
	}
	return ok, nil
}

// arvisReconcileStaleCanonicalReceipts visits at most 100 eligible receipts
// per recovery pass. Untrusted or mismatched receipts are NEVER completed;
// the existing bounded-failure/exhaustion lifecycle handles them instead.
func arvisReconcileStaleCanonicalReceipts(ctx context.Context, db *sql.DB, registry GlobalRadarTrustedVerdictRegistry) {
	rows, err := db.QueryContext(ctx, `
		SELECT p.stream_event_id::text
		FROM arvis_stream_processing p
		JOIN arvis_stream_canonical_finals f ON f.stream_event_id=p.stream_event_id
		WHERE p.status='processing' AND p.updated_at < now()-interval '5 minutes'
		ORDER BY p.updated_at ASC
		LIMIT 100
	`)
	if err != nil {
		if !isUndefinedTableError(err) {
			// Missing new schema on a partial deployment must not suggest success.
			// Legacy fallback below will still fail leases closed.
			return
		}
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		// Authentication errors are intentionally not included in user-visible
		// error strings: no unsafe receipt is promoted or mutated here.
		_, _ = reconcileArvisStreamCanonicalFinal(ctx, db, id, registry)
	}
}
