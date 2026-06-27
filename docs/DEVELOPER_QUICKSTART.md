# Developer Quickstart

This guide helps external developers understand and evaluate the Koschei API without changing production infrastructure.

## What Koschei exposes

Koschei currently provides product-facing surfaces for:

- unified token, wallet and transaction analysis
- ARVIS radar checks
- authenticated live verdict feeds
- public, rate-limited risk badges
- sanitized service and pipeline health

Current routes are documented in the root `README.md`.

## Local verification

Requirements:

- Go 1.23+
- PostgreSQL-compatible database for integration paths
- a Solana RPC endpoint for live evidence collection

Run the API checks:

```bash
git clone https://github.com/bugsbuny243/Koschei-Web3-Hub.git
cd Koschei-Web3-Hub/koschei/api
go test ./...
go vet ./...
go build ./...
```

These commands verify the Go codebase without exposing production credentials.

## Configuration principles

Koschei keeps secrets outside the repository. Production credentials must be supplied through the deployment environment.

Important non-secret runtime controls include:

```env
PORT=10000
KOSCHEI_AUTO_RADAR_ENABLED=1
ARVIS_HEARTBEAT_SECONDS=20
ARVIS_STREAM_VERDICT_SECONDS=12
RAYDIUM_PROGRAM_ID=
PUMP_FUN_PROGRAM_ID=
```

The canonical Solana RPC resolution order is:

1. `SOLANA_RPC_URL`
2. configured provider URL
3. configured Alchemy API key
4. Solana public mainnet fallback

## Response model

Koschei responses are evidence-first. Consumers should expect three important states:

- evidence-backed result: a verdict may be returned
- insufficient evidence: no score or signed verdict is produced
- degraded dependency: the response should surface the dependency state rather than invent a conclusion

Integrations should never interpret missing evidence as a low-risk result.

## Recommended integration flow

1. Submit a supported target to the relevant API surface.
2. Read the evidence state before reading the score or verdict.
3. Display the underlying findings and evidence boundary.
4. Treat recommendations as security context, not financial advice.
5. Retry only when the response identifies a temporary dependency or processing state.

## Public integration roadmap

The external developer surface is being formalized around:

- stable request and response schemas
- copy-paste examples
- explicit authentication and rate-limit behavior
- versioned API documentation
- reusable transaction-decoding and security-summary components

Until the public schema is frozen, integrations should treat undocumented fields as internal and subject to change.
