'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const { observeHealthLiveness } = require('./verify-public-health-liveness.cjs');

test('stateless healthy endpoint is liveness, not PostgreSQL readiness', () => {
  const observed = observeHealthLiveness('{"status":"ok","persistence":"stateless","database":"not_used"}');
  assert.equal(observed.database_connectivity, 'not_probed');
  assert.equal(observed.operational_readiness, 'not_evaluated');
  assert.equal(observed.persistence, 'stateless');
});

test('configured PostgreSQL is not a verified connectivity claim', () => {
  const observed = observeHealthLiveness({status:'ok',persistence:'postgresql',database:'configured'});
  assert.equal(observed.database, 'configured');
  assert.equal(observed.database_connectivity, 'not_probed');
});

test('rejects inconsistent, missing or unhealthy responses', () => {
  for (const payload of [
    {status:'fail',persistence:'postgresql',database:'configured'},
    {status:'ok',persistence:'postgresql',database:'not_used'},
    {status:'ok',persistence:'stateless',database:'configured'},
    {status:'ok'},
  ]) assert.throws(() => observeHealthLiveness(payload));
  assert.throws(() => observeHealthLiveness('{'));
});
