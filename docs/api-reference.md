# Koschei ARVIS API Reference

Koschei ARVIS is being structured as a developer-facing Solana risk intelligence API. These endpoint contracts describe the public surfaces planned for builders and partner integrations.

## Authentication

Production endpoints should use user or partner API credentials. Dashboard routes may continue to use the existing authenticated session flow.

## Common response shape

```json
{
  "ok": true,
  "target": "SOLANA_TARGET",
  "network": "solana-mainnet",
  "final_verdict": {
    "grade": "C",
    "risk_index": 45,
    "risk_level": "medium",
    "evidence": [],
    "rule_version": "arvis-live",
    "signed": true
  }
}
```

## POST /api/v1/risk/check

General target check for tokens, pools, wallets, programs, transactions or claim URLs.

Request:

```json
{
  "target": "SOLANA_TARGET",
  "network": "solana-mainnet"
}
```

Use cases:

- wallet warning screen
- token discovery risk check
- claim URL pre-check
- partner dashboard enrichment

## POST /api/v1/token/scan

Token-specific scan for launch, liquidity and authority evidence.

Request:

```json
{
  "mint": "TOKEN_MINT",
  "network": "solana-mainnet",
  "include_evidence": true
}
```

## POST /api/v1/wallet/analyze

Wallet intelligence endpoint for age, activity, counterparty and sybil hints.

Request:

```json
{
  "wallet": "SOLANA_WALLET",
  "network": "solana-mainnet"
}
```

## POST /api/v1/sybil/detect

Cluster and relation checks for suspicious wallet groups.

Request:

```json
{
  "wallets": ["SOLANA_WALLET_1", "SOLANA_WALLET_2"],
  "network": "solana-mainnet"
}
```

## GET /api/v1/radar/feed

Live feed for radar cards and production proof.

Response includes:

- verified risk cards
- verified monitor cards
- stream health
- completed jobs
- final verdict throughput
- backlog and error counters

## Error behavior

If verified evidence is unavailable, ARVIS should return a withheld verdict response rather than inventing a score.

Example:

```json
{
  "ok": false,
  "error": "verified_evidence_unavailable",
  "charged": false
}
```
