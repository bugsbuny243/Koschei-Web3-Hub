# Koschei ARVIS TypeScript SDK

A small dependency-free TypeScript client for the current Koschei ARVIS production routes.

## Install locally

```bash
cd sdk/typescript
npm install
npm run check
npm run build
```

## API-key example

```ts
import { ArvisClient } from "@koschei/arvis-sdk";

const arvis = new ArvisClient({
  baseUrl: "https://tradepigloball.co",
  apiKey: process.env.ARVIS_API_KEY
});

const queued = await arvis.tokenScan({
  mint: "SOLANA_TOKEN_MINT"
});

console.log(queued.request_id, queued.status);
```

## Shield preflight example

```ts
const result = await arvis.shieldPreflight({
  target: "SOLANA_TARGET",
  wallet: "OPTIONAL_WALLET"
});

console.log(result.action, result.grade, result.risk_index);
```

## Session-authenticated radar example

```ts
const radar = new ArvisClient({
  bearerToken: "CUSTOMER_SESSION_TOKEN"
});

const result = await radar.radarCheck({
  target: "SOLANA_TARGET"
});
```

## Authentication

- Partner routes accept `X-API-Key`.
- Customer radar routes accept `Authorization: Bearer <session-token>` and require an active entitlement.
- API keys are created from the authenticated account API-key screen and are shown only once.

## Live routes included

- `POST /api/v1/radar/check`
- `GET /api/v1/radar/feed`
- `POST /api/v1/scan/token`
- `POST /api/v1/shield/preflight`
- `GET /api/v1/usage`

## Trust rule

A signed verdict should be treated as final only when `signed === true`. Consumers should also inspect evidence and rule metadata rather than relying on the numeric score alone.
