# Koschei Global Radar Event Contract v1

Date: 2026-09-23

## Purpose

`koschei.global-radar-event.v1` is the chain-independent observation envelope for the Global Crypto Radar.

It sits **before** ARVIS decision logic. Chain adapters use it to move observed facts into the common radar plane without turning transport code into security authority.

## Supported event classes

- `block`
- `transaction`
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

This storage path is intentionally available before automatic producers are connected. A producer must already possess the real source digest required by the event contract; normalized probe output is not retroactively relabeled as raw source evidence.

## Next step

Build source adapters that emit this envelope from:

- Solana live stream observations;
- EVM transaction and contract probes;
- Bitcoin address/network observations;
- Sui and Aptos identity/network observations.

After that, the same event stream becomes the input to the cross-chain entity graph.
