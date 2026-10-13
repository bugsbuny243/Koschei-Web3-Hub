# Stream table parity audit — October 2026

Status: **NOT EXECUTED**. This document is an audit plan, not production evidence.

## Scope and preservation

Source: PostgreSQL `security_radar_stream_events`. No production DELETE, DROP, TRUNCATE or index removal has been performed by this PR. Preserve existing data until verified backup and row-level parity are approved. The claimed 97 GB size has not been independently measured.

## Read-only production measurements (run with read-only credentials)

```sql
SELECT pg_size_pretty(pg_total_relation_size('security_radar_stream_events')) AS total_size;
SELECT count(*) AS total_rows,
       count(*) FILTER (WHERE target IS NULL OR target = '') AS no_target,
       count(*) FILTER (WHERE target = signature) AS signature_as_target,
       count(*) FILTER (WHERE evidence_quality = 'decoded_stream_hint') AS decoded_hints,
       count(*) FILTER (WHERE raw_event IS NOT NULL) AS raw_json_rows,
       count(*) FILTER (WHERE raw_event_sha256 IS NOT NULL) AS hashed_rows
FROM security_radar_stream_events;
SELECT evidence_quality, module_id, event_type, target_type, count(*) AS rows
FROM security_radar_stream_events
GROUP BY 1,2,3,4 ORDER BY rows DESC;
SELECT indexname, indexdef FROM pg_indexes
WHERE schemaname = current_schema() AND tablename = 'security_radar_stream_events';
```


## Production execution precautions (not yet performed)

Use a read-only account on a replica where possible. Full-table `count(*)` and `GROUP BY` can scan the entire ~97 GB table and affect latency. Run off-peak, review query plans and stop on timeouts. The following transaction provides a bounded, read-only audit session:

```sql
BEGIN READ ONLY;
SET LOCAL statement_timeout = '30s';
SET LOCAL lock_timeout = '2s';
SELECT current_database() AS database_name, current_schema() AS schema_name,
       pg_size_pretty(pg_total_relation_size('security_radar_stream_events')) AS total_size;
SELECT reltuples::bigint AS estimated_rows
FROM pg_class
WHERE oid = 'security_radar_stream_events'::regclass;
COMMIT;
```

The estimated row count is **not** parity evidence. Run exact counts only in an approved audit window. Confirm `raw_event_sha256` exists before running the hash-coverage query, since migration 135 may not be deployed yet.

Migration 135 adds a nullable hash column and drops the `raw_event` NOT NULL constraint. Even metadata-only PostgreSQL DDL takes table locks; review active sessions and configure a short `lock_timeout` during the controlled rollout. Deploy schema before the new stream writer. Never run an unreviewed migration against the live 97 GB table.

## Parity and retained copy — pending

Before any retention enablement, select preserved evidence classes and copy them into a dedicated retained table in a reviewed forward-only migration. Record source count, copied count, distinct IDs, and an ID-level anti-join in both directions. Require zero missing IDs and a verified backup before allowing any deletion. No counts are asserted until measured.

## Changes and controls

- Filtered event types: pure Buy/Sell logs without lifecycle or authority markers.
- Dropped indexes: **none**.
- Recovered rows: **not measured**.
- New raw notification storage: SHA-256 digest in `raw_event_sha256`; raw JSON no longer duplicated for new writes.
- Retention: `KOSCHEI_STREAM_RETENTION_DAYS=14` by default, `KOSCHEI_STREAM_RETENTION_ENABLED=false` by default. A separate `KOSCHEI_STREAM_RETENTION_PARITY_APPROVED=true` is also required before any deletion.
- Retention execution: worker-only function, no scheduler or API route wired. **Do not enable until parity and backup approval.**

## Remaining acceptance gates

Measure production statistics; verify a retained-table copy and row parity; evaluate index removal only after write-side changes are deployed; run full Go formatting, vet, test and migration CI; obtain explicit authorization before enabling retention.
