import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const schemaURL = new URL("../../schemas/signed-verdict.schema.json", import.meta.url);

function validateAgainstContract(schema, value) {
  const errors = [];
  for (const key of schema.required ?? []) {
    if (!(key in value)) errors.push(`missing required ${key}`);
  }
  const gradePattern = new RegExp(schema.properties.grade.pattern);
  if (!gradePattern.test(value.grade)) errors.push("grade pattern mismatch");
  if (schema.properties.signed.const !== value.signed) errors.push("signed mismatch");
  if (!Array.isArray(value.evidence) || value.evidence.length < schema.properties.evidence.minItems) {
    errors.push("evidence mismatch");
  }
  return errors;
}

test("signed verdict schema accepts dash grade without numeric score", async () => {
  const schema = JSON.parse(await readFile(schemaURL, "utf8"));
  const verdict = {
    grade: "-",
    evidence: ["No grade-changing rule was satisfied; absence of evidence is not an A grade."],
    triggered_rules: [],
    decision_path: ["No grade-changing rule was satisfied."],
    rule_version: "koschei-unified-radar-rules-v1.0.0",
    signed: true
  };

  assert.deepEqual(validateAgainstContract(schema, verdict), []);
  assert.deepEqual(schema.required, ["grade", "evidence", "rule_version", "signed"]);
  assert.equal("risk_index" in schema.properties, false);
  assert.equal(schema.properties.grade.pattern, "^[A-F-]$");
  assert.ok("triggered_rules" in schema.properties);
  assert.ok("decision_path" in schema.properties);
});
