'use strict';

const assert = require('node:assert/strict');
const { mapVerdictCard } = require('../public/js/verdict-card.js');

const dominantHolder = mapVerdictCard({
  final_verdict: {
    signed: true,
    signature: 'koschei-unified-contract:test-signature',
    verdict: 'no_grade_trigger',
    grade: '-',
    ruleset_version: 'koschei-unified-radar-rules-v1.0.0'
  },
  threat_anticipation: {
    exit_capacity: {
      owner_percentage: 36.0263,
      position_liquidity_multiple: 6.14
    },
    pathways: [
      { id: 'dominant_holder_exit', status: 'open', summary: 'open holder exit path' },
      { id: 'mint_inflation', status: 'closed' }
    ]
  }
}, { lang: 'tr' });

assert.equal(dominantHolder.header.state, 'signed_finding');
assert.match(dominantHolder.header.title, /BASKIN HOLDER/);
assert.match(dominantHolder.header.copy, /36,0263/);
assert.match(dominantHolder.header.copy, /6,14/);
assert.doesNotMatch(dominantHolder.header.title, /KANIT TOPLANIYOR/);

const liquidity = mapVerdictCard({
  final_verdict: {
    signed: true,
    signature: 'koschei-unified-contract:liquidity',
    verdict: 'no_grade_trigger',
    grade: '-'
  },
  threat_anticipation: {
    pathways: [
      { id: 'liquidity_removal', status: 'observed', summary: 'Transaction-backed liquidity removal observed.' },
      { id: 'dominant_holder_exit', status: 'open' }
    ]
  }
}, { lang: 'tr' });

assert.match(liquidity.header.title, /LİKİDİTE/);

const unsigned = mapVerdictCard({
  final_verdict: { signed: false, verdict: 'no_grade_trigger', grade: '-' }
}, { lang: 'tr' });

assert.equal(unsigned.header.state, 'gathering');
assert.equal(unsigned.header.title, 'KANIT TOPLANIYOR');

console.log('verdict-card case-specific assertions passed');
