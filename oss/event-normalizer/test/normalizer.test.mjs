import assert from "node:assert/strict";
import test from "node:test";

import { normalizeBatch, normalizeObservation } from "../dist/index.js";

test("normalizes Pump observations into the stable contract", () => {
  const result = normalizeObservation({
    module_id: "pump_sybil_radar",
    event_type: "launch_observed",
    mint: "TOKEN_MINT",
    created_at: "2026-06-27T10:00:00Z",
    metadata: { slot: 123 }
  });

  assert.equal(result.ok, true);
  assert.deepEqual(result.observation, {
    schema_version: "1.0",
    source: "pump",
    module_id: "pump_sybil_radar",
    event_type: "launch_observed",
    signature: undefined,
    target: "TOKEN_MINT",
    target_type: "token",
    network: "solana-mainnet",
    observed_at: "2026-06-27T10:00:00.000Z",
    metadata: { slot: 123 }
  });
});

test("normalizes Raydium pool observations", () => {
  const result = normalizeObservation({
    source: "raydium",
    event_type: "pool_created",
    address: "POOL_ADDRESS",
    observed_at: "2026-06-27T10:01:00Z"
  });

  assert.equal(result.ok, true);
  assert.equal(result.observation.source, "raydium");
  assert.equal(result.observation.target_type, "pool");
});

test("rejects missing or invented evidence timestamps", () => {
  const result = normalizeObservation({
    source: "pumpportal",
    mint: "TOKEN_MINT"
  });

  assert.equal(result.ok, false);
  assert.ok(result.errors.includes("observed_at or created_at must be a valid timestamp"));
});

test("returns rejected indexes for invalid batch items", () => {
  const result = normalizeBatch([
    {
      source: "pumpportal",
      mint: "GOOD_MINT",
      observed_at: "2026-06-27T10:02:00Z"
    },
    {
      source: "unknown-provider",
      observed_at: "2026-06-27T10:02:00Z"
    }
  ]);

  assert.equal(result.observations.length, 1);
  assert.equal(result.rejected.length, 1);
  assert.equal(result.rejected[0].index, 1);
  assert.ok(result.rejected[0].errors.includes("target is required"));
});
