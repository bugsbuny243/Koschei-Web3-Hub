# Public ARVIS live feed performance contract

`GET /api/public/soc/feed` serves recent signed verdicts. It is not a synchronous
inventory endpoint for the entire ingestion journal.

## Request and truth boundaries

- The handler has a four-second overall database work budget. Existing route
  readiness checks remain in place before the handler.
- Verdicts are read on every request, from the last 24 hours only. An empty live
  window never triggers a historical query whose results would be discarded.
- A failed mandatory verdict read remains HTTP 503 with `ok:false`. This change
  does not weaken production smoke checks or replace missing evidence with cases.
- Optional telemetry has at most one second within the request budget. Failure
  produces `pipeline_status:unknown` and explicit `telemetry_status`; it never
  fabricates zero counts or a healthy pipeline.
- Telemetry is coalesced per handler and database handle for 15 seconds. Partial
  snapshots expire after two seconds. Verdict visibility is not cached here.
- `pipeline.observed_at` identifies the telemetry observation time independently
  of response `generated_at`.

## Inventory counters

Lifetime `raw_stream_events`, `recognized_events`, and `visible_verdicts` are
`null` on this bounded public telemetry path. They are not recomputed during
every browser poll. `raw_stream_events_estimate` is a separate PostgreSQL planner
estimate with `estimate_source:postgres_statistics`; it is never an exact count.
It may be stale between ANALYZE runs. Per-source lifetime counts are likewise
null; source freshness uses index-backed latest-event and latest-enrichment
timestamps. Existing operator inventory endpoints remain separate.

The exact current response size and grade counts remain in `summary`. A lack of
new signed verdicts is an empty result, not evidence that a target is safe.

## Validation

The regression tests exercise the real handler/store/database/sql path with an
explicit synthetic driver: a slow telemetry database, failed verdict reads,
empty recent windows, concurrent requests and nullable-vs-estimated counters.
Run `go test -race ./internal/handlers -run 'TestPublicRadar|TestBuildPublicRadar'`.

On 2026-09-21 the production source-freshness query used
`security_radar_stream_events_module_created_idx` and
`security_radar_stream_events_module_evidence_created_idx`; read-only
`EXPLAIN (ANALYZE, BUFFERS)` reported 3.150 ms execution for the two modules.
This is one query observation, not an endpoint percentile or load benchmark.
Post-deployment acceptance still requires public smoke and measured feed HTTP
latency. No index migration or destructive data operation is required.
