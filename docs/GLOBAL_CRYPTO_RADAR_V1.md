# Koschei Global Crypto Radar v1

Status: first implementation slice

## Goal

Build a chain-neutral observation and intelligence plane without flattening different
blockchain security models into one score.

The radar must connect evidence across:

- chains and execution environments
- wallets / accounts / addresses
- transactions
- contracts / programs
- validators / nodes / miners
- bridges and cross-chain flow
- liquidity and market structure
- threat hypotheses
- deterministic evidence-backed verdicts where an authoritative verdict engine exists

## Non-negotiable evidence boundary

Repository capability is not deployment health.

A network may appear in the catalog because address parsing, identity validation,
telemetry, or a read-only probe exists. That must not be represented as complete
historical collection, live availability, production acceptance, or verdict authority.

The Global Radar contract therefore exposes each capability independently.

Schema:

`koschei.global-radar-capability.v1`

Operator endpoint:

`GET /fabric/networks/radar`

## Current network capability policy

### Solana

ARVIS remains the authoritative evidence and verdict engine. Existing transaction,
state and program evidence paths are represented as existing capabilities.

### EVM networks

Ethereum, Base, Arbitrum, Optimism, Polygon, BNB Smart Chain and Avalanche C-Chain
have repository probe paths. Their capability contract remains `probe_ready` where
appropriate and does not grant ARVIS verdict authority.

### Bitcoin

Bitcoin has address/network resolution, transaction probing and node / proof-of-work
telemetry paths. Contract/program analysis is explicitly `not_applicable`; verdict
authority remains disconnected.

### Move networks

Sui and Aptos identity classification exists, but transaction/state/program dispatch
remains fail-closed. Identity readiness must never silently promote them to full radar
coverage.

## Implemented in this slice

### Capability contract

`koschei.global-radar-capability.v1` exposes radar capabilities independently per
network. It is repository-state metadata, not deployment proof.

### Observation envelope

`koschei.global-radar-observation.v1` wraps already-normalized
`IntelligenceEvidence` without creating a verdict. The builder rejects subject,
chain-family, chain or network mismatches and accepts only OBSERVED or VERIFIED
evidence with a concrete source and observation time.

The existing EVM and Bitcoin read-only intelligence probe route now adds a
`radar_observation` projection while keeping the legacy flat probe response
unchanged.

### Durable relation edge

`koschei.global-radar-relation.v1` creates stable graph edge identifiers.

A relation is VERIFIED only when one concrete evidence record explicitly binds both
endpoint subject IDs. Cross-network relations additionally require that the binding
record names the matching source and target networks. Independent observations on
two chains are not treated as proof that those subjects are related.

### Bridge transfer linker

`koschei.global-radar-bridge-link.v1` requires two distinct transaction observations
on distinct networks. Both endpoint evidence records must independently expose the
same explicit `bridge_protocol` and `bridge_transfer_id`, and both sides must carry
transaction hashes. Timing, amount similarity, token symbols and address similarity
cannot create a link.

A verified bridge link proves only that the two observed transactions satisfy the
explicit transfer-link contract. It does not claim common actor identity or intent.

### Network telemetry projection

Normalized `koschei.network-telemetry.v1` records can now be projected into
evidence-only Global Radar node observations. Validator stake share, PoW contribution,
client family, ASN and coarse country evidence are preserved when present; missing
dimensions remain explicit `missing_evidence` and are never converted into risk.

Solana validator telemetry can be projected as one Radar observation per validator.
No synthetic decentralization score is produced.

### Durable probe persistence

The existing `/fabric/networks/probe/intelligence` path can now persist its canonical one-observation Global Radar snapshot into ClickHouse when `KOSCHEI_GLOBAL_RADAR_CLICKHOUSE_ENABLED=1`. The feature is off by default, validates the ClickHouse graph schema at API startup, and fails closed on persistence failure instead of claiming a durable observation that was not stored. The legacy flat probe route is unchanged.

This path remains useful for explicit operator probes.

### Bounded background network telemetry

An additional opt-in worker can continuously persist network-health evidence for explicitly selected networks. It is disabled by default, requires the ClickHouse Global Radar sink, requires an explicit network allowlist, and clamps collection cadence to 5–60 minutes.

The first background slice covers EVM execution-node telemetry, optional Ethereum Beacon telemetry, and Bitcoin Core plus PoW network telemetry. Each collector verifies its native chain/network boundary before projection. EVM execution-node, Ethereum Beacon, Bitcoin Core and Bitcoin PoW collectors now preserve native response-byte digests and can emit canonical Global Radar events into the optional event ledger. Derived Beacon state that combines multiple responses is not forced into a single-source fact; only directly source-bound facts enter the event contract. Failures on one target are reported as partial-cycle errors and never become safety conclusions or synthetic verdicts.

This is still telemetry collection rather than multi-chain transaction firehose ingestion. ARVIS remains the only connected verdict authority.

### Owner-only persisted graph retrieval

The existing ClickHouse graph reader is now exposed through `GET /api/owner/radar/global/records` behind the repository's existing owner authentication boundary. The canonical event ledger is separately readable through `GET /api/owner/radar/global/events` under the same owner boundary. The route requires a registered `network`, defaults to a 24-hour half-open window, accepts optional `subject_id` and `record_type`, and clamps the HTTP surface to at most 1,000 rows even though the lower ClickHouse reader retains its stricter 31-day / 5,000-row hard contract and scan caps.

Returned rows are historical persisted evidence, not a claim about current chain state. The graph reader rechecks requested network/subject/type/time boundaries, validates payload JSON and recomputes each stored payload SHA-256 before the owner route can return it. The event reader also decodes every stored payload back into `koschei.global-radar-event.v1`, re-runs canonical event verification, and checks row identity, source digests, evidence state and observation time against the verified payload. The Fabric capability surface marks operator UI visualization as pending rather than claiming a completed graph frontend.

### Snapshot contract

`koschei.global-radar-snapshot.v1` assembles observations, relation edges, verified
bridge links and ARVIS verdict references into one self-consistent machine-readable
snapshot. Verified relations and verdict references are rejected when their evidence
references are outside the snapshot.

Coverage reports counts for networks, subjects, observations, observed/verified
evidence, relations, cross-network relations, bridge links, verdict references and
missing-evidence items. `risk_score_produced` is always false.

### Liquidity observations

Provider-backed Solana market snapshots can be compared as descriptive liquidity
events. The projection preserves liquidity/volume/price deltas and provider
limitations, but explicitly sets `market_data_can_issue_verdict=false` and
`rug_or_drain_claim=false`.

### ARVIS verdict references

`koschei.global-radar-verdict-reference.v1` preserves an existing evidence-linked,
source-marked-signed ARVIS deterministic verdict as an authoritative graph reference.

Global Radar does not re-grade or re-sign the verdict. The legacy projection remains explicitly unverified and reports `signature_verification=not_reverified_by_global_radar`.

An additive server-owned trusted-key registry contract can now independently reverify the same canonical ARVIS v1 payload with Ed25519. Registry entries are selected by `key_id`, require canonical unpadded base64url 32-byte public keys, and support bounded `valid_from`, `valid_until`, and `revoked_at` lifecycle controls evaluated against the verdict's authenticated `generated_at`. Only callers that explicitly supply this registry may receive `signature_verification=verified_ed25519_trusted_registry`; unknown, malformed, expired, revoked, tampered, or mismatched signatures fail closed.

## Security-center integration update — 2026-09-25

The Global Radar is now registered inside `koschei.security-center-capability.v1` and exposed through `GET /fabric/security-center` and `GET /fabric/security-center/capabilities`. The owner control center now has a persisted graph/event timeline reader over the existing owner-only ClickHouse APIs. This UI is explicitly historical evidence only and does not represent current chain state.

The repository also preserves previously unmounted frontend runtimes through an explicit asset registry instead of deleting them or blindly co-executing incompatible versions.

## Next implementation slices

1. Extend the new durable multi-network block/transaction/log ingest with bounded automatic reorg recovery and multi-provider lineage confirmation without changing ARVIS decision authority.
2. Add an operator graph/timeline visualization over the owner-only graph and canonical event retrieval APIs; both bounded authenticated JSON read contracts now exist.
3. Connect bridge-specific live adapters only where both chain-side transfer identities can be independently anchored.
4. Wire the trusted verdict-key registry into production verdict-reference creation; the verification contract now exists but the legacy unverified projection remains the default for callers that do not supply server-owned trust material.
5. Expand live node telemetry persistence for EVM beacon/execution and Bitcoin Core/PoW collectors without inventing missing geography or client identity.

## Product principle

The objective is not to claim the largest number of supported chains.

The objective is to build the strongest evidence graph across chains while every
unsupported or unverified capability remains explicit and fail-closed.


### Continuous durable multi-network block ingest

The opt-in worker now persists a cross-restart cursor in the migration-007 ClickHouse checkpoint ledger rather than relying on process memory. It is separate from the slower network-health telemetry worker and is controlled by `KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED`, an explicit network allowlist, a bounded 5–300 second cadence, a bounded blocks-per-cycle catch-up limit, and a bounded events-per-block limit.

For EVM targets the adapter verifies chain ID, fetches the exact block identity with `eth_getBlockByNumber`, binds transaction identities from that block, and fetches logs with `eth_getLogs(blockHash=...)`. It emits canonical `block`, `transaction`, and `log` events whose source-response bytes are SHA-256 bound. Bitcoin mainnet similarly binds `getblockhash` and `getblock` evidence and emits canonical block plus transaction-identity events.

Cursor advancement happens only after the event batch is accepted. The next block's parent hash must match the stored block hash; the same height must continue resolving to the stored hash. A mismatch writes a durable `reorg_observed` state and freezes the stream instead of advancing on ambiguous lineage.

These records remain observations from configured endpoints. They do not claim finality, multi-provider canonicality, full transaction bodies, mempool visibility, automatic reorg rewind, actor identity, intent or chain safety. ARVIS remains the only connected verdict authority.

### Runtime health

`GET /fabric/security-center/runtime-health` exposes `koschei.runtime-health.v1` for configured storage, telemetry and head-ingest components. Runtime health is operational metadata only and is kept separate from ARVIS risk/verdict authority.
