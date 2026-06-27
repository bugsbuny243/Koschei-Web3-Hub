# Solana Event Normalizer

A dependency-free open-source helper that converts Pump-style and Raydium-style observations into one deterministic, versioned event contract for downstream risk processing.

## Validate locally

```bash
cd oss/event-normalizer
npm install
npm run check
npm test
npm pack --dry-run
```

## Why

Different data sources expose different field names. Security infrastructure needs a predictable source, target, target type, network, timestamp and metadata shape before evidence processing begins.

The normalizer rejects missing targets, unclassified sources and invalid timestamps. It does not silently replace missing evidence time with the current clock.

## Example

```ts
import { normalizeObservation } from "@koschei/solana-event-normalizer";

const result = normalizeObservation({
  module_id: "pump_sybil_radar",
  event_type: "launch_observed",
  mint: "SOLANA_TOKEN_MINT",
  target_type: "token",
  created_at: "2026-06-27T10:00:00Z"
});

if (!result.ok) {
  console.error(result.errors);
} else {
  console.log(result.observation);
}
```

## Normalized output

```ts
{
  schema_version: "1.0";
  source: "pump" | "raydium" | "unknown";
  module_id: string;
  event_type: string;
  signature?: string;
  target: string;
  target_type: "token" | "pool" | "wallet" | "program" | "transaction" | "unknown";
  network: string;
  observed_at: string;
  metadata: Record<string, unknown>;
}
```

## Batch processing

`normalizeBatch` returns accepted observations separately from rejected input indexes and validation errors. This lets a collector quarantine invalid observations without treating them as verified evidence.

## Trust boundary

This package only normalizes observations. It does not assign a risk score or claim that an event is malicious. Evidence processing and final verdict generation remain separate layers.
