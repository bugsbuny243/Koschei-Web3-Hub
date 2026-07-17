import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
const require=createRequire(import.meta.url);
const {mapVerdictCard}=require('../verdict-card.js');

const fixture=name=>JSON.parse(readFileSync(new URL(`../__fixtures__/${name}`,import.meta.url)));
const unfinished=vm=>vm.checklist.filter(row=>row.state==='window_open'||row.state==='arm_pending');
const assertNoBannedEmptyCopy=value=>{
  const text=JSON.stringify(value).toLowerCase();
  assert.equal(text.includes('veri yok'),false);
  assert.equal(text.includes('no data'),false);
};

test('mint+freeze active + 72% owner-resolved maps red leverage rows',()=>{
  const vm=mapVerdictCard({generated_at:'2026-07-15T00:00:00Z',final_verdict:{grade:'F',ruleset_version:'unified-radar-v1.0',actor_ruleset_version:'actor-v1.0',signature:'1234567890abcdef',generated_at:'2026-07-15T00:00:00Z',signed:true},modules:[{module_id:'token_authority_scanner',evidence_status:'VERIFIED',signed:true,signals:{execution_status:'completed',mint_authority_present:true,freeze_authority_present:true}},{module_id:'holder_concentration',evidence_status:'VERIFIED',signed:true,metrics:{owner_resolved_top_holder_pct:72}}]});
  assert.equal(vm.checklist.find(x=>x.id==='mint').status,'green');
  assert.equal(vm.checklist.find(x=>x.id==='freeze').status,'green');
  assert.equal(vm.checklist.find(x=>x.id==='concentration').status,'green');
  assert.deepEqual(vm.leverage.map(x=>x.id),['mint-authority','freeze-authority','top-owner']);
  assert.match(vm.leverage.find(x=>x.id==='top-owner').text,/72%/);
});

test('raw-only concentration is observed and never owner leverage',()=>{
  const vm=mapVerdictCard({generated_at:'2026-07-15T00:00:00Z',final_verdict:{grade:'D'},modules:[{module_id:'holder_concentration',evidence_status:'VERIFIED',signed:true,metrics:{top_holder_percentage:70}}]});
  const row=vm.checklist.find(x=>x.id==='concentration');
  assert.equal(row.state,'observed');
  assert.equal(row.value,'70% (raw)');
  assert.equal(vm.leverage.some(x=>x.id==='top-owner'),false);
});

test('CFPk live fixture fills stored launch, exit, liquidity and ledger facts',()=>{
  const vm=mapVerdictCard(fixture('cfpk-live-scan.json'));
  assert.equal(vm.schema_version,'koschei-verdict-card-v4');
  assert.equal(vm.checklist.length,20);
  assert.equal(unfinished(vm).length<=5,true,JSON.stringify(unfinished(vm)));
  const launch=vm.checklist.find(x=>x.id==='launch');
  const exit=vm.checklist.find(x=>x.id==='dominant-exit');
  const liquidity=vm.checklist.find(x=>x.id==='liquidity');
  const movement=vm.checklist.find(x=>x.id==='liq-move');
  const wash=vm.checklist.find(x=>x.id==='wash');
  assert.match(launch.value,/4 hours old/);
  assert.match(exit.value,/LIMITED/);
  assert.match(exit.value,/6\.14x liquidity/);
  assert.match(liquidity.value,/181,694\.50/);
  assert.equal(movement.state,'observed');
  assert.match(wash.value,/64 trades/);
  assert.equal(vm.coverage.verified+vm.coverage.observed+vm.coverage.window_open+vm.coverage.not_applicable+vm.coverage.arm_pending,20);
  assertNoBannedEmptyCopy(vm);
  assertNoBannedEmptyCopy(mapVerdictCard(fixture('cfpk-live-scan.json'),{lang:'tr'}));
});

test('historical 58.75% fixture leaves at most two unfinished rows',()=>{
  const vm=mapVerdictCard(fixture('historical-scan.json'));
  assert.equal(unfinished(vm).length<=2,true,JSON.stringify(unfinished(vm)));
  assert.equal(vm.checklist.find(x=>x.id==='concentration').value,'58.75%');
  assert.equal(vm.leverage.some(x=>x.id==='top-owner'),true);
  assertNoBannedEmptyCopy(vm);
});

test('gaps are machine-distinguished instead of one generic empty label',()=>{
  const vm=mapVerdictCard({generated_at:'2026-07-17T04:00:00Z',source_context:{launch_time:'2026-07-17T00:00:00Z'},final_verdict:{verdict:'no_grade_trigger'}});
  assert.equal(vm.header.state,'gathering');
  assert.equal(vm.checklist.find(x=>x.id==='metadata').state,'window_open');
  assert.equal(vm.checklist.find(x=>x.id==='claim').state,'not_applicable');
  assert.equal(vm.checklist.find(x=>x.id==='mev').state,'not_applicable');
  assertNoBannedEmptyCopy(vm);
});

test('stable as-of input makes launch age deterministic',()=>{
  const payload={source_context:{launch_time:'2026-07-17T00:00:00Z'},final_verdict:{verdict:'no_grade_trigger'}};
  const a=mapVerdictCard(payload,{asOf:'2026-07-17T04:00:00Z'});
  const b=mapVerdictCard(payload,{asOf:'2026-07-17T04:00:00Z'});
  assert.deepEqual(a,b);
  assert.match(a.checklist.find(x=>x.id==='launch').value,/4 hours old/);
});
