# Security Center — exact persisted verdict lineage v1

**Review status:** additive draft only. Stack: #1396 (owner suggestions) → #1397 (owner UI) → #1398 (JSONB fingerprint) → this PR. These are not yet merged into main.

## Existing source of the link

Current `SecurityRadarCheckWithAlerts` persists ARVIS alerts via `alerts.Emit`. Its alert JSONB payload includes `signature`, `evidence_ref`, `target`, `grade`, `risk_level` and `rule_version`. The separate authoritative `security_radar_verdicts` store records `module_id`, `target`, `grade`, `risk_level`, `rule_version`, `signed`, `signature` and `signature_algorithm`.

The existing owner-only `GET /api/owner/incidents?suggestions=1` now uses a **parameter-free LATERAL join on persisted alert identity**. A match requires all of:

- source `arvis` + type `arvis.verdict.created`;
- verdict module `final_verdict_engine`;
- signed flag true and algorithm metadata `ed25519`;
- exact signature equals both alert payload `signature` and `evidence_ref`;
- exact verdict target equals both alert column `target` and payload `target`;
- exact grade, risk level and rule version.

Zero matches yield `no_exact_persisted_verdict_match`. More than one yields `ambiguous_persisted_verdict_matches` with **no** verdict ID or inferred network. One matching row yields `matched_persisted_signed_verdict_metadata_only`, with a stored verdict UUID and stored network. There is no fallback join on target alone.

All statuses include the explicit `persisted_signed_verdict_metadata_match_only_not_independent_chain_or_signature_verification` authority boundary. A matching database row **does not** mean an Ed25519 signature was cryptographically verified against a trusted key, an RPC observation was corroborated, or structured EvidenceRows were authenticated. The existing `chain_evidence_verified=false` is never changed.

## Query and exposure

- The query is read-only, with an already bounded seven-day alert window and 100-row limit, using the existing indexed `signature,module_id` verdict lookup.
- The alert payload is consumed in SQL only for exact predicates and in memory for its SHA-256 read-snapshot digest. No raw payload or verdict evidence JSONB is returned.
- The frontend presents the outcome in the expandable operator review area using DOM `textContent` only.
- Existing incident creation still independently verifies `alert_refs`. No automatically created case, no changed alert delivery, no ARVIS grade modification, no new auth/route/database/worker component.

## Validation

Tests exercise missing, ambiguous, not-ARVIS, and incomplete identities. PostgreSQL 17 integration uses disposable **synthetic signed metadata** that satisfies SQL schema constraints but does not represent a cryptographic signature, then mutates the alert's rule version and evidence reference to ensure the linkage fails closed. It also checks no incident was created by a GET. The existing owner UI Node test checks visible lineage and explicit evidence limits.

No migration is introduced, no protected file is changed. Merge only after #1398, run Go and PostgreSQL CI on a rebased branch and inspect the server-side owner authentication boundary.

## Explicitly NOT implemented

This does **not** bind RPC JSON responses, transaction inclusions, Solana token-account/owner snapshots or any `radaradapter.EvidenceRow` to the stored alert. That requires an additional validated source-to-verdict binding and provenance retention layer. Do not market this as chain-verified alert provenance.
