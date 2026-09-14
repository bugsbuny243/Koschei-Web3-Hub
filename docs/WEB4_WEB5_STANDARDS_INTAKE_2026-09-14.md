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

## 1. Model Context Protocol (MCP)

The MCP `2026-07-28` specification moved the protocol core to stateless request/response semantics and added authorization hardening, an extensions framework, multi-round-trip requests, header-based routing, and cacheable list results. Official Tier-1 SDK support includes Go, which matches the primary Koschei Web3 runtime language.

Koschei implication:

- Prefer a **stateless MCP edge adapter** rather than session state inside ARVIS.
- MCP tools expose bounded capabilities; they do not receive direct authority over ARVIS verdicts, entitlements, evidence stores, or signing/effect execution.
- Protocol-version pinning is mandatory.
- External tool output enters as untrusted/provenanced observation until mapped by a native adapter.
- Long-running MCP Tasks may coordinate work but cannot substitute for the deliberately disabled Web3 durable-job path.

Source:
- https://blog.modelcontextprotocol.io/posts/2026-07-28/

## 2. Agent2Agent (A2A)

A2A reached released version `1.0.0` and defines a vendor-neutral model for independent agents to discover capabilities, negotiate modalities, manage collaborative tasks, and exchange information without exposing internal state/tools. In August 2026 it joined the Agentic AI Foundation as a Growth Stage project. The A2A project describes MCP as the vertical tool/data integration layer and A2A as the horizontal agent collaboration layer.

Koschei implication:

- A2A belongs at the **Fabric/agent interoperability edge**, not inside the deterministic Web3 verdict engine.
- Agent Cards may advertise only capabilities that are actually enabled and operator-visible.
- Delegated tasks must carry explicit tenant/case identity, authorization context, correlation IDs, deadlines, and provenance.
- A remote agent result is evidence/interpretation input, never a final authority grant.
- Any effectful task must pass native Koschei intent/authorization/idempotency gates.

Sources:
- https://a2a-protocol.org/dev/specification/
- https://a2a-protocol.org/latest/blog/2026/08/27/a-new-chapter-for-a2a-joining-the-agentic-ai-foundation/
- https://www.linuxfoundation.org/press/a2a-protocol-surpasses-150-organizations-lands-in-major-cloud-platforms-and-sees-enterprise-production-use-in-first-year

## 3. W3C Verifiable Credentials

Verifiable Credentials Data Model v2.0 became a W3C Recommendation on 15 May 2025. VC Data Model v2.1 is a Working Draft dated 16 August 2026, so v2.1 features must be treated as draft/experimental until their standards status advances.

Koschei implication:

- Use VCs only for claims that benefit from issuer/holder/verifier portability, such as organization/operator attestations, integration trust claims, or evidence-origin assertions.
- Do not encode an ARVIS verdict as a credential merely to make it look decentralized.
- Credential verification must include issuer trust policy, proof verification, status/revocation where relevant, freshness, subject binding, and schema/version handling.
- A valid credential proves a signed claim under a trust policy; it does not automatically prove the underlying real-world fact.

Sources:
- https://www.w3.org/TR/vc-data-model-2.0/
- https://www.w3.org/TR/vc-data-model-2.1/

## 4. x402 and internet-native agent payments

The x402 Foundation became operational under Linux Foundation governance in July 2026 to steward HTTP-native payments for agents/APIs/applications.

Koschei implication:

- Do not replace the current entitlement ledger with x402.
- A future x402 adapter may become a payment transport that settles a bounded purchase/credit operation.
- Commercial authorization still resolves through the Koschei entitlement plane before paid security output is released.
- Payment success must not become security verdict authority.

Source:
- https://www.linuxfoundation.org/press/linux-foundation-announces-operational-launch-of-x402-foundation-to-standardize-internet-native-payments-for-ai-agents-and-applications

## 5. Historical TBD / Web5 material

TBD's former Web5 repositories and developer-site material were archived in December 2024. The concepts around decentralized identifiers, credentials, and decentralized data exchange can still inform design, but Koschei must not depend on archived Web5 branding or abandoned implementation surfaces.

Koschei implication:

- Keep useful concepts at the standards level (VCs, identifiers, signed portable claims, decentralized exchange patterns).
- Avoid a `web5/` product namespace that implies a supported external platform generation.
- Admit individual protocols only through versioned adapters and acceptance evidence.

Examples of archived material:
- https://github.com/TBD54566975/developer.tbd.website
- https://github.com/TBD54566975/web5-chatgpt-plugin

## Proposed Koschei adapter boundaries

```text
External agent/tool/credential/payment protocols
                    |
                    v
        protocol/version adapters
        - MCP edge
        - A2A edge
        - VC verifier/issuer edge
        - optional payment edge
                    |
                    v
          Koschei Fabric contracts
       identity / case / provenance /
       capability / effect metadata
                    |
        +-----------+-----------+
        |                       |
        v                       v
 Web3 evidence adapters    operator telemetry
        |
        v
 ARVIS deterministic evidence policy
        |
        v
 native customer-decision contract
```

The external layer cannot directly mint a final Web3 decision, entitlement, Lang capability, or Sentinel model authority.

## Implementation backlog

### P0 — contract hardening before protocol code

- Complete current CORE-01/CORE-02/CORE-04/SIGN-01 blockers in `PROJECT_STATE.md`.
- Define a protocol-adapter security contract: version pin, authn/authz, tenant binding, provenance, replay/idempotency, timeouts, size limits, rate limits, observability, and fail-closed behavior.
- Extend acceptance cases for untrusted remote-agent/tool output and stale/replayed protocol messages.

### P1 — observe-only interoperability

- MCP read-only tool/data gateway exposing explicitly allowlisted Web3 capabilities.
- A2A read-only Agent Card plus observe-only task receiver for case/evidence interpretation handoff.
- VC verifier library behind a narrow interface; no customer-decision authority.

### P2 — gated effects

- Bind approved effectful MCP/A2A actions to SIGN-01 one-use intent/effect receipts.
- Consider x402 only as an optional settlement adapter after entitlement lifecycle acceptance is real.

## Non-goals

- No `Web4` or `Web5` rewrite of the Web3 runtime.
- No protocol-driven bypass of ARVIS, entitlement, Fabric, or signing gates.
- No claim that draft specs are final standards.
- No dependency on archived TBD Web5 code as a production foundation.
- No production-readiness claim until exact-head CI and the relevant acceptance gates execute.