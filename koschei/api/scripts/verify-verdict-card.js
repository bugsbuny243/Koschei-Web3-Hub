'use strict';

const assert = require('node:assert/strict');
const { mapVerdictCard } = require('../public/js/verdict-card.js');

const dominantHolder = mapVerdictCard({
  generated_at: '2026-07-17T04:00:00Z',
  final_verdict: {
    signed: true,
    signature: 'koschei-unified-contract:test-signature',
    verdict: 'no_grade_trigger',
    grade: '-',
    ruleset_version: 'koschei-unified-radar-rules-v1.0.0'
  },
  threat_anticipation: {
    exit_capacity: {
      available: true,
      capacity: 'limited',
      owner_percentage: 36.0263,
      owner_reference_usd_value: 1116150.23,
      liquidity_usd: 181694.5,
      position_liquidity_multiple: 6.14
    },
    pathways: [
      { id: 'dominant_holder_exit', status: 'open', capacity: 'limited', evidence_status: 'OBSERVED', summary: 'open holder exit path' },
      { id: 'mint_inflation', status: 'closed' }
    ]
  }
}, { lang: 'tr' });

assert.equal(dominantHolder.header.state, 'signed_finding');
assert.match(dominantHolder.header.title, /BASKIN HOLDER/);
assert.match(dominantHolder.header.copy, /36,0263/);
assert.match(dominantHolder.header.copy, /6,14/);
assert.doesNotMatch(dominantHolder.header.title, /KANIT PENCERESİ AKTİF/);
assert.match(dominantHolder.checklist.find(row => row.id === 'dominant-exit').value, /LIMITED/);

const liquidity = mapVerdictCard({
  generated_at: '2026-07-17T04:00:00Z',
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
  generated_at: '2026-07-17T04:00:00Z',
  source_context: { launch_time: '2026-07-17T00:00:00Z' },
  final_verdict: { signed: false, verdict: 'no_grade_trigger', grade: '-' }
}, { lang: 'tr' });

assert.equal(unsigned.header.state, 'gathering');
assert.equal(unsigned.header.title, 'KANIT PENCERESİ AKTİF');
assert.equal(unsigned.checklist.length, 20);
assert.equal(unsigned.checklist.find(row => row.id === 'claim').state, 'not_applicable');
assert.equal(unsigned.checklist.find(row => row.id === 'metadata').state, 'window_open');
assert.equal(JSON.stringify(unsigned).toLowerCase().includes('veri yok'), false);
assert.equal(JSON.stringify(mapVerdictCard({}, { lang: 'en' })).toLowerCase().includes('no data'), false);

console.log('verdict-card completeness assertions passed');
