# Public API Reference

This document covers the first externally documented Koschei ARVIS endpoints. The implementation is evidence-first: unavailable evidence must never be interpreted as low risk.

## Authentication

Authenticated endpoints require the same user session or token used by the live product. Some endpoints also require an active entitlement and an available output.

## POST /api/v1/unified/analyze

Runs unified analysis for a supported Solana target.

Request body:

```json
{
  "target": "So11111111111111111111111111111111111111112",
  "target_type": "token",
  "network": "solana-mainnet"
}
```

Accepted request fields:

- `input`
- `context`
- `target`
- `target_type`
- `target_id`
- `network`
- `notes`

Supported target type hints include `wallet`, `token`, `transaction`, `program`, and `project`.

Success responses use this envelope:

```json
{
  "success": true,
  "code": "OK",
  "message": "Analysis completed",
  "data": {
    "input_type": "token",
    "summary": "Evidence-backed summary",
    "sections": {},
    "security_radars": {},
    "sources": ["koschei_security_rules", "solana_rpc"],
    "partial_failures": []
  }
}
```

When live evidence is unavailable, the endpoint returns an explicit failure and does not charge an output:

```json
{
  "ok": false,
  "error": "real_data_unavailable",
  "charged": false,
  "sections": {}
}
```

## POST /api/v1/radar/check

Runs the ARVIS radar analysis arms and returns a signed final verdict only when the required evidence exists.

Request body:

```json
{
  "target": "So11111111111111111111111111111111111111112",
  "network": "solana-mainnet",
  "mode": "manual_dashboard_check"
}
```

`address` is accepted as an alias for `target`.

Success response shape:

```json
{
  "ok": true,
  "bundle": {},
  "arms": [],
  "final_verdict": {
    "signed": true
  }
}
```

If the evidence boundary is not satisfied, the response is unsigned and `charged` is `false`.

## GET /api/v1/risk/badge

Returns a public, rate-limited risk badge.

Query parameters:

- `address` — required target address
- `token` — accepted alias for `address`
- `network` — defaults to `solana-mainnet`

Example:

```bash
curl "https://tradepigloball.co/api/v1/risk/badge?address=So11111111111111111111111111111111111111112&network=solana-mainnet"
```

Success response shape:

```json
{
  "ok": true,
  "address": "So11111111111111111111111111111111111111112",
  "grade": "B",
  "risk_index": 35,
  "risk_level": "medium",
  "verdict": "monitor",
  "recommendation": "Review the verified findings before interacting.",
  "rule_version": "current",
  "signed": true,
  "signature": "...",
  "verified_arm_count": 8
}
```

The sample values above describe the response structure only. They are not a live verdict for the example address.

When evidence is unavailable, the endpoint returns `503` with `signed: false`.

## Status behavior

- `200` — evidence-backed success
- `400` — invalid input
- `401` — authentication required
- `402` — active entitlement or output required
- `502` — authenticated analysis could not obtain sufficient real evidence
- `503` — public badge could not obtain sufficient real evidence

## Integration rule

Always check the evidence state and `signed` value before displaying a verdict. Missing evidence is not a safety signal.
