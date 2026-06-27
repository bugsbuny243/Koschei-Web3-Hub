# Transaction Firewall

Koschei Transaction Firewall is implemented with the Go API and a plain HTML interface.

## Current mode

The first release runs in shadow mode:

- it never signs a transaction
- it never submits a transaction
- it never blocks a wallet automatically
- it returns a security recommendation only
- it stores only a transaction fingerprint, findings and sanitized simulation logs
- it does not store the serialized transaction

## Endpoint

```http
POST /api/v1/shield/transaction
X-API-Key: YOUR_KEY
Content-Type: application/json

{
  "transaction": "BASE64_SERIALIZED_TRANSACTION",
  "encoding": "base64",
  "network": "solana-mainnet",
  "wallet": "OPTIONAL_WALLET"
}
```

## Decisions

The response action is one of:

- `allow`
- `warn`
- `block`
- `withhold`

A failed transaction simulation returns `block`. An unavailable RPC provider or a simulation without evidence returns `withhold` rather than a fabricated safe result.

## Evidence inspected

The first rule set checks simulation evidence for:

- program upgrade instructions
- permanent delegate initialization
- authority changes
- token account freezes
- account owner assignment
- account closure
- delegate approval
- token burn
- transfer-hook execution
- high compute consumption
- broad program call surfaces

Rules are intentionally conservative. A missing signal does not guarantee safety.

## HTML surface

Open:

```text
/transaction-firewall
```

The page uses plain HTML, CSS and browser JavaScript. No framework or TypeScript runtime is required.

## Configuration

The firewall is enabled by default. It can be disabled without a code deployment:

```env
KOSCHEI_TRANSACTION_FIREWALL_ENABLED=false
```

Automatic enforcement is not part of this release.
