# Koschei ARVIS API Reference

This document lists the current production routes separately from planned developer expansion. It must not present roadmap endpoints as already live.

## Authentication

### Customer session routes

Use:

```http
Authorization: Bearer CUSTOMER_SESSION_TOKEN
```

These routes may also require an active entitlement.

### Partner API routes

Use either:

```http
X-API-Key: ARVIS_API_KEY
```

or:

```http
Authorization: Bearer ARVIS_API_KEY
```

API keys have per-minute and monthly quota controls.

---

## Live: POST /api/v1/radar/check

Runs an evidence-backed radar check for a Solana target.

Authentication: customer session + active entitlement.

```json
{
  "target": "SOLANA_TARGET",
  "network": "solana-mainnet",
  "mode": "developer_test"
}
```

The response may include the final verdict, evidence arms, signature metadata and charge status.

---

## Live: GET /api/v1/radar/feed

Returns the customer radar feed and production health information.

Authentication: customer session + active entitlement.

Response may include:

- verified risk cards
- verified monitor cards
- source freshness
- completed and active jobs
- verdict throughput
- backlog and error counters

---

## Live: POST /api/v1/scan/token

Queues an API-key-protected Solana token scan.

Authentication: partner API key.

```json
{
  "mint": "TOKEN_MINT",
  "network": "solana-mainnet",
  "include_ai": false
}
```

Accepted response:

```json
{
  "request_id": "REQUEST_ID",
  "status": "queued",
  "cost_credits": 1
}
```

The request is charged only after the job is successfully reserved. Failed RPC processing is refunded by the backend flow.

---

## Live: POST /api/v1/shield/preflight

Runs a security preflight check for a target, token mint, address or transaction.

Authentication: partner API key.

```json
{
  "target": "SOLANA_TARGET",
  "wallet": "OPTIONAL_WALLET",
  "network": "solana-mainnet",
  "context": {
    "surface": "wallet_warning"
  }
}
```

Response fields may include:

- `action`
- `grade`
- `risk_index`
- `risk_level`
- `verdict`
- `recommendation`
- `signed`
- `signature`
- module evidence quality

---

## Live: GET /api/v1/usage

Returns recent API-key usage events.

Authentication: partner API key.

Usage records may include endpoint, status, reserved credits, charged credits, error code and completion timestamps.

---

## Signed verdict trust rule

Consumers should treat a verdict as final only when:

```json
{
  "signed": true
}
```

They should also display or inspect evidence and rule metadata rather than relying only on the numeric score.

When verified evidence is unavailable, ARVIS should withhold the final customer verdict instead of inventing a grade.

---

## Planned expansion — not production routes yet

The following developer surfaces remain roadmap items until implemented and tested:

- wallet-specific API-key analysis route
- dedicated sybil-cluster batch route
- webhook subscription and delivery routes
- bulk token screening route

These planned routes must remain labeled as roadmap work in public materials.
