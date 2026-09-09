# ClickHouse ARVIS intelligence memory read v1

## Purpose

This layer makes the existing ClickHouse ARVIS shadows queryable as an evidence-memory surface without changing ARVIS runtime behavior.

It correlates only two already-defined shadow sources:

- `security_radar_stream_events`
- `arvis_verdict_snapshots`

The correlation key is the committed `event_id` UUID already carried by an ARVIS verdict. No graph relation, actor attribution, behavior, intent, risk score, or attack path is invented from row co-occurrence.

## Boundary

ARVIS remains authoritative for scanning, verdicts, evidence production, queue/lease/retry state and customer decisions. This read layer does not modify:

- `SecurityRadarStore.InsertVerdict`
- `arvis_stream_processing`
- stream workers
- scoring or rule execution
- customer API responses
- PostgreSQL state
- production ClickHouse schema or data

The command is read-only.

## Exact subject identity

`target` and `network` are compared with exact equality. Chain-native targets are not lowercased. This is required for Solana and for future chain adapters whose identifiers are case-sensitive.

## Evidence integrity

For verdict memory rows the reader:

1. validates UUID and schema contracts;
2. verifies the stored evidence-array SHA-256 against the returned evidence array;
3. canonicalizes the returned signals JSON and verifies its SHA-256;
4. rejects a row if the exact requested target/network boundary is violated.

For stream rows the bounded read intentionally does not dereference `decoded` or `raw_event` payload bytes. It exposes their stored SHA-256 digests plus event metadata. Therefore the response says `stream_payload_policy=digests_only_not_dereferenced`; it does not claim byte-level stream-payload verification.

## Correlation semantics

Each verdict is classified as:

- `linked` — the verdict contains a non-zero `event_id` and the exact target/network/time-window stream read contains that event UUID;
- `no_event_reference` — the verdict is a legitimate standalone/manual verdict with the zero UUID source reference;
- `event_not_observed_in_window` — the verdict references an event that was not present in the bounded stream read.

A missing event stays explicit. The reader never fabricates a relation or silently widens the window.

## Query safety

The reader requires:

- an exact target;
- an exact network;
- an explicit valid time window no larger than 31 days;
- a per-source row limit from 1 to 2,000.

Every ClickHouse query also sets bounded server-side execution/result/scan limits. If a source exceeds the requested row bound, the read fails instead of returning an apparently complete truncated view.

## Operator command

```bash
cd koschei/api
CLICKHOUSE_HTTP_URL='https://<service-host>:8443' \
CLICKHOUSE_DATABASE='koschei_web3' \
CLICKHOUSE_USER='default' \
CLICKHOUSE_PASSWORD='<server-side-secret>' \
go run ./cmd/clickhouse-intelligence-read \
  --target '<exact-chain-target>' \
  --network solana-mainnet \
  --since 2026-09-08T00:00:00Z \
  --until 2026-09-09T00:00:00Z \
  --limit 200
```

Credentials must remain server-side and must not be committed, printed, placed in frontend code, or embedded in `CLICKHOUSE_HTTP_URL`.

## What this unlocks next

Once real shadow data exists and bounded reads are measured, this evidence-memory surface can feed the already-existing `IntelligenceInvestigation` contract. The next projection must preserve the same rule: relationships and behavior findings require concrete evidence references; absence or incomplete evidence remains unknown/unverified.
