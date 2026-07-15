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
  evidence: ["mint authority is active"],
  triggered_rules: [{ rule_id: "R-01", title: "Mint authority active", evidence_status: "VERIFIED" }],
  decision_path: ["R-01 was verified.", "Grade C ceiling applied."],
  rule_version: "arvis-live",
  signed: true
};

test("validates a complete scoreless signed verdict", () => {
  const result = validateSignedVerdict(validVerdict);
  assert.equal(result.ok, true);
  assert.equal(result.errors.length, 0);
  assert.equal(isSignedVerdict(validVerdict), true);
});

test("accepts dash grade when no grade-changing rule fired", () => {
  const verdict = {
    ...validVerdict,
    grade: "-",
    triggered_rules: [],
    decision_path: ["No grade-changing rule was satisfied; absence of evidence is not an A grade."]
  };
  const result = validateSignedVerdict(verdict);
  assert.equal(result.ok, true);
  assert.equal(evaluateVerdictPolicy(verdict).decision, "withhold");
});

test("rejects unsigned or incomplete verdicts", () => {
  const result = validateSignedVerdict({ signed: false, grade: "Z" });
  assert.equal(result.ok, false);
  assert.ok(result.errors.includes("signed must be true"));
  assert.ok(result.errors.includes("evidence must be a non-empty array of strings"));
  assert.ok(result.errors.includes("rule_version is required"));
});

test("evaluates block, warn, allow and withhold policy outcomes", () => {
  assert.equal(evaluateVerdictPolicy({ ...validVerdict, grade: "D" }).decision, "block");
  assert.equal(evaluateVerdictPolicy(validVerdict).decision, "warn");
  assert.equal(evaluateVerdictPolicy({ ...validVerdict, grade: "A" }).decision, "allow");
  assert.equal(evaluateVerdictPolicy({ ...validVerdict, grade: "-" }).decision, "withhold");
  assert.equal(evaluateVerdictPolicy({ signed: false }).decision, "withhold");
});

test("supports custom grade policy", () => {
  const decision = evaluateVerdictPolicy(
    { ...validVerdict, grade: "B" },
    { blockGrades: ["E", "F"], warnGrades: ["B", "C", "D"] }
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
