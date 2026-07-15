# Signed Verdict Schema

Koschei ARVIS uses a stable signed verdict object so Solana builders can consume the same evidence-backed decision from the radar UI, reports, API endpoints and SDKs.

## Goal

A verdict is not a score. It carries the deterministic letter grade, evidence, triggered rules, ordered decision path and ruleset metadata that produced the result.

## Core fields

| Field | Purpose |
| --- | --- |
| `target` | Token, wallet, pool, program, transaction or claim target. |
| `network` | Expected value: `solana-mainnet`. |
| `grade` | A-F grade, or `-` when no grade-changing rule was triggered. `-` is not an A grade. |
| `evidence` | Evidence statements used by the engine. |
| `rule_version` | Deterministic ruleset version that produced or withheld the grade. |
| `triggered_rules` | Rule IDs and evidence statuses that contributed to the decision. |
| `decision_path` | Ordered explanation of how the ruleset reached or withheld the grade. |
| `signed` | Boolean flag showing the verdict was produced by the final verdict engine. |
| `created_at` | Verdict creation timestamp. |

Numeric `risk_index` and categorical `risk_level` are not part of the final verdict contract. Module-level diagnostics may exist internally, but clients must not use them as the final decision.

## Example

```json
{
  "target": "SOLANA_TARGET",
  "network": "solana-mainnet",
  "grade": "C",
  "evidence": [
    "creator reuse was verified across two token creation transactions",
    "the same dominant holder was observed across two token dossiers"
  ],
  "rule_version": "koschei-actor-defense-rules-v1.0.0",
  "triggered_rules": [
    {
      "rule_id": "ARD-C001",
      "title": "Creator reuse",
      "evidence_status": "verified",
      "summary": "The same creator was transaction-backed across multiple tokens."
    }
  ],
  "decision_path": [
    "No hard trigger was present.",
    "The explicit compounding rule set determined the letter grade."
  ],
  "signed": true,
  "created_at": "2026-06-24T00:00:00Z"
}
```

## No-grade rule

A valid contract may contain `"grade": "-"` when no grade-changing rule was triggered. That state must be explained in `decision_path`; it does not imply safety and must never be converted into A.

## Withheld verdict rule

When the required evidence is not available, clients withhold the final customer action instead of inventing a grade. This preserves evidence-first behavior across developer integrations.
