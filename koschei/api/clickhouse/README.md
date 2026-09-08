# Koschei Web3 ClickHouse shadow journal v1

Status: additive validation path only.

## Hard boundary

This work does **not** change the ClickHouse Cloud plan, service tier, replica size,
replica count, TDE setting, or any billing/package setting. Infrastructure-plan
changes are outside this migration.

It also does not modify Koschei Sentinel or Koschei Lang.

## Why shadow first

The deployed ARVIS stream path still relies on PostgreSQL semantics including
`ON CONFLICT`, row updates, transactions, `FOR UPDATE SKIP LOCKED`, and existing
idempotency constraints. Replacing that path in one step would silently change
reliability guarantees.

The first ClickHouse gate is therefore read/copy/verify:

```text
existing PostgreSQL security_radar_stream_events
        |
        | bounded read-only REPEATABLE READ snapshot
        v
ClickHouse security_radar_stream_events shadow
        |
        +--> countDistinct(event_id) parity
        |
        +--> metadata + evidence payload SHA-256 content parity
```

No customer verdict is read from ClickHouse in v1. PostgreSQL remains the live
source for current workers while the shadow data model is measured.

## Schema

Migration:

```text
clickhouse/migrations/001_security_radar_stream_events.sql
```

The table uses:

- native `UUID`, `UInt64`, `DateTime64`, and `JSON` types,
- `LowCardinality(String)` for repeated categorical dimensions,
- byte-exact chain identifiers,
- SHA-256 receipts for `decoded` and `raw_event` payloads,
- constraints that reject non-hex event/evidence hash values,
- `ReplacingMergeTree(ingest_version)` so a repeated shadow copy of the same
  committed PostgreSQL event can converge without mutation-heavy `UPDATE`,
- an `ORDER BY` built from current low-cardinality stream dimensions plus the
  stable PostgreSQL event UUID,
- no guessed partitioning or skipping indexes before measured production data.

`target`, `signature`, and `program_id` must never be lowercased. This is required
for Solana base58 identity integrity and remains the rule for future chain-native
identifiers unless a chain-specific adapter explicitly defines another canonical
form.

The shadow client normalizes `created_at` to the table's millisecond precision
before insertion and before parity hashing, so PostgreSQL sub-millisecond precision
cannot create a false mismatch after ClickHouse `DateTime64(3)` storage.

### Guarded schema command

The schema command uses the same server-side ClickHouse environment variables as
the shadow copier. By default it performs **verification only** and does not write.

From `koschei/api`:

```text
go run ./cmd/clickhouse-schema
```

To deliberately apply migration 001 and immediately verify the resulting engine,
sorting key, and required native column types:

```text
KOSCHEI_CLICKHOUSE_SCHEMA_APPLY=1 go run ./cmd/clickhouse-schema
```

The apply path is intentionally narrow. It accepts only additive `CREATE DATABASE
IF NOT EXISTS` and the trusted `CREATE TABLE IF NOT EXISTS
koschei_web3.security_radar_stream_events` migration. `ALTER`, `DROP`, `TRUNCATE`,
`DELETE`, `UPDATE`, `INSERT`, and unrelated table creation are refused by the
client guard. Credentials and connection URLs are never printed.

The currently connected ChatGPT/ClickHouse query surface is read-only. Therefore
production schema application remains blocked until a server-side ClickHouse
credential is configured; the password itself must stay in the deployment secret
store and must not be pasted into chat or committed to the repository.

## One-shot bounded parity copy

The copier is deliberately a standalone command. It does not alter ARVIS runtime
wiring and does not delete data from PostgreSQL.

Required server-side environment variables:

```text
DATABASE_READ_URL=...          # preferred; DATABASE_URL is fallback
CLICKHOUSE_HTTP_URL=https://<service-host>:8443
CLICKHOUSE_DATABASE=koschei_web3
CLICKHOUSE_USER=default
CLICKHOUSE_PASSWORD=...
```

Optional safety controls:

```text
# RFC3339; defaults to the last 24 hours.
KOSCHEI_CLICKHOUSE_SHADOW_SINCE=2026-09-07T00:00:00Z

# ClickHouse best-practice insert batch. Default 10,000; range 1,000..100,000.
KOSCHEI_CLICKHOUSE_SHADOW_BATCH_SIZE=10000

# Refuse unexpectedly large source windows. Default 100,000; hard cap 10,000,000.
KOSCHEI_CLICKHOUSE_SHADOW_MAX_ROWS=100000

# HTTPS request timeout. Default 5 seconds; bounded to 250..30,000 ms.
CLICKHOUSE_TIMEOUT_MS=5000
```

Run from `koschei/api`:

```text
go run ./cmd/clickhouse-shadow
```

The command:

1. fixes the `[since, now)` window before reading,
2. opens a read-only PostgreSQL `REPEATABLE READ` snapshot,
3. counts the source rows and refuses the run above the configured safety cap,
4. reads source rows in deterministic UUID-text order,
5. computes a SHA-256 content fingerprint over metadata plus the SHA-256 receipts
   of `decoded` and `raw_event`,
6. streams rows in bounded batches,
7. inserts with ClickHouse async inserts and waits for acknowledgement,
8. verifies `countDistinct(event_id)` under bounded ClickHouse query settings,
9. reads the ClickHouse shadow through `FINAL` and independently reconstructs the
   same content fingerprint,
10. fails unless both row count and content fingerprint match exactly.

The content parity check is stronger than row-count parity: a changed target,
signature, slot, program ID, timestamp, decoded evidence payload, or raw evidence
payload changes the resulting fingerprint even when the number of rows stays the
same.

Passwords and connection URLs are never printed by the command.

## What this does not claim

This is **not** a PostgreSQL cutover and is not globally lossless ingestion.
ClickHouse does not yet own:

- the ARVIS processing claim queue,
- enrichment state transitions,
- verdict writes,
- gap-healer checkpoints,
- customer-facing reads.

Those paths currently depend on PostgreSQL transaction/locking semantics and must
be redesigned rather than translated line-for-line.

## Cutover gates

A future live ClickHouse path is allowed only after all of these are evidenced:

1. shadow row and content parity over progressively larger windows,
2. exact case-sensitive identifier parity,
3. replay/idempotency behavior under repeated copies,
4. bounded insert latency and backlog under load,
5. failure injection proving the existing ARVIS verdict path is unaffected,
6. an append-first replacement for mutable processing-state claims,
7. verified export/readback of historical PostgreSQL product data before any
   retirement or deletion,
8. CI tests, vet, build, and security gates green.

Google Drive remains the evidence-oriented archive already defined by the current
memory boundary; ClickHouse is the queryable intelligence/event store candidate,
not a replacement for immutable evidence receipts.
