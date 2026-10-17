'use strict';
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');

const script=fs.readFileSync(path.join(__dirname,'../public/js/owner-verdict-card-coverage.js'),'utf8');

function mount(payload,checklistText='7 complete · 7 partial · 0 N/A'){
  const host={children:[],html:[],appendChild(node){this.children.push(node);},insertAdjacentHTML(_position,html){this.html.push(html);},querySelector(selector){
    const name=selector.replace('.','');
    return this.children.find(x=>x.className===name&&!x.removed)||null;
  }};
  const card={
    querySelector(selector){return selector==='.vc-header>div:last-child'?host:null;},
    querySelectorAll(){return[];},
  };
  const root={
    dataset:{},
    addEventListener(){},
    contains(){return false;},
    querySelector(selector){return selector==='#verdict-card'?card:null;},
  };
  const context={
    window:{
      OwnerRadarKit:{renderUnified(){},scan:async()=>payload},
      KoscheiVerdictCard:{mapVerdictCard(){return{coverage:{text:checklistText},checklist:[]}}},
    },
    document:{
      getElementById(){return null;},
      createElement(){return {id:'',className:'',textContent:'',removed:false,setAttribute(){},remove(){this.removed=true}}},
      head:{appendChild(){}},
    },
    navigator:{clipboard:{writeText:async()=>{}}},
    setTimeout(){},
    CSS:{escape:text=>text},
  };
  vm.runInNewContext(script,context,{filename:'owner-verdict-card-coverage.js'});
  context.window.OwnerRadarKit.renderUnified(root,payload);
  const runtime=host.children.find(x=>x.className==='vc-arm-runtime');
  assert.ok(runtime,'runtime status is mounted separately from checklist');
  return {runtime:runtime.textContent,checklist:host.html.join(''),host,root,kit:context.window.OwnerRadarKit};
}

const coverage={capability_total:14,completed:7,evidence_pending:6,not_applicable:1,
  source_unavailable:0,insufficient_evidence:0,critical_gaps:['holder_funding_relations','launch_and_first_buyer_history'],
  status:'partial_investigation'};
let result=mount({investigation_report:{investigation_coverage:coverage}});
assert.match(result.runtime,/7\/14 tamamlandı/);
assert.match(result.runtime,/6 kanıt bekliyor/);
assert.match(result.runtime,/1 uygulanamaz/);
assert.match(result.runtime,/0 kaynak hatası/);
assert.match(result.runtime,/2 kritik boşluk/);
assert.match(result.runtime,/İmza ve grade/);
assert.match(result.checklist,/Kontrol listesi \(14 ARVIS kolundan ayrı\):/);
assert.match(result.checklist,/7 complete · 7 partial/);
assert.doesNotMatch(result.runtime,/7 partial/,'checklist count may not become ARVIS status');

result=mount({legacy_14_arm_radar:{investigation_coverage:{
  capability_total:14,completed:13,evidence_pending:0,not_applicable:1,
  source_unavailable:0,insufficient_evidence:0,critical_gaps:[],status:'complete_investigation',
}}});
assert.match(result.runtime,/13\/14 tamamlandı/);
assert.match(result.runtime,/durum: tam/);
assert.match(result.runtime,/1 uygulanamaz/);

result=mount({investigation_report:{modules:Array.from({length:14},()=>({module_id:'test'}))}});
assert.match(result.runtime,/kanonik sayaç yok/,'14 architecture modules alone cannot claim completion');

result=mount({investigation_report:{investigation_coverage:{
  ...coverage,evidence_pending:3,
}}});
assert.match(result.runtime,/tutarsız veya eksik/);
assert.doesNotMatch(result.runtime,/7\/14 tamamlandı/);

result=mount({investigation_report:{investigation_coverage:{
  ...coverage,completed:6,evidence_pending:6,source_unavailable:1,
}}});
assert.match(result.runtime,/1 kaynak hatası/);
assert.match(result.runtime,/6\/14 tamamlandı/);

result=mount({investigation_report:{investigation_coverage:coverage}},'<script>alert("x")</script>');
assert.ok(!result.checklist.includes('<script>'),'untrusted checklist status must be escaped');
assert.match(result.checklist,/&lt;script&gt;/);

console.log('Owner ARVIS coverage parity regression passed: collector execution, checklist separation, missing/inconsistent/N-A/unavailable/escaping.');
