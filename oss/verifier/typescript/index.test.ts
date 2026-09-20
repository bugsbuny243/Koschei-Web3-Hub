import assert from "node:assert/strict";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import { readFileSync } from "node:fs";
import test from "node:test";

import {
  canonicalVerdictPayloadBytes,
  verifySignedVerdict,
  type TrustedKeyRegistry,
} from "./index.js";

function signedFixture() {
  const { privateKey, publicKey } = generateKeyPairSync("ed25519");
  const publicJWK = publicKey.export({ format: "jwk" });
  if (typeof publicJWK.x !== "string") throw new Error("missing Ed25519 public key");

  const verdict: Record<string, unknown> = {
    target: "ExampleMint111",
    network: "solana-mainnet",
    grade: "C",
    verdict: "hard_trigger",
    evidence: ["creator reuse verified", "holder reuse observed"],
    rule_version: "koschei-unified-radar-rules-v1.4.0",
    triggered_rules: [
      {
        rule_id: "ARD-C001",
        title: "Creator reuse",
        tier: "compounding",
        evidence_status: "verified",
        grade_effect: "compounding_input",
        count: 1,
        summary: "Creator reuse was verified.",
        evidence_keys: ["creator:ExampleMint111"],
        signatures: ["solana-signature-1"],
      },
    ],
    watch_flags: [],
    decision_path: ["A verified hard trigger determined the published grade."],
    signed: true,
    signature_algorithm: "ed25519",
    key_id: "test-key-v1",
    created_at: "2026-09-20T03:00:00Z",
  };

  const payload = canonicalVerdictPayloadBytes(verdict);
  verdict.payload_hash = "sha256:" + createHash("sha256").update(payload).digest("hex");
  verdict.signature = sign(null, payload, privateKey).toString("base64url");

  const trustedKeys: TrustedKeyRegistry = { "test-key-v1": publicJWK.x };
  return { verdict, trustedKeys };
}

test("accepts a valid verdict signed by a trusted Ed25519 key", () => {
  const { verdict, trustedKeys } = signedFixture();
  assert.deepEqual(verifySignedVerdict(verdict, trustedKeys), { valid: true, errors: [] });
});

test("rejects the audit-style fake signature instead of returning valid=true", () => {
  const { verdict, trustedKeys } = signedFixture();
  verdict.signature = "invalid-signature-for-audit";
  const result = verifySignedVerdict(verdict, trustedKeys);
  assert.equal(result.valid, false);
  assert.ok(result.errors.some(error => error.includes("64-byte Ed25519 signature")));
});

test("rejects payload mutation after signing", () => {
  const { verdict, trustedKeys } = signedFixture();
  verdict.grade = "A";
  const result = verifySignedVerdict(verdict, trustedKeys);
  assert.equal(result.valid, false);
  assert.ok(result.errors.some(error => error.includes("payload_hash")));
  assert.ok(result.errors.some(error => error.includes("signature did not verify")));
});

test("rejects an unknown key id even when the signature bytes are valid", () => {
  const { verdict } = signedFixture();
  const result = verifySignedVerdict(verdict, {});
  assert.equal(result.valid, false);
  assert.ok(result.errors.some(error => error.includes("no trusted Ed25519 public key")));
});

test("rejects a forged payload hash", () => {
  const { verdict, trustedKeys } = signedFixture();
  verdict.payload_hash = "sha256:" + "0".repeat(64);
  const result = verifySignedVerdict(verdict, trustedKeys);
  assert.equal(result.valid, false);
  assert.ok(result.errors.some(error => error.includes("payload_hash")));
});


test("verifies the shared Go producer interoperability vector", () => {
  const fixture = JSON.parse(
    readFileSync(new URL("./testdata/go-producer-vector.json", import.meta.url), "utf8"),
  ) as {
    trusted_public_key: string;
    verdict: Record<string, unknown>;
  };

  const result = verifySignedVerdict(fixture.verdict, {
    "interop-key-v1": fixture.trusted_public_key,
  });
  assert.deepEqual(result, { valid: true, errors: [] });

  const payload = canonicalVerdictPayloadBytes(fixture.verdict);
  const payloadHash = "sha256:" + createHash("sha256").update(payload).digest("hex");
  assert.equal(payloadHash, fixture.verdict.payload_hash);
  assert.match(payload.toString("utf8"), /\\u003cInterop\\u003e\\u0026/);
  assert.match(payload.toString("utf8"), /\\u2029/);
});
