# Web4–Web8 Intelligence Update — 2026-09-28

Window: new material observed after the 2026-09-25 research baseline.

## Standards reality

Web4, Web5, Web6, Web7 and Web8 remain working labels inside Koschei, not accepted generation names at IETF or W3C.

## Web4 — agentic / semantic / federated security

### Agent Action Capsules (AAC) — 2026-09-26
SCITT Internet-Draft draft-mih-scitt-agent-action-capsule-05 defines digest-committed AI-agent action records with executed/blocked/denied/errored/timed-out disposition, deterministic constraints, committed effects, confirmed-effect binding and human-in-the-loop state.
Koschei relevance: strong match for ARVIS evidence envelopes, deterministic decision provenance and portable incident evidence. Research/observe only until the draft stabilizes.
Source: https://datatracker.ietf.org/doc/html/draft-mih-scitt-agent-action-capsule-05

### AAC Disclosure Envelope + Evidence Bundle — 2026-09-26
Companion drafts define selective disclosure of digest-committed agent inputs/outputs and portable evidence bundles with citation closure and explicit missing-record state.
Koschei relevance: useful for independently verifiable ARVIS dossiers without copying secret/raw data into the signed core record.
Sources:
https://datatracker.ietf.org/doc/html/draft-mih-agent-disclosure-envelope-00
https://datatracker.ietf.org/doc/html/draft-mih-zhang-agent-disclosure-bundle-00

### Simple Agent Management Protocol (SAMP) — 2026-09-26
Experimental Internet-Draft draft-efstathiou-samp-agent-management-03 proposes agent discovery, state query, events, subscriptions, autonomy classes, enrollment, trust states and policy-gated operations.
Koschei relevance: maps to Security Center runtime-health plus Sentinel observe-mode inventory; management telemetry must never become verdict authority.
Source: https://datatracker.ietf.org/doc/html/draft-efstathiou-samp-agent-management-03

### Capability Language Core (CLC) — 2026-09-27
Experimental Internet-Draft draft-wei-capability-language-core-00 defines capability identifiers, grant entailment/intersection, constraints, stable reason codes and allow / deny / allow_unresolved.
Koschei relevance: important research input for Koschei Lang policy/proof adapters; allow_unresolved aligns with missing-evidence-is-not-safe semantics.
Source: https://datatracker.ietf.org/doc/draft-wei-capability-language-core/00/

### Judgment Event Protocol (JEP) — 2026-09-26
JEP defines signed canonicalized Judgment, Delegation, Termination and Verification events with stable event identity, detached JWS, artifact hashes and idempotent acceptance.
Koschei relevance: candidate semantics for cross-agent decisions and delegation provenance through future Fabric adapters.
Source: https://datatracker.ietf.org/doc/html/draft-wang-jep-judgment-event-protocol-07

## Web5 — user-controlled identity and verifiable data

### W3C Verifiable Credentials Data Model v2.1 — Working Draft 2026-09-27
Refresh the Web5 identity-evidence research profile against v2.1. Verified identity proves only the credential/controller statement it covers; it does not imply unlimited authority or safety.
Source: https://www.w3.org/TR/2026/WD-vc-data-model-2.1-20260927/

### Wallet State Attestation — 2026-09-27
Informational Internet-Draft draft-borthwick-wallet-state-attestation-01 proposes signed booleans/facts derived from on-chain wallet state for offline verification.
Koschei relevance: bridge Web3 state evidence into Web5-style attestations as issuer-scoped evidence only.
Source: https://datatracker.ietf.org/doc/draft-borthwick-wallet-state-attestation/01/

## Web6 — verifiable autonomous compute / crypto agility

### HPKE revision — 2026-09-26
IETF WG draft draft-ietf-hpke-hpke-05 is Standards Track and would obsolete RFC 9180 if approved.
Koschei relevance: crypto-agility input for encrypted evidence transport; HPKE itself is not a blanket post-quantum guarantee.
Source: https://datatracker.ietf.org/doc/draft-ietf-hpke-hpke/05/

### Conformance Continuity — 2026-09-27
Informational Internet-Draft draft-hillier-conformance-continuity-00 addresses verifiable claims across changing baselines, jurisdictions and mutable subjects while stating what failed to carry.
Koschei relevance: useful for evidence freshness, policy-version provenance and long-lived dossier validity.
Source: https://datatracker.ietf.org/doc/draft-hillier-conformance-continuity/

## Web7 / Web8

No accepted IETF/W3C standards named Web7 or Web8 were identified in this update window. Keep them as internal research labels only.

## Additional agent-protocol watch — surfaced 2026-09-28

### AAuth Protocol v11 — published 2026-09-25

The individual Internet-Draft `draft-hardt-oauth-aauth-protocol-11` defines agent-to-resource authorization with five access modes, key-bound agent identity, signed requests, mission-scoped governance and resource-managed session tokens.

Koschei relevance:
- useful research input for separating agent identity, person authority, mission scope and resource access;
- aligns with the rule that authentication never implies authorization;
- should remain an adapter/research input until the protocol has formal standards standing.

Source:
https://datatracker.ietf.org/doc/html/draft-hardt-oauth-aauth-protocol-11

### Agent Execution Protocol (AEP) — current draft 04

`draft-sato-soos-aep-04` describes a normative boundary between an agent reasoning loop and a governing enforcement component that authorizes actions and records tamper-evident state transitions.

Koschei relevance:
- strong architectural comparison point for ARVIS pre-sign authority, Lang policy enforcement and Sentinel observe-mode evaluation;
- reinforces a hard boundary between model reasoning and deterministic enforcement.

Source:
https://datatracker.ietf.org/doc/draft-sato-soos-aep/

### A2A protocol roadmap and current security model

The A2A roadmap (updated 2026-09-15) prioritizes v1.1 robustness, bidirectional streaming, elicitation/human-in-the-loop flows and validation tooling. The current specification keeps authentication at the protocol/transport boundary and authorization server-side, scoped by identity, skill, action, data policy or OAuth scope.

Koschei relevance:
- A2A is a useful horizontal agent-to-agent transport candidate while MCP remains a tool/data integration surface;
- every A2A task and streaming event must remain subject to explicit authorization and evidence boundaries before entering Koschei's trusted graph.

Sources:
https://a2a-protocol.org/latest/roadmap/
https://a2a-protocol.org/dev/specification/

### MCP security proposals to track

Active MCP proposal work includes:
- SEP-3140: Signed Capability Declarations & Trustworthy Trust Labels;
- SEP-3004: Tamper-Evident Audit Record Contract;
- SEP-2848: Asynchronous Approval for Tool Calls;
- SEP-2643: Structured Authorization Denials.

Koschei relevance:
- capability declarations belong in discovery/provenance, not verdict authority;
- tamper-evident tool-call records could map into the evidence plane;
- asynchronous approval and structured denials match human-in-the-loop and explicit refusal semantics;
- these are proposal-stage items unless separately promoted by MCP governance.

Sources:
https://github.com/modelcontextprotocol/modelcontextprotocol/pulls
https://plan.modelcontextprotocol.io/seps
