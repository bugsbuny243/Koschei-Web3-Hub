# Web4-Web6 Sentinel research delta — 2026-10-05

Status discipline: the items below are active community or Internet-Draft work, not final Web4/Web5/Web6 standards.

## Fresh signals

- W3C Agent Conformance and Benchmarking Community Group continues to center observable evidence, adversarial reference fixtures, versioned corpora, engine/corpus/date metadata, and independent reproducibility.
- IETF WIMSE lists verifier-side delegated-authority evaluation (`draft-jackson-wimse-evaluation-03`), authorization audit records (`draft-gilda-wimse-agent-audit-record-01`), attenuated delegation chains (`draft-asor-wimse-agent-delegation-chain-01`), AI-agent identity applicability (`draft-ni-wimse-ai-agent-identity-03`), and connected-flight chained trust (`draft-seymour-wimse-connected-flight-06`) as active Internet-Drafts.
- The Oct 1 connected-flight revision reinforces a broader architectural signal: agent-to-agent trust should remain bounded and verifiable across delegation hops rather than inheriting unconstrained authority.

## Sentinel implications

1. A model proposal is never authority by itself.
2. Every defensive action must bind to evidence, tenant, target, capability and delegated scope.
3. Delegation must attenuate; a downstream actor cannot gain authority absent from its upstream chain.
4. Authorization decision evidence and independently observed action effect remain separate records.
5. `unknown` observation is not success; verification fails closed when effect cannot be established.
6. Benchmark evidence must bind corpus version, engine version, date and adversarial fixtures before production-readiness claims.

## Integration target

Keep Sentinel inside the existing Web3 runtime and preserve the deterministic ARVIS verdict as an independent evidence authority. The convergence path remains:

`ARVIS evidence -> Sentinel reasoning proposal -> authority/delegation verification -> defensive policy gate -> bounded defensive execution -> independent effect observation -> recovery/audit evidence`

No external offensive action is introduced by this architecture.
