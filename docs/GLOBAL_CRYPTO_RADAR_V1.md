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

## Next implementation slices

1. Add a concrete bridge-transfer linker that can emit the explicit cross-network
   binding evidence required by the relation contract.
2. Add live node/validator/miner telemetry snapshots and concentration dimensions.
3. Add liquidity-event observations without letting market data issue security
   verdicts.
4. Project ARVIS signed deterministic verdicts into the global graph without creating
   a second verdict engine.
5. Add public/operator coverage views that distinguish implemented, configured,
   observed and verified states.
6. Add durable graph persistence/query paths for observations and relation edges.

## Product principle

The objective is not to claim the largest number of supported chains.

The objective is to build the strongest evidence graph across chains while every
unsupported or unverified capability remains explicit and fail-closed.
