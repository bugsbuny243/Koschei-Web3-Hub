# Koschei Global Crypto Radar — Phase 1

Date: 2026-09-23

## Goal

Start the global radar as an additive, chain-independent control plane without weakening the existing ARVIS evidence contract.

The first implementation slice does **not** claim that Koschei is already collecting every chain in real time. It creates a truthful global snapshot over the network adapters already present in the repository and keeps implementation capability, deployment configuration, live availability, and verified evidence as separate states.

## Phase 1 shipped in this branch

- New `koschei.global-radar.v1` snapshot contract.
- New read-only `GET /fabric/radar/global` operator endpoint.
- Global view over the current registered networks:
  - Solana
  - Ethereum
  - Base
  - Arbitrum
  - Optimism
  - Polygon
  - BNB Smart Chain
  - Avalanche C-Chain
  - Bitcoin
  - Sui
  - Aptos
- Explicit truth boundary:
  - no fabricated aggregate transaction counts;
  - no fabricated wallet counts;
  - no inferred wallet/user physical location;
  - geographic evidence is reserved for infrastructure observations with provenance;
  - missing evidence remains `unknown`;
  - adapter capability is not treated as live evidence.
- Tests cover route visibility, network coverage, fail-closed deployment status, cache policy, and mutation-method rejection.

## Architecture direction

The global radar should evolve as five separated planes:

1. **Network sensor plane**
   - one adapter per chain family;
   - chain identity verification before evidence acceptance;
   - RPC/node/provider divergence checks;
   - mempool/stream/block intake where the chain exposes it.

2. **Evidence normalization plane**
   - chain-specific facts become versioned chain-independent evidence;
   - every evidence item keeps source, observation time, subject and provenance;
   - missing data never becomes a safe result.

3. **Entity and attack graph plane**
   - wallet/account, contract/program, token/asset, pool, bridge, validator/node and protocol relations;
   - cross-chain transfer edges;
   - funding and attack-path correlation;
   - confirmed edges separated from inferred hypotheses.

4. **ARVIS decision plane**
   - deterministic rules and evidence-backed A–F verdicts;
   - signed verdicts and independent verification;
   - chain adapters transport evidence but do not own decision policy.

5. **Sentinel intelligence plane**
   - anomaly detection, campaign correlation and adversarial interpretation through versioned Fabric contracts;
   - observe/shadow first;
   - Sentinel interpretation cannot silently mint ARVIS decision authority.

## Next implementation slices

### Phase 2 — common event envelope

Create a versioned event contract for block, transaction, asset, contract/program, bridge, liquidity and network-health observations. The same envelope must work across Solana, EVM, UTXO and Move families while retaining native evidence references.

### Phase 3 — live network health aggregation

Collect and aggregate only observed node/validator/RPC telemetry. Add provider divergence, freshness, client diversity, ASN/hosting concentration and consensus-contribution evidence where the chain exposes trustworthy data.

### Phase 4 — cross-chain entity graph

Create stable entity IDs and typed graph edges for addresses, contracts/programs, assets, bridges and protocols. Inference confidence must never overwrite confirmed evidence state.

### Phase 5 — attack-path and fund-flow engine

Build source-chain -> bridge -> destination-chain continuity and reconstruct evidence-backed attack paths. Add replayable case artifacts rather than opaque risk scores.

### Phase 6 — operator UI

The globe/command-center interface consumes only the global radar contract and evidence graph. Map geography represents infrastructure observations, not assumed wallet/user location. Every headline metric must have a real data source and freshness state.

## Non-goals of Phase 1

- no claim of 24/7 coverage for all registered chains;
- no fabricated global transaction/wallet/contract totals;
- no automatic safety verdict from network registration;
- no coupling of ARVIS core policy to EVM, Bitcoin, Move or Solana-specific data models;
- no direct import of Koschei Sentinel or Koschei Lang internals.
