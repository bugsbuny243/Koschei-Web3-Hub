# ARVIS immutable signed final receipts — 2026-10-31

## Trust boundary

The mutable `security_unified_radar_verdicts` fingerprint projection can update signature metadata on later scans. It is **not** an immutable audit log. Migration `138_unified_verdict_immutable_receipts.sql` creates `security_unified_verdict_receipts` with one row per unique real signed Ed25519 final. It captures **exact canonical signed bytes** as `bytea`, signature, signing key ID, payload hash, subject and original projection row ID.

The application writes projection + receipt in **one transaction**; a missing table, invalid live signature or ledger insert failure rolls back the projection and cannot produce a billable customer receipt. Exact duplicates are idempotent, and conflicting metadata for a duplicate signature fails closed.

Database triggers reject UPDATE, DELETE and TRUNCATE on the new receipt table. These are SQL DML controls, **not** external WORM or protection from database superusers, DDL changes, lost backups, or a malicious administrator. Export/backups and durable key management remain operational requirements.

## Owner SOC verification

Customer `final_verdict_receipt` now contains `ledger_id` plus stable
`soc_evidence_ref: unified-verdict-ledger:<receipt_uuid>:<signature>`.
The owner-only case creation / explicit link flow checks the precise target/network and independently verifies:

1. exact saved immutable canonical JSON bytes (including signed timestamp, evidence, target, network and rules);
2. SHA-256 of the canonical bytes;
3. canonical Ed25519 signature;
4. a public key from the **out-of-band** `KOSCHEI_ARVIS_STREAM_TRUSTED_VERDICT_KEYS_JSON` trust registry (key-id and validity/revocation time checked); and
5. that the resulting verified subject equals the manually selected SOC incident subject.

The trusted keys JSON already used by ARVIS stream recovery is an array of `GlobalRadarTrustedVerdictKey` with `key_id`, `public_key_base64url`, and optional `valid_from`, `valid_until`, `revoked_at`. Retain historical public keys for legitimate prior signatures. **Never** derive historic trust from the row's key ID or a key embedded inside the evidence itself. Missing/malformed registry, wrong signatures, changed bytes and subject mismatches fail closed.

Legacy `unified-verdict:<mutable_uuid>:<signature>` references remain accepted **only at their old stored-signature-match-only assurance level** for backward compatibility. They must not be relabelled as independently verified evidence.

The ledger does not auto-create alerts, decide case severity, assign analysts, or authorize firewall/contract intervention. No public unguarded endpoint was added.

## Deployment and acceptance

- Apply migration **138** before rolling out the new binary. Verify triggers exist, backup retention and the operator-managed trusted-public-key configuration.
- The customer billable gate requires `ledger_id` from a fresh, matching history read. Until the migration is live, signed customer scans **fail closed as persistence pending with no debit**.
- Isolated PostgreSQL integration: signed final persists atomically, history returns ledger ID, cases accept only exact independently verified reference, old mutable row changes do not destroy the receipt, and UPDATE/DELETE/TRUNCATE fail.
- Unit verification: independent trust across key rotation, wrong key, modified payload, invalid hash and noncanonical payload.
- GitHub CI passing does **not** prove Railway authenticated owner scan and SOC write acceptance; live end-to-end test must be reported separately with a controlled operator account.

Historical rows from before migration 138 **cannot be upgraded** into cryptographic immutable receipts merely by copying their current mutable signature fields. Such rows remain legacy unless the original canonical signed bytes and independently trusted key can be proven.
