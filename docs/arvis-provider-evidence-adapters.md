# ARVIS External Intelligence Adapters (read-only, fail-closed)

Source contracts checked 2026-10-19. **This is an auxiliary evidence surface**, not the ARVIS v1.4 canonical decision engine. Unavailable or negative/empty provider data never means SAFE. Every answer has `risk_level:"unknown"`, `signed:false`, and `verification:"external_observation_only"`.

## Developer API

Existing Professional API-key middleware (key entitlement, rate limits, database readiness) protects:

- `POST /api/v1/intelligence/provider` — provider observation
- `GET /api/v1/intelligence/provider/capabilities` — credential configuration status only (not vendor health)

All traffic is outgoing to **fixed vendor origins**, not user-selectable URLs. Server-side keys are **never returned**. Provider failures are explicit 502/503, not synthetic positive risk decisions. The request body is limited to 64 KiB; provider responses are limited to 2 MiB.

Example request:

```json
{"provider":"goplus","operation":"token","chain":"solana","address":"So11111111111111111111111111111111111111112"}
```

## Supported operations

| Provider | Operation | Read-only vendor endpoint |
|---|---|---|
| Blockaid | `token` | `POST /v1/token/scan` |
| Blockaid | `address` | `POST /v1/address/scan` |
| Blockaid | `transaction` | `POST /v0/evm/transaction/scan` |
| Blockaid | `transaction_bulk` | `POST /v0/evm/transaction-bulk/scan` (1–20 transactions) |
| GoPlus | `token` | Solana `GET /api/v1/solana/token_security` or EVM `GET /api/v1/token_security/{chain_id}` |
| GoPlus | `address` | EVM `GET /api/v1/address_security/{address}` |
| GoPlus | `approval` | EVM `GET /api/v1/approval_security/{chain_id}` |
| GoPlus | `phishing` | `GET /api/v1/phishing_site` |
| Bubblemaps | `holders` | `GET /v0/tokens/holders/{chain}/{token_address}` |
| Bubblemaps | `map` | `GET /v0/tokens/map/{chain}/{token_address}` with nodes/relationships/clusters |
| Bubblemaps | `metrics` | `GET /v0/tokens/metrics/{chain}/{token_address}` |
| Bubblemaps | `subgraph` | `POST /v0/relationships/subgraph` (1–20 addresses) |
| Trail of Bits Slither | `findings` | Import completed Slither JSON output ONLY; no subprocess/compiler in API process |

**"CertiK Slither" is not an official combined product.** Slither belongs to Trail of Bits. CertiK services and audit credentials would be a distinct integration.

## Credentials

- `KOSCHEI_BLOCKAID_API_KEY`
- `KOSCHEI_GOPLUS_ACCESS_TOKEN`
- `KOSCHEI_BUBBLEMAPS_API_KEY`

No vendor calls are made if the matching credential is unset. Providers may require a commercial API plan, quotas, or signed agreements. Do not put vendor keys in HTML or in browser requests.

## Deliberate exclusions and remaining acceptance

- This does **not** complete the still-closed automatic ARVIS stream verdict worker (#1417).
- An external vendor "safe" label does not replace native Solana source proof, actor/behavior verified rule evidence, or trusted Ed25519 signature authentication.
- **Blockaid change tracking/webhooks** require webhook signature verification, persistent per-token checkpoints, dedupe and recovery; no subscription is activated by this read-only API.
- **GoPlus MCP transport** requires a separate MCP server registration and tool resource lifecycle. These endpoints expose GoPlus REST intelligence, not an MCP server.
- Bubblemaps Data API answers are customer-visible observations but graph edges are not automatically classified as criminal or malicious.
- Production acceptance needs valid vendor credentials, controlled live sample responses, per-account quotas, network egress review, request latency/cost metrics and client-side docs.
- Solidity/EVM-only Slither cannot audit arbitrary Solana Rust/Anchor bytecode. An isolated pinned toolchain with an input corpus is needed before any hosted static scanning.

References: Blockaid OpenAPI v1 docs; GoPlus API reference; Bubblemaps Data API OpenAPI; Trail of Bits crytic/slither JSON output schema.
