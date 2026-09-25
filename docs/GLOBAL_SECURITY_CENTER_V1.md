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

1. Continuous transaction/event ingest for explicitly configured non-Solana networks.
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

A separate continuous head-ingest worker is also available behind:

- `KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED=1`;
- `KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_NETWORKS`;
- `KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_INTERVAL_SECONDS`.

It requires the canonical Global Radar event ClickHouse ledger. The first version supports Ethereum, Base, Arbitrum, Optimism, Polygon, BNB Smart Chain, Avalanche C-Chain and Bitcoin mainnet. Each target verifies its native network boundary, preserves source-response SHA-256 evidence, and emits a canonical `block` event only when the observed head height advances within the running process.

This is deliberately not described as transaction-firehose ingestion. It observes chain heads. Transaction/log/mempool ingestion, durable cross-restart cursors and reorg-aware block-hash lineage remain separate follow-on work.
