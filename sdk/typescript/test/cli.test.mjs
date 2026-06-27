import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

const cli = fileURLToPath(new URL("../bin/arvis-verdict.mjs", import.meta.url));

const validVerdict = {
  grade: "B",
  risk_index: 28,
  risk_level: "low",
  evidence: ["mint authority revoked"],
  rule_version: "arvis-live",
  signed: true
};

test("CLI accepts a structurally valid signed verdict", () => {
  const result = spawnSync(process.execPath, [cli], {
    input: JSON.stringify(validVerdict),
    encoding: "utf8"
  });

  assert.equal(result.status, 0, result.stderr);
  assert.equal(JSON.parse(result.stdout).ok, true);
});

test("CLI withholds invalid verdicts with exit code 2", () => {
  const result = spawnSync(process.execPath, [cli], {
    input: JSON.stringify({ signed: false, risk_index: 101 }),
    encoding: "utf8"
  });

  assert.equal(result.status, 2, result.stderr);
  const output = JSON.parse(result.stdout);
  assert.equal(output.ok, false);
  assert.ok(output.errors.includes("signed must be true"));
});
