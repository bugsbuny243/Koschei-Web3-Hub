# Retention Restore Acceptance v1

Koschei's radar retention path separates four states:

1. source rows in the PostgreSQL hot store;
2. checksum-bound rows in `radar_retention_archive`;
3. immutable exported NDJSON objects verified by destination readback;
4. restore acceptance reconstructed from the exported object.

## Restore acceptance

`retentionexport.VerifyRestoreAcceptance` takes one exported NDJSON object and:

- verifies the complete object SHA-256 when an expected digest is supplied;
- verifies every exported row checksum against PostgreSQL canonical `jsonb` text;
- rejects duplicate source identities;
- opens a transaction-scoped PostgreSQL restore staging table;
- reconstructs every source identity, checksum and payload into that table;
- requires every source table to be one of the production retention-managed targets;
- round-trips every payload through that source table's **current PostgreSQL row type**;
- requires the typed row to reproduce the archived source ID and canonical payload checksum;
- verifies staged row-count parity;
- verifies staged canonical-payload checksum parity;
- intentionally rolls the transaction back.

The restore acceptance never writes reconstructed records to production source
tables. It proves that the archived bytes can be reconstructed into PostgreSQL
canonical JSON without allowing an automated restore command to overwrite live
security evidence. If an archived payload no longer round-trips through the current
source-table schema, acceptance fails explicitly; schema evolution then requires a
reviewed restore migration rather than silently dropping or inventing fields.

## Post-prune acceptance

The PostgreSQL 17 integration gate exercises the stronger sequence:

```text
archive staging rows
  -> filesystem export
  -> destination readback SHA-256
  -> export ledger mark
  -> archive staging prune
  -> read exported NDJSON only
  -> PostgreSQL restore-stage reconstruction
  -> source-table typed reconstruction
  -> source-ID + row-count + checksum parity
```

This proves the restore path does not depend on the staging archive rows still
being present.

## Operator command

```bash
DATABASE_URL=... go run ./cmd/retention-verify \
  -file /path/to/export.ndjson \
  -sha256 <expected-object-sha256> \
  -restore-acceptance
```

The command is verification-only. It does not restore into canonical production
tables.

## Production boundary

Passing repository CI closes the implementation and PostgreSQL-17 restore-proof
gate. Final GA still requires a production archive destination, a retained
production object, a restore-acceptance run against that exact object's checksum,
and deployment evidence tying the acceptance to the production revision.

## Automatic retention secure destination gate (2026-11-02)

The autonomous `StartSecurityRadarRetentionWorkerV2` and legacy rollback
retention worker share `exportPendingArchive`. Both **halt before any
archive/delete SQL** when `KOSCHEI_RADAR_ARCHIVE_EXPORT_SINK` is unset,
`disabled`, or `filesystem`. The deletion path requires a configured S3
destination with an **HTTPS** endpoint (not plain HTTP or URLs with embedded
user credentials). The sink still must pass its actual provider connectivity
and write/readback checks; an HTTPS-shaped URL alone does not authorize
deletion.

The standalone filesystem exporter and `retention-verify` remain available
for isolated test fixtures and explicit restore verification; their existence
does **not** authorize automated production retention against an ephemeral
container directory. A durable mounted filesystem would require a separate
reviewed architecture and acceptance gate rather than bypassing this policy.

GitHub's PostgreSQL 17 acceptance checks verify:
- Disabled/empty external export stops both workers' shared preflight.
- Filesystem, HTTP and userinfo-bearing S3 endpoints are refused.
- Failed preflight persists only the sanitized
  `archive_export_preflight_failed` run status with zero
  selected/archived/verified/deleted/pruned rows.
- The normal archive export, checksum and restore tests still run separately.

**Production operations remain blocked** until an approved durable S3 object
destination is connected, the exact exported object has passed independent
SHA-256 and row-parity readback, and an operator runs the typed PostgreSQL
restore acceptance against that object. No configuration label, green CI
status or empty archive backlog replaces this evidence.

## Unconnected external proof-before-source-delete pipeline (2026-11-02)

**Critical distinction:** `retentionArchiveQueryV2` and the legacy
`retentionArchiveQuery` atomically stage source rows to
`radar_retention_archive` and delete the original source rows in PostgreSQL.
The separate `after_retention` / `after_retention_v2` object export runs
*after* that SQL. Therefore HTTPS/S3 sink connectivity **does not prove**
that the exact source rows were durably exported before their deletion.
A failed later export would leave those rows only in the same PostgreSQL
archive tier, not in an independently verified cold archive.

Both automatic retention workers now **refuse original source-row deletion**
through `retentionRequireExternallyVerifiedSourceArchive` until a reviewed,
tested, per-row external receipt-before-delete protocol is implemented.
There is deliberately no configuration flag that overrides this guard.
The guard applies even if S3 is reachable; previously staged archive
backlog can still be exported and verified without removing original rows.
The v1/v2 direct worker methods also enforce this guard. The separate
`RunSecurityRadarStreamRetention` decoded-hint deletion path requires the
same external row proof **even when both** `KOSCHEI_STREAM_RETENTION_ENABLED`
and `KOSCHEI_STREAM_RETENTION_PARITY_APPROVED` are explicitly true; parity
approval flags alone cannot supply an immutable off-database receipt.
The low-level SQL builders remain available only for isolated
archive/checksum tests and cannot authorize autonomous production deletion
by themselves.

To restore the automatic deletion capability, implement a reviewed two-phase
pipeline that (1) stages exact immutable source snapshots, (2) exports them
to an approved durable external object store, (3) independently verifies
object SHA-256, source identities and row counts, (4) binds a durable
per-row export receipt to the original source revision, and (5) deletes
only those still-matching originals with bounded locks, retry/idempotency
and a tested rollback path. A successful HTTP connection probe, sink
configuration, database-only checksum or generic boolean flag is never
sufficient. Include PostgreSQL 17 crash/retry, mutation and mismatched
receipt negative tests and production operator approval before promotion.

The production archive target is still absent. CI verifies the fail-closed
guard but cannot establish real production archival or recovery.

## Required independent object digest for restore acceptance (2026-11-02)

The `-restore-acceptance` mode now **requires** `-sha256` as exactly
64 hexadecimal characters. The `VerifyRestoreAcceptance` service function
enforces the same requirement; no caller can obtain successful typed restore
acceptance with an empty or malformed expected object digest. The CLI
rejects a missing digest before opening a file or connecting to PostgreSQL.

The expected SHA-256 must be obtained from an authenticated and independently
reviewed **archive export receipt/object ledger** and correlated with the
actual provider object key, sink, source-table identity, retention run and
export run. Recomputing a hash of a downloaded file and passing it back as
`-sha256` only proves self-consistency, **not** independent archive identity
or immutability. Matching the supplied hash is necessary but **not sufficient**
for production sign-off: require exact-object provider readback, source
identity/count parity, typed restore and approved retention policy.

A dedicated CI gate rejects the CLI's no-digest mode; the PostgreSQL 17
restore acceptance test checks absent, nonhex and mismatched object digests,
then independently exercises the tampered-source-ID negative case using
the tampered bytes' exact expected digest.

## Export ledger provider-error redaction (2026-11-02)

The `radar_retention_export_runs.error_message` field previously stored
`runErr.Error()` directly. Object-storage responses and SDK errors can
contain endpoint paths, request identifiers, signed query parameters, or
credential-bearing details. Failed export runs now persist only fixed,
non-sensitive reason codes:

- `archive_export_failed`: a generic exporter failure.
- `archive_export_checksum_mismatch`: at least one object readback
  checksum mismatch; the separate `checksum_mismatches` counter remains
  intact.

Exported row/object/byte counts, status, last safe object reference and
lock-acquired detail remain available for troubleshooting. An isolated
PostgreSQL 17 regression test inserts synthetic failed/completed export
runs, checks the persisted error-code boundary and removes only its own
test records.

**Important limitations:** This change only protects new records after
the updated code is deployed. It does not inspect or rewrite existing
production `error_message` contents. Do not copy historical raw provider
errors into issue comments, CI reports, logs, or public dashboards; any
historical cleanup requires separate authorized review.

## Archive staging checksum parity before export (2026-11-02)

Before choosing an external object key or sending staged rows to a sink,
`SQLRepository.LoadPending` now recomputes each row's canonical
`sha256(payload::jsonb::text)` within PostgreSQL and refuses the entire
batch if it differs from the staged `row_checksum`. The exporter must not
treat a correctly transmitted but internally corrupted archive object as
valid cold-storage evidence.

`SQLRepository.MarkExported` independently performs the same canonical
payload checksum check within its atomic update, so a changed or invalid
staging payload cannot be marked as externally verified by trusting the
stored checksum column alone. The operation fails closed, preserving the
staged row for explicit operator investigation rather than recording an
export receipt for data with failed source-row integrity.

The isolated PostgreSQL 17 test `TestArchiveStagingChecksumMustMatchPayloadPostgres17`
creates a deliberately mismatched synthetic archive row (a correctly
formatted 64-hex string with the wrong value), proves both load and
export-marking reject it, and cleans up the synthetic source-independent
run and staging records. This is **repository CI integrity evidence**;
it is not proof that a real production S3 object has passed operator
acceptance, nor does it enable automatic deletion of any source row.

## Read-only operator search of one approved archive object

The `cmd/retention-lookup` tool provides a **bounded metadata-only** first
step toward historical evidence retrieval. It searches one previously
downloaded NDJSON export file, **not** a bucket-wide S3 inventory or a live
ARVIS runtime index. There is no public HTTP route, no private object URL
in the output and no production source-table writes.

Before returning **any** match, the tool requires:
- an independently supplied approved object's 64-hex SHA-256 and byte
  equality with the local file; a locally recomputed hash alone is not
  independent provenance;
- a PostgreSQL connection used only for canonical JSONB SHA-256 verification
  of **every row in the object**, including rows outside the requested filter;
- source identity parity for **every row**: each row's declared `source_id`
  must match the current managed table's ID column in its JSON payload.
  A forged nonmatching row rejects the entire lookup, even when its payload
  checksum is mathematically correct;
- one known retention-managed `source_table`, plus either an exact
  `source_id` or an archive timestamp bound;
- a maximum of 100 result entries (default 25), reporting `truncated`
  if the object contains further matches.

Results contain `archive_id`, `source_table`, `source_id`,
`row_checksum` and `archived_at`. They **never include raw payloads**.
Timestamp bounds are UTC-aware RFC3339; `archived_at` means the PostgreSQL
staging archive timestamp, **not** blockchain event time or attack time.
Even a successful lookup does not prove signed ARVIS provenance, a complete
cross-object search, or reconstructability through the current table schema.
Use `retention-verify -restore-acceptance` separately for typed restore.

Example for an authorized operator using a **read-only database role**:

```bash
DATABASE_URL='postgresql://<read-only-role>@<approved-host>/<database>' \
  go run ./cmd/retention-lookup \
    -file /approved/local-copy/export.ndjson \
    -sha256 "$TRUSTED_OBJECT_SHA256_FROM_APPROVED_RECEIPT" \
    -source-table security_radar_events \
    -source-id "$EXACT_SOURCE_ID"
```

Alternatively provide `-archived-from 2026-10-01T00:00:00Z` and
`-archived-until 2026-10-02T00:00:00Z` for a bounded archive-time window,
and optionally `-limit 25`. A zero-match result means only **this one object**
does not contain the requested row; it says nothing about other archived
objects. No file, bucket, or production database is changed by the tool.

A PostgreSQL 17 acceptance test checks authorized matching metadata,
archive-time filtering, bounded results, absence of payload leakage, missing
or forged object digest, unrecognized table, unbounded queries, a corrupt
**nonmatching** row, and a deliberately forged source identity whose payload
checksum is otherwise valid. CI also asserts that the CLI refuses a
missing trusted object SHA before opening any object or connecting to a DB.

**Future, separately reviewed work:** owner-scoped authorization and audit
trails, an independently accepted S3 object inventory and digest/receipt
index, controlled object retrieval, source/event-time search, typed
reconstruction, and runtime ARVIS integration. Do not expose this operator
command as a customer-facing search API or claim that it provides global
historical search.
