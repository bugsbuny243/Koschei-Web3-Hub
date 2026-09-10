# Koschei Web3 repository contract

This repository's primary product domain is **Koschei Web3**, and the Koschei Web3 security core must remain isolated and intact.

A single deployed TradePI service may also host explicitly isolated TradePI product modules under their own namespaces (for example `/agents`, `/api/agents/*`, and dedicated internal packages) when this avoids unnecessary duplicate infrastructure. These modules must not be presented as Koschei Web3 capabilities and must not import or copy ARVIS or Koschei Lang internals.

- ARVIS is a core intelligence and evidence engine inside Koschei Web3.
- Matrix belongs to Koschei Lang, not Koschei Web3.
- Koschei Sentinel is reactivated and may integrate through Koschei Fabric in **observe-first, contract-only** mode. Sentinel must not import or mutate Koschei Web3 security internals, and its paid-compute / model-promotion safety gates remain independent and fail-closed.
- Koschei Lang remains a separate project and the owner of its language, Matrix, compiler/runtime, policy, proof, and identity internals. Fabric integration must use explicit adapters/contracts; no direct production dependency on Lang internals may be introduced until compatibility and release gates are satisfied.
- Existing Koschei Web3, Koschei Lang, and Koschei Sentinel behavior must be preserved during Fabric integration. New capabilities are additive, versioned, and reversible until explicitly promoted.
- Every new backend capability exposed through Fabric must have an operator-visible frontend/status surface and telemetry before it is considered complete.
- Koschei Web3 is a Web3 Security Validation & Risk Intelligence Platform. It must not collapse into a generic wallet, token, honeypot, rug, contract scanner, or generic business-automation product.
- Non-Web3 TradePI modules must stay in isolated route/package namespaces and must not alter Koschei Web3 evidence, security, entitlement, or customer-decision contracts.
- Chain-specific evidence collection belongs behind adapters. Core intelligence, evidence policy, attack-path reasoning, and customer decision contracts must remain chain-independent.
- ARVIS must preserve provenance and explainability. Missing evidence stays UNKNOWN and must never be converted into SAFE.
- Do not request, store, log, or commit private keys, seed phrases, passphrases, provider secrets, bot tokens, or webhook secrets.
- Do not fabricate on-chain data, risk evidence, production capability, enterprise features, inventory, price, appointment availability, or revenue attribution. Test fixtures and mocks must be explicit.

## Koschei Fabric boundary

Koschei Fabric joins Koschei Web3, Koschei Lang, and Koschei Sentinel as a **federated workspace**, not by collapsing their codebases or ownership boundaries.

- Cross-project communication uses versioned contracts, adapters, events, or HTTP interfaces.
- New integrations default to `observe` or `shadow` mode and must not change production decisions until explicitly promoted.
- Existing APIs and evidence semantics remain available.
- Shared domains may include identity, trust, policy, agents, events, evidence, storage, compute, crypto, telemetry, and compatibility, but each project's internals remain owned by that project.
- Web4/Web5/Web6 work is additive. Web3 remains the production foundation rather than a deprecated layer.

## Single-service product boundary

TradePI product modules may share the same Railway/HTTP service for cost and operational simplicity, but they must remain logically separated:

- Koschei Web3 routes and packages remain the security product.
- TradePI AI Agents use `/agents`, `/api/agents/*`, `/webhooks/telegram`, and future `/webhooks/whatsapp` namespaces.
- Shared infrastructure is limited to generic transport, configuration, database connectivity, observability, and deployment plumbing.
- Cross-product calls must use explicit interfaces/contracts rather than direct access to security internals.

## Chain architecture

Chain adapters are evidence transports, not product identities. The existing working Solana collector remains isolated behind its adapter boundary. Future chain support must not couple core intelligence to one chain's account, token, program, or transaction model.
