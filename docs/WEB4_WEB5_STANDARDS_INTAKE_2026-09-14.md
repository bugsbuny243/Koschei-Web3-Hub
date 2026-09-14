# Web4 / Web5 Standards Intake — 2026-09-14

This document translates current agentic/decentralized-web work into concrete Koschei Web3 architecture decisions. `Web4` and `Web5` are treated as umbrella labels, not as standards with authority over the product.

## Executive decision

Koschei should adopt durable primitives behind adapters rather than introduce a monolithic `web4` or `web5` subsystem.

Priority order:

1. **MCP 2026-07-28** for agent-to-tool/data interoperability.
2. **A2A 1.0** for cross-agent discovery, delegation, and task collaboration.
3. **W3C Verifiable Credentials 2.x** for portable, cryptographically verifiable claims where a real customer/evidence flow needs them.
4. **HTTP-native payment/authorization primitives such as x402** only behind entitlement/effect gates and only for concrete commercial/API flows.
5. Historical TBD/Web5 material is research input only; archived branding/repos are not a production dependency.

## MCP

Koschei should prefer a stateless MCP edge adapter rather than session state inside ARVIS. MCP tools expose bounded capabilities; they do not receive authority over ARVIS verdicts, entitlements, evidence stores, or signing/effect execution. Protocol-version pinning is mandatory, and external tool output enters as untrusted/provenanced observation until mapped by a native adapter.

Source: https://blog.modelcontextprotocol.io/posts/2026-07-28/

## A2A

A2A belongs at the Fabric/agent interoperability edge, not inside the deterministic Web3 verdict engine. Agent Cards may advertise only capabilities that are actually enabled and operator-visible. Delegated tasks must carry explicit tenant/case identity, authorization context, correlation IDs, deadlines, and provenance. Remote-agent results are evidence/interpretation input, never final authority.

Sources:
- https://a2a-protocol.org/dev/specification/
- https://a2a-protocol.org/latest/blog/2026/08/27/a-new-chapter-for-a2a-joining-the-agentic-ai-foundation/

## W3C Verifiable Credentials

Use VCs only for claims that benefit from issuer/holder/verifier portability. Credential verification must include issuer trust policy, proof verification, status/revocation where relevant, freshness, subject binding, and schema/version handling. A valid credential proves a signed claim under a trust policy; it does not automatically prove the underlying real-world fact.

Sources:
- https://www.w3.org/TR/vc-data-model-2.0/
- https://www.w3.org/TR/vc-data-model-2.1/

## x402 and internet-native payments

Do not replace the current entitlement ledger with x402. A future x402 adapter may become a payment transport that settles a bounded purchase/credit operation, but commercial authorization still resolves through the Koschei entitlement plane before paid security output is released.

Source: https://www.linuxfoundation.org/press/linux-foundation-announces-operational-launch-of-x402-foundation-to-standardize-internet-native-payments-for-ai-agents-and-applications

## Historical TBD / Web5 material

Archived TBD/Web5 repositories remain research input only. Koschei should keep useful concepts at the standards level and avoid a `web5/` production namespace that implies dependence on an archived platform generation.

## Adapter boundary

```text
external protocol
      |
      v
versioned edge adapter
      |
      v
Koschei Fabric contract
      |
      v
Web3 evidence/provenance mapping
      |
      v
ARVIS deterministic policy
      |
      v
native customer decision contract
```

The external layer cannot directly mint a final Web3 decision, entitlement, Lang capability, or Sentinel model authority.

## Implementation order

### P0 — harden the contract

- Finish current entitlement/readiness/Fabric/signing blockers before effectful protocol integration.
- Define adapter requirements for version pinning, authn/authz, tenant binding, provenance, replay/idempotency, timeouts, size limits, rate limits, observability, and fail-closed behavior.
- Add acceptance cases for stale/replayed messages and untrusted remote-agent/tool output.

### P1 — observe-only interoperability

- MCP read-only tool/data gateway exposing explicitly allowlisted Web3 capabilities.
- A2A read-only Agent Card and observe-only task receiver.
- VC verifier behind a narrow interface; no customer-decision authority.

### P2 — gated effects

- Bind any approved effectful MCP/A2A action to native one-use intent/effect receipts.
- Consider x402 only after entitlement lifecycle acceptance is real.

## Non-goals

- No Web4/Web5 rewrite of the Web3 runtime.
- No protocol-driven bypass of ARVIS, entitlement, Fabric, or signing gates.
- No claim that draft specifications are final standards.
- No production-readiness claim until exact-head CI and relevant acceptance gates execute.
