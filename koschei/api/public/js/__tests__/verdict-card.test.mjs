import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
const require=createRequire(import.meta.url);
const {mapVerdictCard}=require('../verdict-card.js');

test('mint+freeze active + 72% holder maps red rows and leverage exact values',()=>{
  const vm=mapVerdictCard({final_verdict:{grade:'F',ruleset_version:'unified-radar-v1.0',actor_ruleset_version:'actor-v1.0',signature:'1234567890abcdef',generated_at:'2026-07-15T00:00:00Z'},modules:[{module_id:'token_authority_scanner',evidence_status:'VERIFIED',signals:{mint_authority_present:true,freeze_authority_present:true}},{module_id:'holder_concentration',evidence_status:'VERIFIED',metrics:{owner_resolved_top_holder_pct:72}}]});
  assert.equal(vm.checklist.find(x=>x.id==='mint').status,'red');
  assert.equal(vm.checklist.find(x=>x.id==='freeze').status,'red');
  assert.equal(vm.checklist.find(x=>x.id==='concentration').status,'red');
  assert.deepEqual(vm.leverage.map(x=>x.id),['mint-authority','freeze-authority','top-owner']);
  assert.match(vm.leverage.find(x=>x.id==='top-owner').text,/72%/);
});

test('all evidence pending maps neutral evidence gathering and gray checklist',()=>{
  const vm=mapVerdictCard({final_verdict:{verdict:'no_grade_trigger',ruleset_version:'unified-radar-v1.0',actor_ruleset_version:'actor-v1.0'}});
  assert.equal(vm.header.state,'gathering');
  assert.equal(vm.header.tone,'neutral');
  assert.equal(vm.leverage.length,0);
  assert.equal(vm.checklist.every(x=>x.status==='gray'),true);
});

test('INFERRED-only concentration is yellow checklist with no leverage',()=>{
  const vm=mapVerdictCard({final_verdict:{verdict:'no_grade_trigger'},modules:[{module_id:'holder_concentration',evidence_status:'INFERRED',metrics:{owner_resolved_top_holder_pct:72}}]});
  assert.equal(vm.checklist.find(x=>x.id==='concentration').status,'yellow');
  assert.equal(vm.leverage.some(x=>x.id==='top-owner'),false);
});

test('historical fixture view-model snapshot',()=>{
  const fixture=JSON.parse(readFileSync(new URL('../__fixtures__/historical-scan.json',import.meta.url)));
  const vm=mapVerdictCard(fixture);
  assert.deepEqual({schema_version:vm.schema_version,header:vm.header,leverage:vm.leverage.map(x=>x.id),statuses:vm.checklist.map(x=>[x.id,x.status])},{
    schema_version:'koschei-verdict-card-v1',
    header:{state:'graded',tone:'orange',grade:'D',title:'D',copy:'triggered by rules [URD-C003]',ruleset_version:'unified-radar-v1.0',actor_ruleset_version:'actor-v1.0',signature_short:'abcdef12…567890',generated_at:'2026-07-15T10:00:00Z'},
    leverage:['freeze-authority','top-owner','repeat-actor'],
    statuses:[['launch','green'],['mint','green'],['freeze','red'],['wash','gray'],['address','gray'],['liquidity','green'],['funding','gray'],['concentration','red'],['sniper','gray'],['first-buyer','gray'],['track','red'],['creator-sell','red'],['dominant-exit','gray'],['liq-move','gray'],['program','gray'],['metadata','gray'],['claim','gray'],['mev','gray'],['distribution','gray'],['signed','green']]
  });
});
