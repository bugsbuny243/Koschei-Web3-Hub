'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const filename = path.join(__dirname, '..', 'customer-universal-address-scan-v1.js');
const original = fs.readFileSync(filename, 'utf8');
const marker = "document.readyState==='loading'?";
assert.ok(original.includes(marker), 'customer script should expose stable loading guard');
const instrumented = original.replace(marker,
  "window.__testCustomerDecisionPolicy=customerDecisionAvailability;window.__testCustomerCard=resultCard;\n" + marker);
const ctx = {
  window: {},
  document: {readyState: 'loading', addEventListener() {}},
};
vm.runInNewContext(instrumented, ctx, {filename});
const policy = ctx.window.__testCustomerDecisionPolicy;
const card = ctx.window.__testCustomerCard;

function result(network, verdictAuthority, executable = true) {
  return {
    target: {raw: 'fixture-target', network_hint: network},
    status: 'observed', evidence_status: 'observed',
    investigation_plan: {executable, verdict_authority: verdictAuthority},
    trust: {observed:true}, evidence_refs:['fixture-evidence'],
  };
}

test('customer EVM and Bitcoin read-only evidence never becomes a signed grade', () => {
  for (const network of ['ethereum-mainnet', 'base-mainnet', 'arbitrum-mainnet', 'bitcoin-mainnet']) {
    const customer = result(network, 'evidence_only_no_grade_authority');
    assert.match(policy(customer).label, /EVIDENCE ONLY/);
    const html = card({ok:true,network,label:network,data:{result:customer}});
    assert.match(html, /NO RISK GRADE/);
    assert.match(html, /not proof of safety/);
  }
});

test('customer sees canonical decision boundary instead of owner-only optimism', () => {
  const customer = result('solana-mainnet', 'existing_deterministic_rules_only');
  assert.equal(policy(customer).label, 'SIGNED VERDICT NOT PROVIDED');
  const html = card({ok:true,network:'solana-mainnet',label:'Solana',data:{result:customer}});
  assert.match(html, /Customer decision availability/);
  assert.match(html, /SIGNED VERDICT NOT PROVIDED/);
  assert.doesNotMatch(html, /SAFE TO INVEST/);
});

test('unconnected, unknown and ambiguous networks remain unknown to customer', () => {
  assert.equal(policy(result('sui-mainnet','no_verdict_authority',false)).label, 'COLLECTOR NOT CONNECTED');
  assert.equal(policy({target:{requires_network:true}}).label, 'NETWORK REQUIRED');
  assert.equal(policy({target:{raw:'unknown'}}).label, 'DECISION NOT AVAILABLE');
  assert.equal(policy(result('unclassified','no_verdict_authority')).label, 'NO CONNECTED VERDICT AUTHORITY');
});

test('all customer decision labels escape untrusted result markup', () => {
  const customer = result('base-mainnet', 'evidence_only_no_grade_authority');
  customer.target.raw = '<img src=x onerror=alert(1)>';
  const html = card({ok:true,label:'Base',data:{result:customer}});
  assert.doesNotMatch(html, /<img/);
  assert.match(html, /&lt;img/);
});

test('customer sees full native EVM/Bitcoin observations and nested provenance, not just references', () => {
  for (const network of ['ethereum-mainnet', 'bitcoin-mainnet']) {
    const customer = result(network, 'evidence_only_no_grade_authority');
    customer.network_evidence = {
      id: 'ev-native-1', source: 'network-probe-fixture',
      provenance: 'synthetic-observation-source', status: 'observed',
      attributes: {collector_metric_not_in_compact_rows: 37, deep_relation: {evidence_keys: ['a', 'b']}},
    };
    customer.asset_evidence = {id:'ev-asset-1', attributes:{holder_observation:{unknown:false}}};
    const html = card({ok:true, label:network, data:{result:customer}});
    assert.match(html, /All collected evidence and provenance/);
    assert.match(html, /Native network observation — complete collector evidence/);
    assert.match(html, /synthetic-observation-source/);
    assert.match(html, /collector_metric_not_in_compact_rows/);
    assert.match(html, /deep_relation/);
    assert.match(html, /holder_observation/);
    assert.match(html, /ev-native-1/);
  }
});
