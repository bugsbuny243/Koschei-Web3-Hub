# Global Radar Graph V2 — snapshot isolation and migration gate (2026-10-20)

## Confirmed schema problem

Migration `005_global_radar_graph_records.sql` is immutable and already checksum-pinned. Its `ReplacingMergeTree(ingest_version)` `ORDER BY` omits `snapshot_sha256`. ClickHouse uses the sorting key as the replacement identity. Equal record IDs and payloads in different snapshots can therefore converge into one row under `FINAL`, breaking canonical snapshot replay. Existing missing snapshots cannot be recreated from a merged legacy table without another authoritative data source.

## Prepared code and DDL

- Additive migration `009_global_radar_graph_records_v2.sql` creates a new table with `snapshot_sha256` included in `ORDER BY`.
- The old `005` table, its checksum, all existing data and the default runtime behavior remain unchanged.
- With **explicit** `KOSCHEI_GLOBAL_RADAR_GRAPH_V2_ENABLED=1`, graph read, write and campaign replay use the new table, and startup schema verification rejects a wrong V2 sorting key.
- The new table is EMPTY after migration. The migration does not perform an automatic copy, backfill, live swap or replay cursor reset.

## Operator-only safe activation sequence (do not automate on a live service)

1. Keep `KOSCHEI_GLOBAL_RADAR_CLICKHOUSE_ENABLED`, background telemetry and Global Campaign workers **disabled** during migration. Start the ClickHouse service only after explicit cost approval.
2. Apply the checksum-verified migrations in a controlled environment; verify the V2 engine, 18-column contract and sorting key. CI exercises this in ClickHouse 26.2 and verifies that two distinct snapshot SHA-256 values with the same record/payload survive `FINAL`.
3. Inventory historical rows in legacy `global_radar_graph_records FINAL`, snapshots and current PostgreSQL replay checkpoint. Agree on a replay/backfill boundary **before** writing to V2. Recover any missing snapshots from their authoritative input sources; copying an already-collapsed `FINAL` view alone cannot restore lost history.
4. While legacy ingestion is paused, backfill into V2 with an explicit column list, excluding materialized `record_date`. Recheck both row-level payload hashes and snapshot rebuild hashes. Compare per-snapshot record counts and snapshot cursor coverage across the entire intended replay window.
5. Cut over `KOSCHEI_GLOBAL_RADAR_GRAPH_V2_ENABLED=1` and the relevant graph writer/reader consumers **together**, only after reconciliation proves V2 contains the history needed by the durable campaign replay cursor. Rollback by disabling the V2 flag, but reconcile any writes made solely to V2 before doing so.
6. Only then consider enabling ClickHouse graph collection and higher-cost background ingestion, with RPC and ClickHouse budget ceilings. Do **not** automatically enable them in this patch.

## Exact acceptance checks

- `system.tables.sorting_key` for V2 equals `record_type, network, record_date, subject_id, record_id, snapshot_sha256, payload_sha256`.
- Under V2 `FINAL`, repeated delivery of the same snapshot+payload converges, while identical records from **different** snapshots remain independent.
- Complete historical snapshot reconstruction verifies the original `snapshot_sha256` before advancing any durable campaign replay cursor.
- No live DDL, production variables or credentials are modified by this PR.
