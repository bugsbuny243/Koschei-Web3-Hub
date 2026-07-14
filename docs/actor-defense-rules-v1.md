# Koschei Actor Defense Ruleset v1.0.0

Ruleset identifier: `koschei-actor-defense-rules-v1.0.0`

## Decision contract

Koschei does not calculate a weighted risk score for an actor dossier. There is no probability, confidence average, weighted sum or `0–100` output.

A letter verdict is produced only by a named, versioned rule. Every verdict contains:

- the ruleset version;
- the exact triggered rule IDs;
- each rule's evidence class;
- evidence keys and transaction signatures where available;
- the deterministic decision path;
- a signature over the canonical rule result.

No triggered rule means no letter grade. Absence of evidence is not an `A` verdict.

## Evidence classes

| Evidence class | Grade effect |
| --- | --- |
| `VERIFIED` | May activate a hard trigger or participate in a compounding rule. |
| `OBSERVED` | May participate only in a named compounding rule. |
| `INFERRED` | Watch flag only. It cannot change a grade. |
| `UNVERIFIED` | Excluded from the verdict. |

## Tier 1 — hard triggers

A hard trigger sets the best possible grade. Supporting observations remain visible but do not move a hard-trigger grade in ruleset v1.0.0.

| Rule | Required evidence | Grade ceiling |
| --- | --- | --- |
| `ARD-H001` — Creator liquidity removal | Creator/deployer role observed; actor is an exact signer; parsed liquidity-removal instruction; the same transaction touches a creator-linked mint; evidence status `VERIFIED`. | `D` |
| `ARD-H002` — Direct creator-to-dominant-holder funding | Creator/deployer role observed; exact parsed outgoing SOL or SPL transfer; counterpart is an owner-resolved holder with at least 20% observed share; evidence status `VERIFIED`. | `D` |
| `ARD-H003` — Previous-token removal/rug history | Transaction-backed previous-token incident attached to the creator/deployer wallet; evidence status `VERIFIED`. | `C` |

A log message alone cannot activate `ARD-H001`. A wallet relation alone cannot activate `ARD-H002`.

## Tier 2 — compounding rules

A single compounding observation does not issue a letter grade. When no hard trigger exists, two or more distinct `VERIFIED` or `OBSERVED` compounding rules lower the baseline by one grade to `B`.

| Rule | Match condition |
| --- | --- |
| `ARD-C001` — Creator/deployer reuse | Same creator/deployer wallet connected to at least two observed tokens. |
| `ARD-C002` — Dominant-holder reuse | Same owner-resolved wallet is dominant across at least two observed tokens. |
| `ARD-C003` — Related-actor recurrence | Owner-resolved related actor recurs across the actor's token surface. |
| `ARD-C004` — Repeated direct transfer | Same direct SOL/SPL transfer relation occurs at least twice. |
| `ARD-C005` — Observed liquidity removal | Parsed instruction or log indicates removal activity but the hard-trigger verification boundary is not met. |

Compounding rules are based on distinct rule IDs, not an arithmetic sum of evidence weights.

## Tier 3 — watch flags

`INFERRED` relations are rendered under `watch_flags`. They are investigation leads and never alter the grade. `UNVERIFIED` records are counted as excluded and do not appear as grade inputs.

## Queue ordering

The owner investigation queue is categorical:

1. `HARD TRIGGER`
2. `COMPOUNDING`
3. `EVIDENCE PENDING`
4. `WATCH`
5. `VERIFIED REVIEW`
6. `MONITOR`

There is no numerical queue priority.

## AI boundary

Rules produce the verdict. An AI model may translate the triggered rules and evidence into human-readable narrative, but it may not select, raise, lower or override the grade.

The correct attribution is: **the rules issued the grade; the AI explained it.**

## Versioning

Any change to rule conditions, evidence boundaries, grade ceilings, compounding behavior or signature payload requires a new ruleset version. Historical signed verdicts retain the version that produced them and are never silently reinterpreted under a newer ruleset.
