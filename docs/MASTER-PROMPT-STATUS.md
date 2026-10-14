# Multi-chain radar delivery status — 2026-10-13

This change implements read-only collection behind the existing `radaradapter.Adapter` and `services.AdaptRadarEvidence` contracts. It does not replace the Solana engine or authorize a new grading engine. Every new investigation returns `verdict: withhold`, `grade_issued: false`, raw-response SHA-256, observation time, network identity, provider host and a height/context reference.

## Delivered code

| Priority | Scope | Implemented | Boundaries |
|---|---|---|---|
| 1 | Tron | Base58Check/hex, pinned mainnet genesis, confirmed permission state, numeric/named enums, weighted thresholds, active operation masks, recovery authority, two-host comparison, failover, contract origin, TRC-20 constants, recent permission changes and bounded indexed token events | External control is not proof of fraud. Snapshot reads follow the confirmed head; they are not archival state proofs. No private-key possession claim. |
| 2 | EVM, 14 chain IDs | EIP-55, chain identity, block-hash-pinned code/storage/calls, 7702 designator and delegate hash, source-pinned sweeper registry, EIP-1967 implementation/admin/beacon, UUPS and Diamond interfaces, implementation code hash and two-block control/code comparisons, owner/Safe-compatible/timelock interfaces, bounded ERC-20/NFT/Permit2 permissions, governance events, transaction/receipt authorization binding | Interface compatibility is not verified Safe code identity. Discovery covers 512 blocks with pair caps. Unknown historical approvals and off-chain signatures remain unknown. No unsupported RPC fallback to latest. |
| 3 | TON | Raw/friendly address normalization, workchain/testnet checks, mainnet identity, account cell hashes, indexed jetton-master state, pinned canonical master plus `get_wallet_address` and `get_wallet_data` binding, trace completeness/failure checks | Indexed completeness is not an independent full message-tree proof or economic settlement verdict. Getters are sequential; no atomic historical claim. |
| 4 | Cosmos/IBC | Hub, Osmosis, Celestia, Injective, Sei native; HRP/checksum, chain identity, outgoing authz/feegrant, governance proposals, IBC channel/client state | 100-entry window; no exhaustive grant history, packet reconciliation or bridge solvency verdict. |
| 5 | Stellar | RPC network passphrase, G addresses, account/trustline XDR keys and controls, signer weights/thresholds, issuer/trustline flags | Unsupported XDR stays unknown. No Horizon dependency. Hubble/archive retrieval is not implemented. |
| Cross-chain | Frontend/signature surface | Configured origin only, TLS leaf/chain fingerprints, HTML/JS SHA-256 versus operator baseline, configured DNS/CT observations and origin-bound captured wallet requests | No arbitrary browser execution or wallet interception. Captures must be supplied by an operator-controlled endpoint. Baseline drift is an observation, not proof of compromise. |
| Runtime | API, UI, operations | Owner capabilities/investigate, Professional investigation route, request budgets, host pacing/cooldown/failover, bounded responses, read-only method allowlist, structured telemetry, owner ARVIS panel, generated OpenAPI/matrix and CI checks | Configuration status is distinct from live health. In-process daily budgets reset with process restart and are not distributed billing quotas. |

## Not claimed complete

The catalog explicitly labels Hyperliquid core, XRPL, Cardano, Polkadot, NEAR, Litecoin, Dogecoin, BCH and Starknet `declared_not_executable`. The roadmap ranked these as deferred. Existing Solana, Bitcoin, Sui and Aptos routes remain separate.

Full archival allowance enumeration, exhaustive proxy change timelines, bytecode/source verification, exhaustive Safe code identity, bridge mint/burn reconciliation, oracle comparisons, timelock bypass proofs, three-provider network health, live MEV, agent runtime authority mapping, DID/VC verification and PQ migration inventories remain outside this collector delivery. They must not be represented by fabricated scores or a claimed completed product.

No paid subscription was purchased and no production provider key/configuration was installed by this change. Live chain/provider smoke tests and deployment promotion remain unverified. A registry entry marked executable means offline fixture conformance, not production readiness. The operator UI exposes this distinction.

## Patch and audit reconciliation

The prerequisite PR #1349 applies the supplied Tron enum/identity/evidence/route-contract fixes and removes the three request-scored routes. The current closeout changes are merged, retaining the stricter mint-owner provenance gate and shared-dashboard fail-closed behavior.

The October 6 deadcode attachment is a historical snapshot at `9a19ed7`, not a deletion instruction for the current tree. Its 215 unreachable and 353 test-only functions include isolated products, compatibility wrappers and dormant defense code. No bulk deletion was performed. The actionable public-surface issues are closed: quarantined routes remain unregistered; legacy forms no longer invoke them; static middleware denies test/fixture paths and standalone `.test.js/.test.cjs/.test.mjs` files. `/api/wallet/score` remains a registered Professional compatibility route. The legacy owner telemetry panel now calls the registered `/api/owner/token-telemetry` contract and reads its real field names; missing observations remain unavailable. `/api/public/tool-prices` is registered on current main, so that item in the October 6 snapshot is stale.

## Verification commands

From `koschei/api`:

```
go test ./...
go vet ./...
go run ./cmd/openapi-gen -check
go run ./cmd/radar-capability-gen -check
node --test public/js/arvis-premium-contract.test.cjs public/js/impact-contract.test.cjs
```

Fixtures are explicitly synthetic and cover schema, identity, binding, checksum, omitted evidence, wrong-chain responses, critical confirmation, transport budgets and no-grade behavior. Database integration suites still require their configured database environment; local skips are not production validation.

## October 14 follow-up

Added `cmd/radar-smoke` with explicit expected-relation assertions and sanitized provenance receipts. Missing provider configuration or evidence cannot pass. Tron V1 token events now preserve uint values and distinguish historical approval events from current permissions. Fixed the gofmt issue reported by both Sensitive Static Probe CI and Release Gates Verification; their replacement runs must pass before promotion. API Required CI and CodeQL passed on the preceding commit.

## EVM control history follow-up

Contract-code subjects now compare EIP-1967 implementation/admin/beacon slots with a snapshot up to 512 blocks earlier. Implementation bytecode is read at both pinned block hashes and compared independently of its address. These observations flow through the existing owner/Professional evidence endpoint, owner evidence drawer, request-count telemetry and WITHHOLD contract.

The comparison adds at most eight provider calls. Both block endpoints are checked again before comparison rows are emitted. Missing archive state, unsupported EIP-1898, invalid block/address/code encodings or a changed endpoint produce explicit limitations. Existing single-block observations remain bounded to their own block hashes. No fallback to latest state, safety grade, upgrade transaction attribution or complete historical claim is introduced. Equal endpoints cannot rule out intermediate upgrades; beacon implementation histories and non-EIP-1967 paths remain outside this comparison.

Protocol references: https://eips.ethereum.org/EIPS/eip-1967 and https://eips.ethereum.org/EIPS/eip-1898.
