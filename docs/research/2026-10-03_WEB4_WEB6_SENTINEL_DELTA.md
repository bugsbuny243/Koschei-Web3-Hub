# Web4-Web6 Sentinel research delta — 2026-10-03

Status discipline: the W3C items below are Community Group work, and the IETF items below are Internet-Drafts/work in progress unless otherwise stated. They are not treated as final standards.

## Fresh delta

- W3C Agent Conformance and Benchmarking continues to emphasize independently reproducible results, observable evidence per check, adversarial fixtures, and versioned engine/corpus/date metadata.
- IETF WIMSE now lists verifier-side delegated-authority evaluation semantics revision `draft-jackson-wimse-evaluation-03` dated 2026-09-30.
- The active agent authorization audit-record draft remains `draft-gilda-wimse-agent-audit-record-01`; it separates authorization decision from independently observed effect and derives agreement rather than trusting an executor acknowledgement alone.
- WIMSE also lists authenticated provenance work for delegation chains (`draft-reddy-wimse-aggregate-signatures-01`, 2026-09-28).

## Sentinel engineering consequence

The Web3-integrated Sentinel path should preserve four separate facts:

1. what evidence was observed,
2. what authority/policy allowed,
3. what defensive action was requested/executed,
4. what an independent observer actually saw afterward.

A model response or executor success flag is not containment proof. Production evidence should bind engine/corpus/version/date and the observed-effect evidence digest. Unknown/unobservable effect must fail closed rather than being treated as success.

## Sources

- W3C Agent Conformance and Benchmarking Community Group, accessed 2026-10-03.
- IETF WIMSE documents index, accessed 2026-10-03.
- `draft-gilda-wimse-agent-audit-record-01`, active individual Internet-Draft, September 2026.
