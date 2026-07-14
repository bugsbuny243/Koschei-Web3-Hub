# Repository agent contract

All Codex and automated implementation work in this repository must begin by reading and referencing [`ACTOR_INVESTIGATION_ENGINE.md`](./ACTOR_INVESTIGATION_ENGINE.md).

## Non-negotiable product rules

1. Koschei is an evidence-first actor investigation engine, not a risk-card generator.
2. New actor/token investigation work must answer at least one question from the canonical document's ten-question filter.
3. Actor verdicts use versioned deterministic rules. Weighted formulas, probabilities and `0–100` scores are prohibited.
4. `INFERRED` evidence is watch-only. `UNVERIFIED` evidence cannot affect a grade or appear as a verified claim.
5. AI may explain triggered rules but may not generate, raise, lower or override a grade.
6. Serious claims require evidence rows with signature, slot, timestamp, source, destination, amount, program and verification status.
7. Initial-distribution and holder follow-up must be mint-specific/ATA-based; recipient-wide full wallet history scans are prohibited in broad pipelines.
8. Actor index history is persistent. Raw-event retention must not delete durable actor memory.
9. Do not introduce demo, beta, placeholder, synthetic or fabricated production outputs.
10. Do not modify auth, Neon Auth, sessions, owner cookies, KOSCH entitlement or verified-wallet implementation unless the user explicitly requests that exact work.
11. Do not delete a legacy production path until its replacement has proven behavioral parity and rollback safety.

Every task description and pull request touching investigation behavior must name the applicable section(s) and ruleset version from the canonical document.
