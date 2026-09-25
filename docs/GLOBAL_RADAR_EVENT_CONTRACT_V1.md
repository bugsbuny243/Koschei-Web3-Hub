# Koschei Global Radar Event Contract v1

Date: 2026-09-23

## Purpose

`koschei.global-radar-event.v1` is the chain-independent observation envelope for the Global Crypto Radar.

It sits **before** ARVIS decision logic. Chain adapters use it to move observed facts into the common radar plane without turning transport code into security authority.

## Supported event classes

- `block`
- `transaction`
- `log`
- `asset`
- `contract`
- `bridge`
- `liquidity`
- `network_health`

The contract currently accepts registered Solana, EVM, Bitcoin/UTXO and Move-family networks through the shared network catalog.

## Required trust properties

1. Every observed or verified event must carry at least one SHA-256 source digest.
2. `VERIFIED` is not inferred from the producer name; the adapter must explicitly provide verified evidence.
3. Missing evidence is represented as `UNAVAILABLE` rather than converted into a benign result.
4. Network IDs must already exist in the network registry.
5. Native chain references remain attached to the event, but the core envelope does not depend on a Solana account model, EVM calldata model, Bitcoin UTXO model or Move object model.
6. Event digests are deterministic: source digests, native references and fact ordering are canonicalized before hashing.
7. Free-form opaque JSON is intentionally excluded from the signed/digested core. Large native payloads remain external evidence artifacts and are bound by SHA-256.

## Contract shape

```text
schema_version
producer
kind
network_id
subject_kind
subject_id
observed_at_unix_ms
evidence_state
native_refs[]
source_digests_sha256[]
facts[]
event_sha256
```

## Relationship to existing SecurityEvidenceEvent

The repository already has `koschei.security-evidence/v1`, which binds evidence findings and supports Ed25519 producer authentication.

The Global Radar Event is not a replacement for it:

```text
chain adapter
    -> global radar event
    -> correlation / graph / normalization
    -> security evidence event
    -> ARVIS deterministic decision
    -> signed verdict
```

A future adapter will promote correlated radar events into `koschei.security-evidence/v1` only when the evidence contract requirements are satisfied.

## Durable event ledger

ClickHouse migration `006_global_radar_events.sql` defines an append-first ledger keyed by the canonical `event_sha256`. Exact delivery replay converges through `ReplacingMergeTree(ingest_version)`, while distinct canonical event digests remain separate historical records.

The writer re-verifies every event digest before any network write, canonicalizes the event again, stores the complete canonical event JSON, and binds those stored bytes with a separate payload SHA-256. The ledger does not create a risk grade or promote evidence state.

The EVM, Bitcoin, Sui and Aptos intelligence probes can now emit this envelope directly from exact native response-byte digests. EVM binds `eth_chainId` and `eth_getCode` separately; Bitcoin binds mainnet genesis verification and address activity separately. Optional persistence is controlled independently by `KOSCHEI_GLOBAL_RADAR_EVENT_CLICKHOUSE_ENABLED=1`; startup verifies migration 006 for the canonical event ledger and migration 007 for the durable ingest-checkpoint ledger before the sink is accepted.

Graph snapshot persistence and event-ledger persistence are separate replay-convergent writes rather than a distributed transaction. If either configured sink fails, the request fails closed; retrying the same canonical evidence converges by stable snapshot/event identity.

A producer must already possess the real source digest required by the event contract; normalized probe output is not retroactively relabeled as raw source evidence.

## Durable block-ingest cursor and lineage

`koschei.global-radar-ingest-checkpoint.v1` is an operational cursor contract stored in ClickHouse migration `007_global_radar_ingest_checkpoints.sql`. A cursor records the network/stream identity, height, block hash, parent hash, the canonical block-event digest that justified advancement, observation time, and one of `canonical`, `reorg_observed`, or `rewind`.

Cursor advancement is deliberately ordered after canonical event persistence. If event persistence succeeds and checkpoint persistence fails, the next cycle may replay the same source evidence; the cursor is never allowed to claim a block that was not first written to the event ledger. Exact checkpoint replay converges while distinct lineage states remain queryable historical records.

The continuous EVM adapter now emits one `block` event plus transaction-identity and block-scoped `log` events from `eth_getBlockByNumber` and `eth_getLogs(blockHash=...)`. Bitcoin emits one `block` event plus transaction-identity events from `getblockhash` and `getblock`. Native response bytes remain bound by SHA-256; the adapters do not claim full transaction bodies, mempool completeness, finality, intent, or safety.

When a next block's parent hash does not match the durable cursor, or the same height resolves to a different block hash, the worker writes a durable `reorg_observed` cursor state and stops advancing that stream. Automatic rewind is intentionally not performed in this slice.


## Next step

Build source adapters that emit this envelope from:

- Solana live stream observations;
- EVM block, transaction-identity, log and contract probes. The continuous block adapter preserves exact `eth_chainId`, block and block-scoped log response-byte SHA-256 values; the address/code probe separately preserves exact `eth_chainId` and `eth_getCode` response-byte SHA-256 values;
- Bitcoin address/network observations. The Esplora address probe now preserves separate exact response-byte SHA-256 values for mainnet genesis verification and address activity;
- Sui and Aptos identity/network observations. Both probes now retain the exact bounded identity-response SHA-256 needed for native provenance.

After that, the same event stream becomes the input to the cross-chain entity graph.
