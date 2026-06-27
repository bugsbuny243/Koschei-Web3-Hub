# Batch Screening and Risk Policy

Koschei ARVIS supports additive developer workflows on the existing token scan and usage routes.

## Batch token screening

Send up to 20 unique Solana token mints to the existing token scan endpoint:

```http
POST /api/v1/scan/token
X-API-Key: YOUR_API_KEY
Idempotency-Key: launchpad-import-42
Content-Type: application/json

{
  "mints": ["MINT_A", "MINT_B", "MINT_C"],
  "network": "solana-mainnet"
}
```

The response returns a request ID, the number of queued targets, reserved outputs and a result URL. Each successful target consumes one output. Failed targets are refunded automatically.

## Idempotent retries

Clients should attach an `Idempotency-Key` when a request might be retried. Reusing the same key with the same API key and endpoint returns the original request instead of reserving outputs again.

Keys are limited to 128 characters.

## Result polling

```http
GET /api/v1/usage?request_id=REQUEST_ID
X-API-Key: YOUR_API_KEY
```

A non-terminal response includes `poll_after_ms`. A terminal response includes:

- request status
- outputs reserved and charged
- latency
- error or partial-failure code
- stored single or batch result

The existing usage history remains available at `GET /api/v1/usage`. Add `include_results=1` only when full stored results are needed.

## TypeScript SDK

```ts
const queued = await arvis.tokenScanBatch({
  mints: ["MINT_A", "MINT_B"],
  idempotencyKey: "launchpad-import-42"
});

const completed = await arvis.waitForRequest(queued.request_id);
console.log(completed.result);
```

## Risk policy engine

The SDK converts a structurally valid signed verdict into one deterministic integration action:

```ts
const decision = evaluateVerdictPolicy(verdict, {
  blockAt: 70,
  warnAt: 40,
  blockLevels: ["high", "critical"],
  warnLevels: ["medium"]
});
```

Possible decisions:

- `block`
- `warn`
- `allow`
- `withhold`

Unsigned, invalid or incomplete verdicts always return `withhold`; they never silently become `allow`.

## Compatibility

Existing single-token requests remain unchanged. The new batch body, idempotency header and result query are optional additions to the current API contract.
