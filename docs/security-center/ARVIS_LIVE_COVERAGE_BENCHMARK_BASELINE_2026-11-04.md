# ARVIS live coverage benchmark — baseline (2026-11-04)

Tracking: [#1519](https://github.com/bugsbuny243/Koschei-Web3-Hub/issues/1519)

## Source and truth boundary

Source: `docs/CHAIN_CAPABILITY_MATRIX.md` at main commit `675b0150b72f08c93ef3a6015200fe716de391ae`. This is an **inventory**, not a live RPC test or a competitor performance result. `executable` means offline fixture conformance, not deployed, healthy or complete. The `offchain` profile is not a blockchain.

| Catalog state | Count | Networks / profiles |
| --- | ---: | --- |
| executable (offline fixture) | 24 | arbitrum-mainnet, avalanche-mainnet, base-mainnet, berachain-mainnet, bnb-mainnet, celestia-mainnet, cosmoshub-mainnet, ethereum-mainnet, hyperevm-mainnet, injective-mainnet, linea-mainnet, monad-mainnet, offchain, optimism-mainnet, osmosis-mainnet, polygon-mainnet, scroll-mainnet, sei-evm-mainnet, sei-mainnet, solana-mainnet, stellar-mainnet, ton-mainnet, tron-mainnet, zksync-mainnet |
| existing_separate_route | 3 | aptos-mainnet, bitcoin-mainnet, sui-mainnet |
| declared_not_executable | 9 | bitcoincash-mainnet, cardano-mainnet, dogecoin-mainnet, hyperliquid-core, litecoin-mainnet, near-mainnet, polkadot-mainnet, starknet-mainnet, xrpl-mainnet |
| **Total** | **36** | 35 network entries plus one offchain profile |

## Runtime evidence, separately observed

Railway project `exciting-spontaneity`: `Koschei-Web3-Hub` production and test environment both online, 1/1 running replicas each, last deployment `675b0150b72f08c93ef3a6015200fe716de391ae` successful, zero recent deployment failures in reported window. This is **service availability**, not per-network scan proof. Test environment has staged, unapplied background-worker changes; leave untouched.

## Test protocol

1. Obtain operator-approved, public, immutable target manifest (1–10 cases per invocation) with chain, target kind and required relations. Do not store secrets or customer identifiers.
2. Record runtime provider configuration **presence only**, feature flags and deployed commit; never expose provider credentials.
3. Run `go run ./cmd/radar-smoke -live -manifest <approved.json>` from `koschei/api` with the approved `RADAR_PROVIDERS_JSON` runtime environment. Tool uses bounded read-only calls, 45-second timeout per case, and withholds grades.
4. Preserve each receipt: network, target, pass/fail, collector status, provider request count, required/missing relations, raw-response SHA-256, native chain identity, block/slot and observation time. A pass validates only required relations, not full scan completeness.
5. Test missing provider, incorrect chain, RPC failure, unavailable history and inconsistent provenance. These must not produce a safety claim.
6. For Solana authoritative ARVIS, independently verify investigation arm completion, canonical signed final, durable receipt and customer route. Do not infer this from observe-only `radar-smoke`.
7. Benchmark GoPlus, RugCheck, Honeypot.is and Forta only on overlapping supported chains, public samples and matching time windows; document unavailable cases and independent ground truth before computing precision/recall.

## Current measurement ledger

| Measurement | Result | Evidence |
| --- | --- | --- |
| Catalog coverage | 24 offline-executable, 3 separate route, 9 declared-only | Repository capability matrix |
| Production service health | Online, 1/1 replicas | Railway environment status, 2026-11-04 |
| Per-chain live provider success | **NOT TESTED** | Approved manifest and runtime execution required |
| ARVIS 14-arm current completion | **NOT TESTED** | Historical report is not a current measurement |
| Competitor matched-sample accuracy | **NOT TESTED** | No shared labeled corpus collected |
| Live safety verdict correctness | **NOT TESTED** | Requires canonical signed verdict and independent ground truth |

**Do not** report these untested measurements as successful or claim Koschei outperforms competitors.

## Follow-up

Track executable live tests, reproducible evidence receipts and benchmark corpus under #1519. Never enable staged worker settings or send transactions as a prerequisite for read-only acceptance.
