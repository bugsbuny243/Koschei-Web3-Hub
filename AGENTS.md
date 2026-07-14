# Repository agent contract

All Codex and automated implementation work in this repository must begin by reading and referencing [`ACTOR_INVESTIGATION_ENGINE.md`](./ACTOR_INVESTIGATION_ENGINE.md).

## Non-negotiable product rules

1. Koschei is an evidence-first actor investigation engine, not a risk-card generator.
2. New actor/token investigation work must answer at least one question from the canonical document's ten-question filter.
3. Actor and unified Radar verdicts use versioned deterministic rules. Weighted formulas, probabilities and `0–100` final scores are prohibited.
4. The owner-facing primary Radar is one manual pipeline: 14 legacy ARVIS arms + actor investigation + market/holder behavior rules + one letter-only final verdict.
5. The unified behavior ruleset includes explicit, versioned rules for volume/liquidity gap, dominant-holder position/liquidity pressure, creator sell acceleration and dominant-holder first observed exit.
6. `INFERRED` evidence is watch-only. `UNVERIFIED` evidence cannot affect a grade or appear as a verified claim.
7. AI may explain triggered rules but may not generate, raise, lower or override a grade.
8. Serious claims require evidence rows with signature, slot, timestamp, source, destination, amount, program and verification status.
9. Initial-distribution and holder follow-up must be mint-specific/ATA-based; recipient-wide full wallet history scans are prohibited in broad pipelines.
10. Actor index history is persistent. Raw-event retention must not delete durable actor memory.
11. Quota-consuming automatic scanning is opt-in and disabled by default. Manual owner scans must never silently enable background workers.
12. Do not introduce demo, beta, placeholder, synthetic or fabricated production outputs.
13. Do not modify auth, Neon Auth, sessions, owner cookies, KOSCH entitlement or verified-wallet implementation unless the user explicitly requests that exact work.
14. Do not delete a legacy production path until its replacement has proven behavioral parity and rollback safety.

Every task description and pull request touching investigation behavior must name the applicable section(s), the actor ruleset version and the unified Radar ruleset version.
