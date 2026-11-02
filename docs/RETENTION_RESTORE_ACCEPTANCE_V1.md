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
