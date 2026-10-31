# Owner SOC — independent signed-receipt readiness (2026-10-31)

This is a **read-only diagnostic** on the **existing owner-only**
`GET /api/owner/soc/readiness`. It is not a public endpoint and does not
create cases, run scans, debit customer credits, mutate historical evidence,
or trigger enforcement.

## Evidence scope

`readiness.signed_verdict_receipts` tests at most the 20 newest
`security_unified_verdict_receipts` records. It reads exact canonical payload
bytes (up to 64 KiB per receipt), Ed25519 signature, key ID, stored payload
hash, network and target. It compares signed subject to the stored subject,
checks byte-level canonical JSON, SHA-256 and signature against
`KOSCHEI_ARVIS_STREAM_TRUSTED_VERDICT_KEYS_JSON` (independently configured
public keys and historical validity/revocation window). No private signer
material is read, and the API never returns the payload, identifiers,
signatures or keys.

Possible states:

- `unavailable_not_accepted`: database/table/query unavailable.
- `not_accepted_no_persisted_signed_receipts`: ledger is readable but empty.
- `not_accepted_independent_trust_unavailable`: no valid independent public
  trust registry; stored key IDs do not constitute verification.
- `not_accepted_immutable_receipt_integrity_failure`: at least one sampled
  receipt is corrupt, cannot be cryptographically verified, or mismatches its
  stored subject.
- `bounded_historical_signatures_verified_not_end_to_end_accepted`: all sampled
  historical decisions verified using external configured trust; **does not**
  establish a real new customer scan or owner case linkage.

The existing SOC case audit readiness state remains separate and unchanged;
neither projection can claim full production SOC acceptance. Empty observations
must never be displayed as healthy.

## Acceptance

- Unit test rejects absent database.
- Isolated PostgreSQL test covers empty ledger, genuine signed receipt,
  missing public trust, later mutation of the *mutable* fingerprint projection,
  and a forged database-inserted signature.
- The existing SOC PostgreSQL CI calls this test through
  `TestOwnerSOCImmutableCanonicalReceiptPostgres` and rejects any skip.
- After rollout, an authorized operator can fetch the existing owner-only
  readiness API in a normal session and record the *non-sensitive* state.
  Authentic customer scan→signed final→immutable DB receipt→explicit owner SOC
  case creation/link→case read-back still requires a separately controlled
  end-to-end acceptance test.

## Related operational blocker

Railway had no bucket or volume connected and the retention worker reported
`KOSCHEI_RADAR_ARCHIVE_EXPORT_S3_ENDPOINT` invalid. Retention is already
fail-closed before archive-backed deletion. Do not set a fabricated endpoint,
switch to ephemeral filesystem, or bypass archive verification. Configure and
verify an approved durable object store and restore test independently;
an online API process is not evidence that retention archival is healthy.
