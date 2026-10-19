package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// arvisStreamRawUnifiedVerdict serializes the original signed decision without
// invoking UnifiedRadarVerdict.MarshalJSON, which recomputes the contract and
// can change authentication fields. Signing material must never be minted by
// the persistence layer.
type arvisStreamRawUnifiedVerdict UnifiedRadarVerdict

type arvisStreamCanonicalFinalEnvelope struct {
	KeyID              string                       `json:"key_id"`
	PayloadHash        string                       `json:"payload_hash"`
	SignatureAlgorithm string                       `json:"signature_algorithm"`
	Target             string                       `json:"target"`
	Network            string                       `json:"network"`
	Digest             string                       `json:"digest"`
	Verdict            arvisStreamRawUnifiedVerdict `json:"verdict"`
}

// persistArvisStreamCanonicalFinal persists a previously issued and independently
// authenticated v1.4 final, together with the source stream event's processing
// completion, in one PostgreSQL transaction. It does NOT derive rules, mint
// signatures, publish scores, or enable the currently withheld stream worker.
// The caller must use the canonical actor/behavior engine and a trusted key
// registry; evidence-arm compatibility finals are never accepted.
func persistArvisStreamCanonicalFinal(
	ctx context.Context, db *sql.DB, event arvisStreamTarget,
	verdict UnifiedRadarVerdict, registry GlobalRadarTrustedVerdictRegistry,
) (string, error) {
	if db == nil {
		return "", errors.New("ARVIS canonical final persistence database unavailable")
	}
	if err := validateArvisStreamCanonicalVerdict(event, verdict, registry); err != nil {
		return "", err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("start ARVIS canonical final transaction: %w", err)
	}
	defer tx.Rollback()
	if err := persistArvisStreamCanonicalFinalTx(ctx, tx, event, verdict, registry); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit ARVIS canonical final and processing status: %w", err)
	}
	return event.ID, nil
}

// persistArvisStreamCanonicalFinalTx is also used by PostgreSQL acceptance
// tests inside a rollback-only fixture transaction. No partial completion is
// allowed: callers of this helper must commit only after it returns nil.
func persistArvisStreamCanonicalFinalTx(
	ctx context.Context, tx *sql.Tx, event arvisStreamTarget,
	verdict UnifiedRadarVerdict, registry GlobalRadarTrustedVerdictRegistry,
) error {
	if tx == nil {
		return errors.New("ARVIS canonical final transaction unavailable")
	}
	if err := validateArvisStreamCanonicalVerdict(event, verdict, registry); err != nil {
		return err
	}

	// Lock the genuine raw ledger entry. The caller-provided event cannot
	// substitute another target, network or transaction at persistence time.
	var target, network, signature, moduleID, targetType string
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(target,''), network, COALESCE(signature,''), module_id, target_type
		FROM security_radar_stream_events WHERE id=$1::uuid FOR UPDATE
	`, event.ID).Scan(&target, &network, &signature, &moduleID, &targetType); err != nil {
		return fmt.Errorf("ARVIS source stream event unavailable: %w", err)
	}
	if target != event.Target || network != event.Network || signature != event.Signature ||
		moduleID != event.ModuleID || targetType != "token" {
		return errors.New("ARVIS canonical final source event identity mismatch")
	}

	var status, claimTarget, claimSignature string
	if err := tx.QueryRowContext(ctx, `
		SELECT status,target,signature FROM arvis_stream_processing
		WHERE stream_event_id=$1::uuid FOR UPDATE
	`, event.ID).Scan(&status, &claimTarget, &claimSignature); err != nil {
		return fmt.Errorf("ARVIS canonical final processing claim unavailable: %w", err)
	}
	if claimTarget != event.Target || claimSignature != event.Signature ||
		(status != "processing" && status != "completed") {
		return errors.New("ARVIS canonical final requires matching active or completed processing claim")
	}

	envelope := arvisStreamCanonicalFinalEnvelope{
		Target: event.Target, Network: event.Network,
		Digest: verdict.Digest, KeyID: verdict.KeyID, PayloadHash: verdict.PayloadHash, SignatureAlgorithm: verdict.SignatureAlgorithm,
		Verdict: arvisStreamRawUnifiedVerdict(verdict),
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("encode ARVIS canonical signed final: %w", err)
	}
	// Immutable event-unique snapshot. An existing row is acceptable ONLY if
	// it is exactly the same JSONB signed decision and event identity; retries
	// never overwrite a previous signed conclusion.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO arvis_stream_canonical_finals
		(stream_event_id,target,network,source_signature,source_module,verdict_payload,payload_hash,key_id)
		VALUES ($1::uuid,$2,$3,$4,$5,$6::jsonb,$7,$8)
		ON CONFLICT (stream_event_id) DO NOTHING
	`, event.ID, event.Target, event.Network, event.Signature, event.ModuleID,
		string(payload), verdict.PayloadHash, verdict.KeyID); err != nil {
		return fmt.Errorf("persist ARVIS canonical signed final: %w", err)
	}

	var readback []byte
	var persistedTarget, persistedNetwork, persistedSignature, persistedModule, persistedHash, persistedKey string
	if err := tx.QueryRowContext(ctx, `
		SELECT target,network,source_signature,source_module,verdict_payload,payload_hash,key_id
		FROM arvis_stream_canonical_finals WHERE stream_event_id=$1::uuid FOR UPDATE
	`, event.ID).Scan(&persistedTarget, &persistedNetwork, &persistedSignature, &persistedModule,
		&readback, &persistedHash, &persistedKey); err != nil {
		return fmt.Errorf("read back ARVIS canonical final: %w", err)
	}
	if persistedTarget != event.Target || persistedNetwork != event.Network ||
		persistedSignature != event.Signature || persistedModule != event.ModuleID ||
		persistedHash != verdict.PayloadHash || persistedKey != verdict.KeyID {
		return errors.New("ARVIS persisted canonical final identity or signer mismatch")
	}
	var stored arvisStreamCanonicalFinalEnvelope
	if err := json.Unmarshal(readback, &stored); err != nil {
		return fmt.Errorf("decode ARVIS stored canonical final: %w", err)
	}
	if stored.Target != event.Target || stored.Network != event.Network || stored.Digest != verdict.Digest ||
		stored.PayloadHash != persistedHash || stored.KeyID != persistedKey || stored.SignatureAlgorithm != "ed25519" {
		return errors.New("ARVIS stored canonical final binding differs from source")
	}
	// JSONB-normalized structural equality plus a fresh trusted Ed25519 and
	// verified source-citation check, not just a signed=true database flag.
	var expectedJSON, storedJSON any
	if err := json.Unmarshal(payload, &expectedJSON); err != nil {
		return err
	}
	if err := json.Unmarshal(readback, &storedJSON); err != nil {
		return err
	}
	// Comparing marshaled canonical JSON maps is safe against JSONB key order.
	expectedCanonical, _ := json.Marshal(expectedJSON)
	storedCanonical, _ := json.Marshal(storedJSON)
	if string(expectedCanonical) != string(storedCanonical) {
		return errors.New("ARVIS stream event already has a different immutable canonical final")
	}
	reconstructed := UnifiedRadarVerdict(stored.Verdict)
	reconstructed.Target = stored.Target
	reconstructed.Network = stored.Network
	reconstructed.Digest = stored.Digest
	reconstructed.SignatureAlgorithm = stored.SignatureAlgorithm
	reconstructed.KeyID = stored.KeyID
	reconstructed.PayloadHash = stored.PayloadHash
	if err := validateArvisStreamCanonicalVerdict(event, reconstructed, registry); err != nil {
		return fmt.Errorf("ARVIS stored signed final authentication failed: %w", err)
	}

	if status == "processing" {
		result, err := tx.ExecContext(ctx, `
			UPDATE arvis_stream_processing
			SET status='completed', last_error='', processed_at=now(), updated_at=now()
			WHERE stream_event_id=$1::uuid AND status='processing'
		`, event.ID)
		if err != nil {
			return fmt.Errorf("complete ARVIS signed-final processing claim: %w", err)
		}
		updated, err := result.RowsAffected()
		if err != nil || updated != 1 {
			return errors.New("ARVIS signed final processing claim transition not durable")
		}
	}
	return nil
}
