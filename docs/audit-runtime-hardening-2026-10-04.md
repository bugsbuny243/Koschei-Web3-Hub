# Audit runtime hardening — 2026-10-04

This change closes exposed probe/webhook entry points and connects compact
Global Campaign evidence to a bounded runtime and owner surface. It does not
promote production response authority or claim continuous chain coverage.

## Entry points and workers

- `/fabric/networks/probe/intelligence` requires existing owner authentication
  before RPC and persistence. Both probe routes share PostgreSQL budgets when
  configured: 10/minute per client and 120/minute per application. Each route
  permits four simultaneous probes per process. Stateless deployments use
  the bounded memory limiter, so their budgets are per process.
- Telegram and WhatsApp POST webhooks reject an absent verification secret.
  Telegram requires a positive update ID and durable provider-event
  deduplication. This prevents ordinary replays, not every partial-delivery failure.
- The customer webhook worker starts through runtime-role bootstrap, records
  operational health, and is cancelled and awaited on shutdown. Its observation
  count measures attempted deliveries, not successful receipts.
- TradePI workers start explicitly for `combined`/`worker` roles. The public demo
  has its own memory-only fixture service and cannot write customer state.
- Public JSON and HTML health views replace provider error details with a stable
  error class. `/health` describes configured PostgreSQL versus stateless
  persistence; it is not a database readiness probe.
- Base58 identities remain case-sensitive. Only 40-digit EVM hexadecimal
  addresses receive case normalization in the public scan request guard.
- ClickHouse checkpoint/gap reads carry 10-second, one-million-row and 256-MiB
  read limits. No live ClickHouse performance result is asserted.

## Campaign integration

Set `KOSCHEI_GLOBAL_CAMPAIGN_RUNTIME_ENABLED=1` only after applying application
migrations through `129_global_campaign_runtime.sql` and enabling verified
graph storage with `KOSCHEI_GLOBAL_RADAR_CLICKHOUSE_ENABLED=1`.
`APP_DATABASE_URL` is required. The default remains off; invalid enabled
configuration fails startup.

The sink writes the canonical graph to ClickHouse first, then enqueues compact
references and C6/C7 projections in PostgreSQL. Only connected components with
at least two subjects and a canonical VERIFIED relation are candidates.
Independent head observations and unverified relationships create no campaigns.
Bounds: 128 observations, 256 relation/bridge records, 16 components per input,
200 KB compact JSON per component and 10,000 pending jobs across producers.
Overflow fails closed.

Worker roles acquire a process-specific owner and monotonic fencing token.
They renew the lease each bounded cycle and process at most five jobs. The
lease row stays locked in the transaction that materializes/merges a campaign,
stores its C6/C7/Fabric/command projection and acknowledges the source.
Generation survives release; a stale token cannot acknowledge work. Invalid
handoffs and deterministic evidence conflicts are quarantined; transient
database failures remain pending.

The owner console links to `/owner/campaigns`; its protected data endpoint is
`GET /api/owner/campaigns`. It displays up to 25 current revision projections,
source hashes, missing evidence, processing time and queue counts. Campaign
evidence accumulates across merges; temporal/bridge details cover the latest
processed source window. A newer revision without a matching runtime projection
is omitted. Capability metadata and runtime health expose the activation gate.

## Limits and acceptance

- Graph grouping supplies no threat-family source adapter, incident action,
  production forwarder, authorization or independent effect collector.
  Emerging campaigns require operator classification. Missing threat inputs
  and unexecuted responses remain explicit. Automatic response stays disabled.
- Head/probe collectors usually emit independent observations. This integration
  consumes verified linked snapshots supplied by adapters; it does not invent
  relationships or deploy a complete cross-network campaign collector.
- ClickHouse and PostgreSQL have no distributed transaction. A crash between
  graph write and enqueue requires canonical snapshot replay. Source digests
  make ordinary replay idempotent; reconciliation and retention remain open.
- Owner count queries have an HTTP deadline. Accumulated queue history needs
  measured retention/aggregation before scale-out. A second worker may report
  lease-busy; that is not evidence that the active worker lost its lease.
- `backendFrontendParity` remains the policy requirement. A separate
  `backendFrontendParityVerified=false` and repository source distinguish that
  requirement from verified deployment parity.
- Regression tests cover secret absence, owner-before-RPC authorization,
  probe quota/concurrency, public-error redaction, persistence reporting,
  demo isolation, cancellation, replay-safe derivation and corrupt references.
  PostgreSQL acceptance covers materialization/ack atomicity, read-back, replay
  and same-owner fencing. Native PostgreSQL 17 multi-connection acceptance is
  added in `.github/workflows/global-campaign-runtime.yml`. Adding a workflow
  does not prove an unpushed revision passed CI.

Customer verifiers and Token-2022/webhook copy now match the existing
Professional entitlement policy. Paid access is not relaxed.
