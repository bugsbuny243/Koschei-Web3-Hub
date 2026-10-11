# Koschei Web3 Hub — ARVIS six-radar code audit (2026-10-11)

Scope: **Koschei Web3 Hub only**. Repository code review, not a production deployment or proof of live provider connectivity.

| Area | Existing evidence in repository | Immediate gap / next acceptance |
| --- | --- | --- |
| Token Radar (Solana) | `internal/handlers/security_radar_exposure.go`: authority, holder concentration, Token-2022, sniper timing, program relation and liquidity sections | Preserve the Solana live core; require transaction-enriched mint evidence, tested stream persistence and provider-backed findings |
| Wallet Radar | `internal/networktarget/evm_probe.go`: EIP-7702 delegation observation; `internal/radaradapter/tron_permissions.go`: Tron permission parsing | Connect read-only collectors to actual customer scan path; verify native chain identity and permission changes; no compromise verdict from observation alone |
| Contract Radar | EVM bytecode/chain-ID probing, existing ERC-1967 and authority analysis paths | Golden fixtures for upgrade/admin detection and independent critical confirmation |
| Flow Radar | `internal/services/security_radar_graph.go`: nodes/edges persisted by verdict | Trace cross-chain provenance; avoid equating related addresses with a proven illicit actor |
| DeFi Radar | `internal/handlers/liquidity_radar.go`: request-based risk score and emergency webhook dispatch | Client-provided reserve drop and liquidity amounts are **not** chain-verified. Reject invalid values now; require pool-state evidence before verified liquidity-loss claims |
| Phishing Radar | `public/address-poisoning-shield.html`, preflight and customer scan paths | Verify executable signature-request analysis and frontend-origin evidence before claiming live protection |

## Shared execution boundary

`internal/services/universal_adapter_registry.go` requires installed runtime adapters; `universal_adapter_execution.go` distinguishes declared profiles from executed collectors; `radar_adapter_bridge.go` validates evidence before mapping into `IntelligenceEvidence`. A declared network is not a successful scan.

## Network rollout

1. Tron: account permissions and TRC-20 USDT flows, with native provenance.
2. Ethereum: EIP-7702, approvals, proxy/admin authority.
3. Base, Arbitrum, Polygon, BNB, Optimism: reuse common EVM collection with network-specific chain IDs.
4. TON: jetton identity and asynchronous message traces.
5. Avalanche, Cosmos/IBC, Stellar: native chain-specific adapters and fixtures.

## Acceptance gates

- Missing data, provider failure and unsupported network => UNKNOWN, never SAFE.
- A risk score from request fields is a **heuristic**, not verified on-chain loss.
- Critical findings require reproducible block/slot/ledger identity, raw-response hash, provider provenance and independent confirmation.
- Do not promote cross-chain verdict authority without explicit review and regression tests.
- Run Go 1.26.9 gofmt, go vet and go test; retain the change in a draft PR until CI passes.
