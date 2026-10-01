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
