# Koschei ARVIS TypeScript SDK

A dependency-free TypeScript client for the current Koschei ARVIS production routes.

## Validate locally

```bash
cd sdk/typescript
npm install
npm run check
npm test
npm pack --dry-run
```

The package requires Node.js 18 or newer.

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

console.log(result.action, result.grade, result.triggered_rules);
```

## Signed verdict validation

```ts
import { validateSignedVerdict } from "@koschei/arvis-sdk";

const validation = validateSignedVerdict(result);
if (!validation.ok) {
  console.log({ action: "withhold", errors: validation.errors });
} else {
  console.log(
    validation.verdict.grade,
    validation.verdict.triggered_rules,
    validation.verdict.decision_path
  );
}
```

Structural validation requires:

- `signed === true`
- an A-F grade, or `-` when no grade-changing rule was triggered
- a non-empty evidence array
- a rule version
- valid deterministic rule objects when `triggered_rules` is present
- non-empty decision steps when `decision_path` is present

A `-` grade is not an A grade and does not mean the target is safe. The default SDK policy withholds when no grade-changing rule was triggered.

This function validates the developer contract. It does not replace cryptographic verification when a signature-verification mechanism is available.

## Verdict CLI

The package includes a zero-dependency command-line validator.

```bash
arvis-verdict verdict.json
cat verdict.json | arvis-verdict
```

Exit codes:

- `0`: structurally valid signed verdict
- `1`: unreadable input or invalid JSON
- `2`: parsed JSON does not satisfy the signed-verdict contract

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

A result is not a final signed verdict because it contains a score. Koschei does not use a numeric final risk score. Consumers validate signed status, grade, evidence, triggered rules and decision path, and withhold the final UI decision when required fields are missing.
