# Signed Verdict Schema

Koschei ARVIS uses a stable signed verdict object so Solana builders can consume the same risk output from the radar UI, reports, API endpoints and future SDKs.

## Goal

A verdict should never be a vague score. It should carry evidence, rule metadata and a clear final status.

## Core fields

| Field | Purpose |
| --- | --- |
| `target` | Token, wallet, pool, program, transaction or claim target. |
| `network` | Expected value: `solana-mainnet`. |
| `grade` | Human readable A-F risk grade. |
| `risk_index` | Numeric risk score from 0 to 100. |
| `risk_level` | `low`, `medium`, `high`, or `critical`. |
| `evidence` | Evidence statements used by the engine. |
| `rule_version` | Rule version that produced the verdict. |
| `signed` | Boolean flag showing the verdict was produced by the final verdict engine. |
| `created_at` | Verdict creation timestamp. |

## Example

```json
{
  "target": "SOLANA_TARGET",
  "network": "solana-mainnet",
  "grade": "C",
  "risk_index": 45,
  "risk_level": "medium",
  "evidence": [
    "launch surface observed",
    "liquidity context available",
    "holder concentration requires review"
  ],
  "rule_version": "arvis-live",
  "signed": true,
  "created_at": "2026-06-24T00:00:00Z"
}
```

## Withheld verdict rule

If verified evidence is not available, ARVIS should withhold the final customer verdict instead of inventing a grade.

This rule is important for trust, developer integrations and grant review because it keeps the infrastructure evidence-first.
