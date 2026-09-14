# Koschei Web3 Product Direction — 2026-09-10

This document restores the product-direction target referenced by `PROJECT_STATE.md`. It records scope; it does **not** claim production completion, live-chain coverage, paid-customer acceptance, or model/runtime readiness.

## Product identity

Koschei Web3 is a **Web3 Security Validation & Risk Intelligence Platform**. ARVIS remains the evidence/intelligence core for Web3 customer decisions.

The product must not collapse into a generic wallet, token scanner, contract scanner, business-automation suite, or AI-agent shell.

## Network scope

The product direction is **all-network / all-address**, while implementation remains evidence-driven and adapter-based.

- Existing Solana/Pump behavior is preserved.
- A network/address appearing in the UI or registry is not proof that a live collector exists.
- Coverage must distinguish `supported`, `partial`, `offline-resolution-only`, `unavailable`, and `unknown` states where applicable.
- Core evidence, provenance, attack-path reasoning, and decision contracts remain chain-independent.
- New live collectors are admitted behind explicit adapters and acceptance gates.

## Address resolution

Network-scoped address resolution may identify the requested network/address type and route work to the appropriate adapter without pretending that evidence exists.

Resolution must not:

- fabricate transactions, balances, labels, ownership, relationships, or risk signals;
- map unsupported networks to Solana semantics;
- convert missing collector coverage into a safe verdict;
- bypass evidence provenance or signed customer-decision contracts.

## Address-free telemetry

Process and platform telemetry that does not require a wallet/address may be surfaced independently when it has a truthful source and status. Such telemetry is operational context, not chain evidence, unless a native evidence contract explicitly binds it into a case.

## Lang and Sentinel commercial/project boundaries

- Koschei Lang remains an independent repository/product and owns language/runtime/authority internals.
- Koschei Sentinel remains an independent repository/product and owns threat/model/evaluation/research interpretation.
- Lang + Sentinel may have joint commercial packaging without merging code ownership or authority.
- Koschei Web3 remains independently deployable and remains the owner of Web3 customer decision authority.
- Cross-project integration uses Koschei Fabric versioned contracts/adapters.

## Admission rule for new protocols

New Web4/Web5/agentic-web protocols are additive adapters, not new sources of decision authority. Before promotion they must define:

1. protocol/version pinning;
2. authentication and authorization boundary;
3. replay/nonce/idempotency behavior where effects exist;
4. provenance and evidence mapping;
5. failure semantics (`UNKNOWN`/withheld rather than invented success);
6. operator-visible telemetry;
7. compatibility and rollback strategy;
8. exact-head tests/acceptance evidence.

See `docs/WEB4_WEB5_STANDARDS_INTAKE_2026-09-14.md` for the current external-standards intake.

## Completion language

This document is a direction contract. Production completion still requires the blockers and acceptance work recorded in `PROJECT_STATE.md`, including entitlement-ledger validation, exact-head readiness truthfulness, common Fabric acceptance, one-use signed intent/effect acceptance, and the relevant Lang/Sentinel independent gates.