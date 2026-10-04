# Production readiness changes — 2026-10-04

This release closes the code-level snapshot handoff and outbound HTTP security
backlogs. It does not declare Global Radar GA, enable automatic response, or
substitute test fixtures for live acceptance evidence.

## Resulting behavior

- An enabled campaign producer commits a bounded canonical snapshot to PostgreSQL
  before ClickHouse publication. Expired five-minute claims recover after a crash.
  Stable source hashes deduplicate enqueue; campaign jobs and publication receipts
  commit together. Completed receipts discard snapshot JSON. Failed canonical
  payloads remain quarantined for operator review. Outstanding work is capped at
  10,000 rows; pressure is returned to the producer.
- Publication HTTP calls hold no PostgreSQL transaction. Fencing tokens and the
  database clock prevent stale ACK/retry writes after ownership transfer.
- Campaign workers wake on enqueue, due retries, or the existing 15-minute recovery
  ceiling. Empty queues release the worker lease and do not poll every ten seconds.
- `/owner/campaigns` shows pending, publishing, failed and completed deliveries.
  Explicit Radar bridge continuity feeds technical threat-family projections;
  missing tempo and ARVIS threat-pathway inputs stay incomplete. This conveys no
  identity, malicious intent, verdict or execution authority.
- Production provider requests use HTTPS, public IPs pinned at connection time,
  verified TLS, and same-origin redirects. Mixed public/private DNS answers,
  local/special addresses, URL credentials, proxies and alternate TLS dial paths
  cannot bypass the guard. Connection pools are bounded. Solana pressure/fallback
  middleware recursively protects each provider while keeping its retry budget.
- G703/G704 are blocking again in both security workflows. The shared HTTP
  boundary contains two documented local G704 annotations; it is covered by DNS,
  redirect and private-address tests. There are no global rule exemptions.

## Verified live inventory

| Component | Observed state | Implication |
|---|---|---|
| Neon production | PostgreSQL 17.11; min/max 0.25 CU; suspend after 300 seconds | Desired compute settings already exist; actual idle behavior needs traffic evidence |
| Neon storage | Approximately 104 GB decimal; stream heap approximately 74 GB | Old 8 GB estimates are obsolete; retention cost must use current measurements |
| Campaign queue | Empty | Zero work is not campaign coverage evidence |
| Retention ledger | Latest measured run selected/archived/deleted zero rows | No live restore proof can be inferred from this run |
| ClickHouse Cloud | 26.6; existing stream journal; Global Radar tables absent | Graph/event/checkpoint/gap migrations must precede activation |
| Railway variables | Global Radar enable flags absent; `CLICKHOUSE_PASSWORD` absent from the variable inventory | Do not enable the campaign runtime before connection/schema verification |
| External archive | Railway S3 bucket `radar-archive`, Amsterdam; service reference variables configured | Next deployment verifies authenticated write/read even with an empty archive queue |

Provider secrets are never copied to this repository or report. External archive
references resolve inside Railway. An immutable connection marker is distinct
from a real archived-row export/restore proof; its log explicitly says
`restore_proof=false`.

## Validation and operational evidence

- Native PostgreSQL 17 CI covers stage deduplication, crash recovery, active lease
  exclusion, stale ACK/retry fencing, uncertain publication retry, atomic enqueue,
  receipt compaction and poison quarantine. A single-connection test pool proves
  no SQL transaction spans external publication.
- Migration 132 was applied successfully on the separate Neon branch
  `readiness-20261004`, PostgreSQL 17.11. Production migrations run through the
  existing application migration ledger on deployment.
- Go tests, vet/build, targeted race checks, 99 browser unit tests and gosec run
  without the previous global rule exclusions. CI records native database tests
  and package checksums separately from deployment evidence.
- `Production Observation Evidence` records the exact deployed revision, public
  coverage/runtime snapshots and authenticated queue counts when the existing
  Actions owner credential is available. Each hourly observation has a checksum
  and a 30-day artifact. Missing credentials or evidence produce `UNKNOWN`.
- TypeScript CI publishes reviewable `.tgz` artifacts with source revision and
  SHA-256 checksums. These artifacts do not prove npm registry publication;
  `@koschei/verifier` remains private until its publication metadata and publisher are established.

## Activation and acceptance work that still requires evidence

1. Configure the existing ClickHouse application credential through Railway's
   secret interface. Run `KOSCHEI_CLICKHOUSE_SCHEMA_APPLY=1 go run
   ./cmd/clickhouse-global-radar-schema` from `koschei/api` with the existing
   ClickHouse environment. It validates all four checksum manifests before
   applying additive migrations 005–008 and verifies graph, events, checkpoint
   and gap contracts, including the Cloud shared engine variant.
2. Verify actual provider network identity and explicit linked chain evidence
   before enabling the existing collector/campaign flags. Independent heads,
   identical addresses and time proximity cannot create a verified campaign.
3. Collect at least 24 hours for the exact deployed revision. Hourly snapshots
   alone cannot prove continuous coverage. Include queue/lag histories, source
   coverage/gaps, provider disagreement, restart/reorg recovery, a real archive
   restore, measured MTTD/MTTR and actual cost per event. Reuse the existing
   fail-closed `GlobalRadarProductionReadinessPolicyV1` gate.
4. npm registry publication and a release tag require actual publisher/tag
   evidence. Packaging artifacts and a passing CI run are not publication.
5. Optional automatic response still requires a production forwarder, authenticated
   runtime identity and independently collected effects. The observer remains
   read-only and cannot execute transactions.

## Rollback

Disable `KOSCHEI_GLOBAL_CAMPAIGN_RUNTIME_ENABLED` to stop new campaign handoffs;
retain the outbox and receipts for recovery. Roll back application code to
`176ac2163002e21ceb22f58e0eebf2e693731019` if needed. Migration 132 is additive
and must remain in place; no rollback deletes evidence. To suspend external export,
set `KOSCHEI_RADAR_ARCHIVE_EXPORT_SINK=disabled` and redeploy; the existing archive
backlog guard remains enforced. Do not reduce the retention window or delete old
evidence to shrink storage before a real destination restore has been verified.

ClickHouse design follows `insert-async-small-batches` (`async_insert=1` and
`wait_for_async_insert=1`), `schema-pk-filter-on-orderby` for bounded graph reads,
and `agent-query-safety` for live diagnostics. PostgreSQL claims use the
`lock-skip-locked` queue pattern and partial pending/lease indexes.
