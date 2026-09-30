# Koschei Global Crypto Security Center v1

Date: 2026-09-25

Status: additive control-plane foundation

## Direction

No existing security capability is deleted merely because a page, adapter, worker or visual runtime is not currently mounted. The repository is treated as an accumulating security system. Existing behavior is preserved; new integration is additive, versioned and reversible until promoted.

The target is a federated crypto-security center that joins:

- native chain/network sensors;
- canonical source-digest events;
- evidence normalization and provenance;
- cross-network correlation and graph memory;
- deterministic ARVIS decisions where authority is actually connected;
- transaction preflight and execution assurance;
- actor-defense memory and repeat-actor correlation;
- alerting, watchlists and webhooks;
- durable Postgres/ClickHouse evidence memory;
- Koschei Sentinel through observe-first Fabric contracts;
- Koschei Lang through versioned policy/proof adapters without importing Lang internals.

## New capability registry

Schema:

`koschei.security-center-capability.v1`

Operator surfaces:

- `GET /fabric/security-center`
- `GET /fabric/security-center/capabilities`

The registry is repository capability metadata, not deployment health. A capability marked implemented can still be unavailable in a particular deployment if its database, RPC, ClickHouse, feature gate or provider configuration is absent.

The registry currently describes these layers independently:

1. Global Radar event plane.
2. Solana ARVIS evidence and deterministic verdict authority.
3. EVM / UTXO / Move native probe paths.
4. Cross-network evidence graph and bridge relations.
5. Network / validator / node telemetry.
6. Persistent actor-defense intelligence.
7. Transaction preflight and state recheck.
8. Execution assurance and defense validation.
9. Conditional Defense OS laboratory capabilities.
10. Durable evidence memory.
11. Watchlist, alert and webhook response plane.
12. Sentinel observe-first model integration.
13. Koschei Lang policy/proof integration boundary.

## Frontend/runtime wiring added in this slice

### Owner control center

`/owner-production` now mounts the already-existing additive owner modules in dependency order:

- `owner-unified-radar.js`
- `owner-defense-network.js`
- `owner-actor-distribution.js`
- `owner-command-center-v2.js`
- `owner-global-radar-v1.js`

The new Global Radar owner panel reads the already-existing owner-only persisted graph and canonical event APIs. It explicitly labels them as historical persisted evidence, not current chain state.

### Customer intelligence desk

`/scan` now exposes the already-existing EVM approval/spender-authority route as a real customer evidence surface:

- backend: `POST /api/scan/approval`
- frontend: `evm-spending-intelligence.js`

The surface reads current ERC-20 allowance plus spender authority evidence (contract code, EIP-7702 and ERC-1967 evidence where available). It does not infer approval provenance, owner intent or safety.

### Customer dashboard compatibility

`customer-command-universe-v2.js` is preserved and registered, but is not co-executed on `/dashboard`: the active Customer Workspace V2 acceptance contract explicitly retires that command-universe DOM contract. Keeping it in the inventory preserves the module without violating the current authoritative workspace runtime.

### Security-center visual/runtime inventory

The security-center surface mounts the non-authoritative presentation runtimes:

- `cipher.js`
- `koschei-security-world.js`
- `feedback-button.js`

Other preserved JS modules are registered with an explicit state rather than deleted or blindly co-executed. This prevents duplicate submit handlers, conflicting auth/session semantics or incompatible visual runtimes while keeping every module visible in the integration inventory.

## Evidence boundary

The control plane must preserve these rules:

- missing evidence remains `UNKNOWN` or `UNAVAILABLE`;
- repository capability is not deployment health;
- historical persisted evidence is not current chain state;
- a native probe is not ARVIS verdict authority;
- Sentinel observation cannot silently mutate a Web3 verdict;
- Lang remains owner of its compiler/runtime/policy/proof internals;
- cross-chain similarity is not proof of identity or common control;
- no private keys, seed phrases or provider secrets are requested or stored.

## Next hardening slices

The next implementation work should extend the same registry rather than creating parallel products:

1. Expand provider-diversity confirmation beyond two-provider block lineage into broader source diversity and operator recovery controls.
2. Live bridge adapters where both sides can bind the same native transfer identity.
3. Production trusted-key registry wiring for ARVIS verdict references.
4. Per-capability runtime health records that distinguish configured, reachable, degraded and unavailable without converting health into risk.
5. Bounded queue/backpressure metrics for every streaming collector.
6. Evidence retention tiers and cryptographic archive manifests.
7. Cross-network entity graph projections backed only by explicit relation evidence.
8. Sentinel model-evaluation telemetry in observe/shadow mode before any promotion gate.
9. Lang policy/proof adapter conformance tests at the Fabric boundary.
10. Operator incident workflow linking evidence, alerts, dossiers and response actions without giving AI explanation authority over deterministic verdicts.


## Runtime health registry and continuous head ingest — 2026-09-25

Runtime availability is now tracked separately from repository capability metadata.

Schema:

`koschei.runtime-health.v1`

Surface:

`GET /fabric/security-center/runtime-health`

The registry records only operational status and bounded counters:

- configured / disabled;
- live / degraded / unavailable / stopped;
- last success and failure timestamps;
- consecutive failure count;
- successful and failed cycle counts;
- observation count;
- network and component kind.

It does not expose provider URLs, credentials, database DSNs, signing keys or other secret material. A healthy component is not a safety verdict, and a degraded component is not evidence that a chain or asset is unsafe.

A separate durable block-ingest worker is available behind:

- `KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED=1`;
- `KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_NETWORKS`;
- `KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_INTERVAL_SECONDS`;
- bounded per-cycle block and per-block event limits.

It requires both the canonical Global Radar event ledger and the migration-007 durable checkpoint ledger. Ethereum, Base, Arbitrum, Optimism, Polygon, BNB Smart Chain and Avalanche C-Chain emit canonical block, transaction-identity and block-scoped log events. Bitcoin emits canonical block and transaction-identity events. Each source adapter verifies its native network boundary and binds native RPC response bytes by SHA-256.

The durable cursor advances only after the block event batch is accepted. Block hash and parent hash lineage are checked before progression. Optional independent confirmation can require a second configured provider to report the same height/hash/parent identity before persistence. Provider disagreement fails closed. When bounded automatic recovery is enabled, a discontinuity is first recorded as `reorg_observed`, then historical canonical checkpoints are searched backward for a stored ancestor that both providers independently match. A successful recovery writes a `rewind` checkpoint and the next cycle resumes from that ancestor. This still does not claim finality, mempool visibility, full transaction-body completeness, actor identity, intent or chain safety.


## Multi-provider lineage confirmation and bounded recovery — 2026-09-25

The durable block-ingest worker can now use a separately configured confirmation RPC for every supported EVM target and Bitcoin mainnet.

Two independent controls remain off by default:

- `KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_REQUIRE_CONFIRMATION=1` requires block identity agreement before a block event batch can be persisted.
- `KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_AUTO_REORG_RECOVERY=1` enables bounded common-ancestor recovery after a recorded lineage discontinuity.

Recovery never trusts a replacement branch merely because the primary endpoint changed. The worker loads previously stored canonical checkpoints, probes the same historical height through both primary and confirmation providers, requires matching block hash and parent hash, and only then writes a `rewind` checkpoint. The search is bounded by `KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_MAX_REORG_REWIND` and cannot exceed 64 blocks.

Provider agreement here is an operational lineage check, not a consensus-finality proof or ARVIS verdict.


## Operator incident workflow v1 — 2026-09-28

The Security Center now has an owner-only incident-response workflow rather than a read-only audit-events page.

Persistence:

- `security_incident_cases` stores bounded operator case state: severity, status, network, target and links to existing evidence/alert/dossier references.
- `security_incident_actions` is an append-only operator timeline for creation, investigation, containment, resolution, notes, severity changes and evidence-link actions.
- Migration: `123_security_center_incident_workflow.sql`.

Owner API:

- `GET|POST /api/owner/incidents`
- `GET|POST /api/owner/incidents/{id}`

Owner Control Center:

- the Security page reads both the existing security audit stream and the incident queue;
- an owner can open a case and move it through `open -> investigating -> contained -> resolved/closed`;
- linked evidence references remain references to existing source material instead of being copied into a new verdict system.

Authority boundary:

- incident status is operational response metadata only;
- resolving or closing an incident cannot change an ARVIS grade, verdict signature, evidence row or source dossier;
- AI explanation is not an incident-state authority;
- missing linked evidence is not converted into safety.

This completes the first implementation slice of the previously listed operator-incident-workflow hardening item. Future work should connect eligible deterministic alerts/dossiers to case creation suggestions without automatically declaring an incident or changing verdict authority.


## Radar flow observability v1 — 2026-09-30

The owner control plane now exposes a bounded pipeline-depth view:

- `GET /api/owner/radar/flow-metrics`
- schema: `koschei.radar-flow-metrics.v1`
- owner authentication and application persistence are required.

The endpoint measures separate persisted stages instead of treating raw collection volume as equivalent to verified evidence:

1. collected stream rows;
2. recognized rows whose module is known;
3. transaction-enriched mint evidence;
4. completed ARVIS stream-processing work;
5. signed final-verdict-engine decisions backed by verified evidence;\n6. new durable `security_alert_events` rows, measured separately from verdicts.\n
Each stage reports bounded 15-minute and 24-hour counts plus latest-observation freshness. The endpoint also reports the last-24-hour evidence-quality distribution, per-network collected-event distribution, pending/processing/stale/failed/exhausted queue state and stage-throughput ratios in basis points.

These ratios are operational throughput ratios, not cohort conversion rates. They can exceed 100% when a worker drains older backlog during the measurement window.

Truth boundary:

- no all-time hot-store scan is required by this endpoint;
- `recognized` does not mean `verified`;
- `transaction_enriched_mint` is the explicit enrichment boundary used by the current Solana stream pipeline;
- a counted verdict must be a signed `final_verdict_engine` row with verified evidence;\n- durable alert metrics count newly created deduplicated alert rows; repeated occurrences may update the same row and are not falsely presented as new alerts;\n- the metrics describe persisted pipeline rows and do not claim chain-wide or global coverage.
