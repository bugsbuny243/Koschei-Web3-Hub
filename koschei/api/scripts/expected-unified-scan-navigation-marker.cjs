#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');

function expectedNavigationMarker(source) {
  // Derive deployment contract from the checked-out frontend source.
  // If the bootstrap is absent or ambiguous, fail closed rather than
  // accepting a stale asset or silently skipping the production probe.
  const pattern = /\bscript\.src\s*=\s*(['"])(\/js\/unified-scan-navigation\.js\?v=[1-9]\d*)\1\s*;/g;
  const matches = [...String(source).matchAll(pattern)];
  if (matches.length !== 1) {
    throw new Error(`Expected exactly one versioned unified scan bootstrap, found ${matches.length}`);
  }
  const [, quote, asset] = matches[0];
  return `script.src=${quote}${asset}${quote}`;
}

if (require.main === module) {
  try {
    const sourceFile = path.resolve(__dirname, '..', 'public', 'js', 'public-solana-scan.js');
    process.stdout.write(expectedNavigationMarker(fs.readFileSync(sourceFile, 'utf8')));
  } catch (error) {
    process.stderr.write(`UNIFIED_SCAN_CONTRACT_FAILURE: ${error.message}\n`);
    process.exitCode = 1;
  }
}

module.exports = { expectedNavigationMarker };
