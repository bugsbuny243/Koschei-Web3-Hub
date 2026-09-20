import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import Ajv2020 from "ajv/dist/2020.js";
import addFormats from "ajv-formats";

const schemaUrl = new URL("../../schemas/signed-verdict.schema.json", import.meta.url);
const schema = JSON.parse(await readFile(schemaUrl, "utf8"));
const ajv = new Ajv2020({ allErrors: true, strict: true });
addFormats(ajv);
const validate = ajv.compile(schema);

test("signed verdict schema accepts an authenticated no-grade contract shape", () => {
  const verdict = {
    target: "ExampleMint111",
    network: "solana-mainnet",
    grade: "-",
    verdict: "no_grade_trigger",
    evidence: ["No grade-changing rule was triggered in the evaluated evidence set."],
    rule_version: "koschei-unified-radar-rules-v1.4.0",
    triggered_rules: [],
    watch_flags: [],
    decision_path: ["No grade-changing rule was satisfied; absence of evidence is not an A grade."],
    signed: true,
    signature: "A".repeat(86),
    signature_algorithm: "ed25519",
    key_id: "koschei-test-key-v1",
    payload_hash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    created_at: "2026-09-20T03:00:00Z"
  };

  assert.equal(validate(verdict), true, JSON.stringify(validate.errors));
  assert.equal(Object.hasOwn(verdict, "risk_index"), false);
  assert.equal(Object.hasOwn(verdict, "risk_level"), false);
});

test("signed verdict schema requires cryptographic binding fields", () => {
  assert.deepEqual(schema.required, [
    "target",
    "network",
    "grade",
    "verdict",
    "evidence",
    "rule_version",
    "triggered_rules",
    "decision_path",
    "signed",
    "signature",
    "signature_algorithm",
    "key_id",
    "payload_hash",
    "created_at"
  ]);
  assert.equal(Object.hasOwn(schema.properties, "risk_index"), false);
  assert.equal(Object.hasOwn(schema.properties, "risk_level"), false);
  assert.ok(schema.properties.triggered_rules);
  assert.ok(schema.properties.watch_flags);
  assert.match(schema.properties.signature.pattern, /86/);
});
