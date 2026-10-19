#!/usr/bin/env node
'use strict';

const fs = require('node:fs');

function observeHealthLiveness(raw) {
  const body = typeof raw === 'string' ? JSON.parse(raw) : raw;
  if (!body || body.status !== 'ok') {
    throw new Error('public health liveness status is not ok');
  }
  const persistence = String(body.persistence || '');
  const database = String(body.database || '');
  const expected = { stateless: 'not_used', postgresql: 'configured' };
  if (!Object.hasOwn(expected, persistence) || expected[persistence] !== database) {
    throw new Error('public health persistence/database mode is missing or inconsistent');
  }
  // A public /health reply is a liveness snapshot, NEVER a DB connectivity
  // assertion, entitlement check, schema verification or worker readiness proof.
  return {
    status: 'ok',
    persistence,
    database,
    database_connectivity: 'not_probed',
    operational_readiness: 'not_evaluated'
  };
}

if (require.main === module) {
  try {
    if (process.argv.length !== 3) throw new Error('usage: verify-public-health-liveness.cjs /path/to/health.json');
    process.stdout.write(JSON.stringify(observeHealthLiveness(fs.readFileSync(process.argv[2], 'utf8'))));
  } catch (error) {
    process.stderr.write(`PUBLIC_HEALTH_CONTRACT_FAILURE: ${error.message}\n`);
    process.exitCode = 1;
  }
}

module.exports = { observeHealthLiveness };
