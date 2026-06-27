import assert from "node:assert/strict";
import test from "node:test";

import {
  ArvisApiError,
  ArvisClient,
  evaluateVerdictPolicy,
  isSignedVerdict,
  validateSignedVerdict
} from "../dist/index.js";

const validVerdict = {
  target: "SOLANA_TARGET",
  network: "solana-mainnet",
  grade: "C",
  risk_index: 45,
  risk_level: "medium",
  evidence: ["mint authority is active"],
  rule_version: "arvis-live",
  signed: true
};

test("validates a complete signed verdict", () => {
  const result = validateSignedVerdict(validVerdict);
  assert.equal(result.ok, true);
  assert.equal(result.errors.length, 0);
  assert.equal(isSignedVerdict(validVerdict), true);
});

test("rejects unsigned or incomplete verdicts", () => {
  const result = validateSignedVerdict({
    signed: false,
    risk_index: 120,
    grade: "Z",
    risk_level: "unknown"
  });
  assert.equal(result.ok, false);
  assert.ok(result.errors.includes("signed must be true"));
  assert.ok(result.errors.includes("risk_index must be between 0 and 100"));
  assert.ok(result.errors.includes("evidence must be a non-empty array of strings"));
});

test("rejects fractional risk indexes", () => {
  const result = validateSignedVerdict({ ...validVerdict, risk_index: 45.5 });
  assert.equal(result.ok, false);
  assert.ok(result.errors.includes("risk_index must be an integer"));
});

test("evaluates block, warn, allow and withhold policy outcomes", () => {
  assert.equal(evaluateVerdictPolicy({ ...validVerdict, risk_index: 85, risk_level: "critical" }).decision, "block");
  assert.equal(evaluateVerdictPolicy(validVerdict).decision, "warn");
  assert.equal(evaluateVerdictPolicy({ ...validVerdict, grade: "A", risk_index: 12, risk_level: "low" }).decision, "allow");
  assert.equal(evaluateVerdictPolicy({ signed: false }).decision, "withhold");
});

test("supports custom policy thresholds", () => {
  const decision = evaluateVerdictPolicy(
    { ...validVerdict, grade: "B", risk_index: 35, risk_level: "low" },
    { warnAt: 30, blockAt: 90, blockLevels: ["critical"], warnLevels: [] }
  );
  assert.equal(decision.decision, "warn");
});

test("sends API-key requests to the live token scan route", async () => {
  let capturedUrl = "";
  let capturedInit;
  const client = new ArvisClient({
    baseUrl: "https://example.test/",
    apiKey: "test-key",
    fetchImpl: async (url, init) => {
      capturedUrl = String(url);
      capturedInit = init;
      return new Response(JSON.stringify({ request_id: "req_1", status: "queued", cost_credits: 1 }), {
        status: 200,
        headers: { "Content-Type": "application/json" }
      });
    }
  });

  const response = await client.tokenScan({ mint: "TOKEN_MINT" });
  assert.equal(capturedUrl, "https://example.test/api/v1/scan/token");
  assert.equal(new Headers(capturedInit.headers).get("X-API-Key"), "test-key");
  assert.deepEqual(JSON.parse(capturedInit.body), {
    mint: "TOKEN_MINT",
    network: "solana-mainnet",
    include_ai: false
  });
  assert.equal(response.request_id, "req_1");
});

test("queues deduplicated batch scans with an idempotency key", async () => {
  let capturedInit;
  const client = new ArvisClient({
    baseUrl: "https://example.test",
    apiKey: "test-key",
    fetchImpl: async (_url, init) => {
      capturedInit = init;
      return new Response(JSON.stringify({ request_id: "batch_1", status: "queued", cost_credits: 2 }), {
        status: 202,
        headers: { "Content-Type": "application/json" }
      });
    }
  });

  const response = await client.tokenScanBatch({
    mints: ["MINT_A", "MINT_A", " MINT_B "],
    idempotencyKey: "launchpad-screen-42"
  });
  const headers = new Headers(capturedInit.headers);
  assert.equal(headers.get("Idempotency-Key"), "launchpad-screen-42");
  assert.deepEqual(JSON.parse(capturedInit.body), {
    mints: ["MINT_A", "MINT_B"],
    network: "solana-mainnet",
    include_ai: false
  });
  assert.equal(response.request_id, "batch_1");
});

test("retrieves a completed asynchronous request", async () => {
  let capturedUrl = "";
  const client = new ArvisClient({
    baseUrl: "https://example.test",
    apiKey: "test-key",
    fetchImpl: async (url) => {
      capturedUrl = String(url);
      return new Response(JSON.stringify({
        ok: true,
        request_id: "req_1",
        endpoint: "/api/v1/scan/token",
        status: "completed",
        terminal: true,
        credits_reserved: 1,
        credits_charged: 1,
        result_available: true,
        result: { mint: "MINT_A" },
        created_at: "2026-06-27T00:00:00Z"
      }), { status: 200, headers: { "Content-Type": "application/json" } });
    }
  });

  const result = await client.requestStatus("req_1");
  assert.equal(capturedUrl, "https://example.test/api/v1/usage?request_id=req_1");
  assert.equal(result.terminal, true);
  assert.deepEqual(result.result, { mint: "MINT_A" });
});

test("returns structured API errors", async () => {
  const client = new ArvisClient({
    apiKey: "test-key",
    fetchImpl: async () => new Response(JSON.stringify({ error: "quota_exceeded" }), {
      status: 429,
      headers: { "Content-Type": "application/json" }
    })
  });

  await assert.rejects(
    () => client.usage(),
    (error) => error instanceof ArvisApiError && error.status === 429 && error.message === "quota_exceeded"
  );
});
