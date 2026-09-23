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

## Next implementation slices

1. Normalize live observations into one chain-neutral evidence envelope while
   preserving chain-native semantics.
2. Add durable entity and relation identifiers for wallet/account/contract/bridge
   graph edges.
3. Add cross-chain correlation records that point to concrete source evidence instead
   of probabilistic attribution.
4. Add live node/validator/miner telemetry snapshots and concentration dimensions.
5. Add bridge transfer observation and source/destination transaction linkage.
6. Add liquidity-event observations without letting market data issue security
   verdicts.
7. Project ARVIS signed deterministic verdicts into the global graph without creating
   a second verdict engine.
8. Add public/operator coverage views that distinguish implemented, configured,
   observed and verified states.

## Product principle

The objective is not to claim the largest number of supported chains.

The objective is to build the strongest evidence graph across chains while every
unsupported or unverified capability remains explicit and fail-closed.
