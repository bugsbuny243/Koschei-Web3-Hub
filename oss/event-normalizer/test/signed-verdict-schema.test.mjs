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

test("signed verdict schema accepts '-' without numeric risk fields", () => {
  const verdict = {
    grade: "-",
    evidence: ["No grade-changing rule was triggered in the evaluated evidence set."],
    rule_version: "koschei-unified-radar-rules-v1.0.0",
    signed: true,
    triggered_rules: [],
    decision_path: ["No grade-changing rule was satisfied; absence of evidence is not an A grade."]
  };

  assert.equal(validate(verdict), true, JSON.stringify(validate.errors));
  assert.equal(Object.hasOwn(verdict, "risk_index"), false);
  assert.equal(Object.hasOwn(verdict, "risk_level"), false);
});

test("signed verdict schema no longer requires or defines risk_index", () => {
  assert.deepEqual(schema.required, ["grade", "evidence", "rule_version", "signed"]);
  assert.equal(Object.hasOwn(schema.properties, "risk_index"), false);
  assert.equal(Object.hasOwn(schema.properties, "risk_level"), false);
  assert.ok(schema.properties.triggered_rules);
  assert.ok(schema.properties.decision_path);
});
