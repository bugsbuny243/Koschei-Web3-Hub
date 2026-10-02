# Web4-Web6 Sentinel convergence research — 2026-10-02

Status discipline: the W3C items below are Community Group work, and the IETF WIMSE items are Internet-Drafts unless explicitly stated otherwise. They are not treated as final standards.

## Fresh signals

- W3C Agent Conformance and Benchmarking continues to emphasize independently reproducible measurements, observable evidence per check, adversarial fixtures, and versioned engine/corpus/date metadata.
- W3C Agent Identity Registry Protocol continues to focus on cryptographically verifiable agent identity, controlling-organization binding, authorization scope, credential lifecycle/revocation, and protocol interoperability.
- W3C AI Agent Protocol continues to focus on discovery, identity, capability/intent exchange, authorization, privacy, and interoperable agent collaboration.
- IETF WIMSE now lists verifier-side delegated-authority evaluation semantics revision -03 dated 2026-09-30.
- IETF draft-gilda-wimse-agent-audit-record-01 keeps authorization decision and observed effect distinct and derives agreement from them. Sentinel should preserve that separation rather than treating executor acknowledgement as proof of containment.

## Product consequence

Koschei Web3 and Sentinel should converge at the evidence/control boundary without weakening ARVIS deterministic verdict authority:

1. ARVIS remains the deterministic signed evidence/verdict plane.
2. Sentinel consumes bounded evidence projections and produces defensive analysis/plans.
3. Model output never becomes authority by itself.
4. Identity/delegation/policy checks gate any authorized defensive action.
5. Intended action and independently observed effect remain separate evidence objects.
6. Production claims require versioned corpus, engine/version/date metadata, adversarial fixtures and reproducible evidence.

## Deployment consequence

The production target is one Koschei Web3 service surface. Sentinel is integrated as an internal defensive subsystem of that service/repository rather than requiring a separate production service. Integration must remain fail-closed and must not change the signed ARVIS verdict merely because Sentinel is unavailable.
