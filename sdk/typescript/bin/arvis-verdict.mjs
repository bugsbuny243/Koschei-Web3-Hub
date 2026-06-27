#!/usr/bin/env node

import { readFile } from "node:fs/promises";
import process from "node:process";

import { validateSignedVerdict } from "../dist/index.js";

async function readStdin() {
  const chunks = [];
  for await (const chunk of process.stdin) chunks.push(chunk);
  return Buffer.concat(chunks).toString("utf8");
}

async function main() {
  const file = process.argv[2];
  const raw = file ? await readFile(file, "utf8") : await readStdin();
  if (!raw.trim()) {
    throw new Error("Provide a verdict JSON file or pipe JSON through stdin.");
  }

  let payload;
  try {
    payload = JSON.parse(raw);
  } catch (error) {
    throw new Error(`Invalid JSON: ${error instanceof Error ? error.message : String(error)}`);
  }

  const validation = validateSignedVerdict(payload);
  process.stdout.write(`${JSON.stringify(validation, null, 2)}\n`);
  process.exitCode = validation.ok ? 0 : 2;
}

main().catch((error) => {
  process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
});
