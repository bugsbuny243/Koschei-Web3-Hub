import assert from "node:assert/strict";
import test from "node:test";

import {
  ArvisApiError,
  ArvisClient,
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
