# Solana Event Normalizer

A small open-source helper that converts Pump-style and Raydium-style observations into one stable event contract for downstream risk processing.

## Why

Different data sources expose different field names. Security infrastructure needs a predictable target, source, target type, network, timestamp and metadata shape before evidence processing begins.

## Example

```ts
import { normalizeObservation } from "@koschei/solana-event-normalizer";

const result = normalizeObservation({
  module_id: "pump_sybil_radar",
  event_type: "launch_observed",
  mint: "SOLANA_TOKEN_MINT",
  target_type: "token",
  created_at: new Date().toISOString()
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

## Trust boundary

This package only normalizes observations. It does not assign a risk score or claim that an event is malicious. Evidence processing and final verdict generation must remain separate layers.
