# Owner SOC production read-only observation — 2026-11-01

## Purpose and boundary

The append-only Ed25519 receipts and owner-only verification endpoints are already deployed, but a successful Railway deployment or a PostgreSQL hash check **does not** establish production end-to-end acceptance. A historical receipt is not automatically an incident.

This operator tool makes the currently available **bounded, authenticated, read-only** evidence reproducible:

- GET /api/owner/soc/readiness (maximum 20 newest immutable receipts, independent historic trusted public keys, canonical SHA-256 and Ed25519 verification).
- GET /api/owner/soc/verdict-candidates (maximum 20 newest token receipts, independently verified by the server, with exact current owner SOC link state).
- Validate both response schemas and sample limits; fail closed for unavailable trust, corrupt/unknown responses, redirects, authorization failures, malformed references and oversized bodies.
- Report only counts, states and stable blocker codes. Never emit signed payload bytes, signatures, target identifiers, signing keys or owner credentials. This tool has **no POST calls**.

The output intentionally keeps production_e2e=not_accepted even if historical signatures and linked candidate samples are present. It does not perform a fresh customer scan, debit, owner incident creation, action history read-back, complete ledger sweep, archival export/restore or external WORM proof.

## Invocation

In a trusted operator environment with an existing secure owner secret injection:

    cd koschei/api
    KOSCHEI_BASE_URL=https://tradepigloball.co node scripts/verify-owner-soc-production-readiness.mjs

The script reads KOSCHEI_OWNER_SECRET from the environment; never put the secret in a command-line argument, artifact, ticket, log or PR. It accepts HTTPS origins only (except local loopback HTTP for tests). Non-successful historical signature or candidate verification returns a nonzero exit code. A zero exit code means *bounded historical observation succeeded*; it does **not** mean production SOC end-to-end acceptance.

Run isolated contract tests without any credential or network:

    cd koschei/api
    node --check scripts/verify-owner-soc-production-readiness.mjs
    node --test scripts/verify-owner-soc-production-readiness.test.mjs

The SOC PostgreSQL 17 acceptance workflow executes those same tests for relevant changes.

## Separate live operator acceptance

An authorized operator must additionally record an approved controlled new customer scan and the resulting receipt ledger ID, confirm the stored signature against a configured independent historical trust registry, create or link a genuinely applicable owner incident, and read back the exact evidence reference and action history in the authenticated SOC workspace. Do not manufacture a production incident or charge merely to make readiness counters nonzero.

A read-only database row count or successful hash comparison cannot replace that operation. Any missing trust, absent case link, failed migration, mismatched subject or signature remains **not accepted**.

## Remaining blockers

- Stream verdict legacy worker is intentionally fail-closed. The separately guarded canonical scheduler must not be enabled without operator dependency review and source-proof acceptance.
- Railway radar retention archive export was halted by missing/invalid KOSCHEI_RADAR_ARCHIVE_EXPORT_S3_ENDPOINT. No arbitrary endpoint, ephemeral disk or bypass is acceptable. An approved durable object store, independently verified restore, credential rotation, retention policy and audit trail are needed.
- PostgreSQL UPDATE/DELETE/TRUNCATE protections do not constitute external WORM or administrator-resistant ledger integrity. Independently held anchors/checkpoints and verified archive retention are separate future acceptance gates.
