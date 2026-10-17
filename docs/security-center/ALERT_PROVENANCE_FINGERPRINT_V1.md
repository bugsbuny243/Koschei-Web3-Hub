# Security Center — persisted alert provenance fingerprint v1

This draft PR builds on [#1397](https://github.com/bugsbuny243/Koschei-Web3-Hub/pull/1397), which itself depends on the owner-only incident review backend [#1396](https://github.com/bugsbuny243/Koschei-Web3-Hub/pull/1396). **Merge order: #1396 → #1397 → this change.**

## Purpose and evidence boundary

The existing owner-only `GET /api/owner/incidents?suggestions=1` projection is enriched with the **original persisted alert record metadata**:
- `created_at`, `last_seen_at`, `occurrence_count`
- `stored_payload_sha256` = lowercase SHA-256 of **the exact UTF-8 bytes of `security_alert_events.payload::text` read by PostgreSQL for that response**.
- `payload_hash_scope` explicitly declares the hash is a JSONB text read snapshot, not an immutable chain digest or a signature.
- `chain_evidence_verified=false` is a **fixed fail-closed status**, not inferred from the alert's severity or payload.

These fields are **source provenance context only**. They do not prove chain inclusion, a malicious actor, a valid validator signature, an immutable event history, or a full ARVIS verdict. The PostgreSQL JSONB text representation may not match the originally submitted JSON byte sequence; the hash is only reproducible from the same stored value and DB output representation.

The raw JSONB payload is read *inside* the Go handler solely for digest computation, not exposed in API JSON or the DOM. No provider URLs, secrets, embedded payload fields, private keys or customer records are added to the response. All review UI text uses DOM `textContent` and no HTML insertion.

## Existing behavior preserved

- No new route: opt-in existing GET response gains additive fields.
- No change to default `GET /api/owner/incidents`, POST creation/alerts linkage, underlying security verdicts, review acceptance authority, incident audit actions or authorization middleware.
- No database schema changes; SQL query remains bounded to indexed `created_at` interval and a limit of 100. No background jobs or network requests.
- The existing owner Security page and incident form are not replaced. The independent review card shows provenance inside its expandable evidence boundary.

## Validation

PostgreSQL integration test uses temporary synthetic alert fixtures and checks:
1. `payload::text` bytes reproduce returned fingerprint;
2. an edited synthetic payload changes fingerprint;
3. payload content markers do **not** appear in JSON response;
4. `chain_evidence_verified` remains false; and
5. manual incident creation still uses the existing alert reference and suppresses a linked active candidate.

Run the existing Node UI regression plus Go 1.26.9, PostgreSQL 17 and full required CI. This test never claims to authenticate an original upstream event or chain.
