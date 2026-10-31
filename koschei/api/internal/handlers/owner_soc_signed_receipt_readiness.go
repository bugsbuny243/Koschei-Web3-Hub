package handlers

import (
	"context"
	"database/sql"
	"strings"

	"koschei/api/internal/services"
)

const ownerSOCSignedReceiptProbeLimit = 20

// ownerSOCSignedReceiptProbe is a deliberately bounded, owner-only diagnostic.
// It never returns subject identifiers, canonical payload bytes, signer keys or
// signatures to the caller, and does not claim live transaction acceptance.
type ownerSOCSignedReceiptProbe struct {
	State                  string `json:"state"`
	LedgerTableReadable    bool   `json:"ledger_table_readable"`
	SampledReceipts        int    `json:"sampled_receipts"`
	VerifiedReceipts       int    `json:"verified_receipts"`
	IndependentTrustLoaded bool   `json:"independent_trust_loaded"`
	Scope                  string `json:"scope"`
	Boundary               string `json:"boundary"`
}

const ownerSOCReceiptProbeScope = "read_only_newest_20_immutable_receipts; exact_canonical_ed25519; configured_historical_public_keys; no_customer_or_production_writes"
const ownerSOCReceiptProbeBoundary = "A bounded historical signature sample does not prove fresh live ARVIS scan, customer debit, SOC case creation, complete ledger integrity, operator attendance, or external WORM retention"

func probeOwnerSOCSignedReceipts(ctx context.Context, db *sql.DB) ownerSOCSignedReceiptProbe {
	probe := ownerSOCSignedReceiptProbe{
		State:    "unavailable_not_accepted",
		Scope:    ownerSOCReceiptProbeScope,
		Boundary: ownerSOCReceiptProbeBoundary,
	}
	if db == nil {
		return probe
	}
	var exists bool
	if err := db.QueryRowContext(ctx, "SELECT to_regclass('security_unified_verdict_receipts') IS NOT NULL").Scan(&exists); err != nil || !exists {
		return probe
	}
	// Cap both the row count and returned payload bytes. A corrupted enormous
	// bytea is not sent to the handler or treated as a valid proof.
	rows, err := db.QueryContext(ctx, `
		SELECT network,target_kind,target_id,signature,signature_algorithm,key_id,
		       payload_hash,
		       CASE WHEN octet_length(signed_payload) BETWEEN 1 AND 65536
		            THEN signed_payload ELSE NULL END
		FROM security_unified_verdict_receipts
		ORDER BY recorded_at DESC,id DESC
		LIMIT 20
	`)
	if err != nil {
		return probe
	}
	defer rows.Close()
	probe.LedgerTableReadable = true
	type receipt struct {
		network, kind, target, signature, algorithm, keyID, payloadHash string
		bytes                                                           []byte
	}
	entries := make([]receipt, 0, ownerSOCSignedReceiptProbeLimit)
	for rows.Next() {
		var x receipt
		if err := rows.Scan(&x.network, &x.kind, &x.target, &x.signature, &x.algorithm, &x.keyID, &x.payloadHash, &x.bytes); err != nil {
			probe.State = "unavailable_not_accepted"
			return probe
		}
		entries = append(entries, x)
	}
	if err := rows.Err(); err != nil {
		return probe
	}
	probe.SampledReceipts = len(entries)
	if len(entries) == 0 {
		probe.State = "not_accepted_no_persisted_signed_receipts"
		return probe
	}
	registry, err := services.SOCUnifiedVerdictTrustRegistry()
	if err != nil {
		probe.State = "not_accepted_independent_trust_unavailable"
		return probe
	}
	probe.IndependentTrustLoaded = true
	for _, entry := range entries {
		if entry.algorithm != services.UnifiedVerdictSignatureAlgorithmV1 ||
			strings.TrimSpace(entry.kind) == "" || len(entry.bytes) == 0 {
			probe.State = "not_accepted_immutable_receipt_integrity_failure"
			return probe
		}
		identity, err := services.VerifyUnifiedRadarSignedReceipt(
			entry.bytes, entry.signature, entry.payloadHash, entry.keyID, registry,
		)
		if err != nil || identity.Network != entry.network || identity.Target != entry.target {
			probe.State = "not_accepted_immutable_receipt_integrity_failure"
			return probe
		}
		probe.VerifiedReceipts++
	}
	if probe.VerifiedReceipts != probe.SampledReceipts {
		probe.State = "not_accepted_immutable_receipt_integrity_failure"
		return probe
	}
	probe.State = "bounded_historical_signatures_verified_not_end_to_end_accepted"
	return probe
}
