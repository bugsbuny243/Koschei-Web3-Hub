'use strict';
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');

const addon=fs.readFileSync(path.join(__dirname,'../public/js/owner-source-evidence-health.js'),'utf8');
const html=fs.readFileSync(path.join(__dirname,'../public/owner-production.html'),'utf8');
assert.match(html,/owner-source-evidence-health\.js\?v=1/);

class Element {
  constructor(tag){this.tag=tag;this.className='';this.textContent='';this.children=[];this.removed=false;}
  appendChild(item){this.children.push(item);return item;}
  remove(){this.removed=true;}
  querySelector(selector){
    return this.children.find(x=>!x.removed&&selector==='.'+x.className)||null;
  }
}
function allText(node){return[node.textContent,...node.children.flatMap(allText)];}
function evaluate(report){
  const area=new Element('section');
  const root={querySelector(q){return q==='.investigation-coverage-card'?area:null;}};
  let nativeRenders=0;
  const env={
    window:{OwnerRadarKit:{renderUnified(){nativeRenders++;},scan:async()=>({investigation_report:report})}},
    document:{getElementById(){return null;},createElement:tag=>new Element(tag)},
  };
  vm.runInNewContext(addon,env,{filename:'owner-source-evidence-health.js'});
  env.window.OwnerRadarKit.renderUnified(root,{investigation_report:report});
  const source=()=>area.children.filter(x=>!x.removed&&x.className==='owner-source-evidence-health');
  assert.equal(nativeRenders,1,'native radar renderer was not called');
  return{root,env,source,content:()=>source().flatMap(allText).join(' ')};
}
const arms=[
  ...['token_authority_scanner','holder_concentration','funding_cluster_detector','program_relation_scan'].map(module_id=>({
    module_id,execution_status:'completed',evidence_status:'partial_rpc_evidence_deadline',
    subchecks:{required_checks_status:'unknown',required_checks_passed:false},
  })),
  ...Array.from({length:7},(_,i)=>({module_id:'completed_'+i,execution_status:'completed',
    evidence_status:'verified_launch_timing',subchecks:{required_checks_status:'unknown',required_checks_passed:false}})),
  ...Array.from({length:2},(_,i)=>({module_id:'pending_'+i,execution_status:'evidence_pending'})),
  {module_id:'mev_shield',execution_status:'not_applicable'},
];
const ansemShape={
  investigation_coverage:{capability_total:14,completed:11,evidence_pending:2,not_applicable:1,
    source_unavailable:0,insufficient_evidence:0,arms},
  full_scan_live_evidence:{status:'partial_timeout',wallets_requested:4,wallets_completed:1,rpc_failures:2},
  investigation_transparency:{collection_gaps:[{reason:'Helius creator history could not be collected: helius RPC error -32015: Transaction version (1) is not supported by the requesting client.'}]},
  holder_cluster:{confidence:'medium'},
  final_verdict:{signed:true,grade:'D'},
};
let out=evaluate(ansemShape);
let s=out.content();
for(const claim of ['11/14 tamamlandı','2 bekliyor','1 uygulanamaz','0 kol kaynağı alınamadı',
  '4 — token_authority_scanner','1/4 cüzdan','2 RPC hatası','partial_timeout',
  '11 tamamlanmış kol','Holder cluster confidence','medium','grade confidence değildir',
  'Desteklenmeyen işlem sürümü','İmzalı verdict','D — imza metadata bilgisi']){
  assert.ok(s.includes(claim),'missing bounded diagnostic '+claim);
}
assert.ok(s.includes('source_unavailable=0'),'arm-level source availability was confused with RPC errors');
out.env.window.OwnerRadarKit.renderUnified(out.root,{investigation_report:ansemShape});
assert.equal(out.source().length,1,'stale/duplicate health reports after render');

const pyusdShape={
  investigation_coverage:{capability_total:14,completed:7,evidence_pending:6,not_applicable:1,
    source_unavailable:0,arms:[
      ...Array.from({length:7},(_,i)=>({module_id:'complete_'+i,execution_status:'completed',evidence_status:'verified'})),
      ...Array.from({length:6},(_,i)=>({module_id:'pending_'+i,execution_status:'evidence_pending'})),
      {module_id:'mev_shield',execution_status:'not_applicable'},
    ]},
  full_scan_live_evidence:{status:'complete',wallets_requested:4,wallets_completed:4,rpc_failures:0},
  holder_cluster:{confidence:'none'},final_verdict:{grade:'D',signed:true},
};
out=evaluate(pyusdShape);s=out.content();
for(const claim of ['7/14 tamamlandı','4/4 cüzdan','0 RPC hatası','Tamamlanan ama kısmi kaynak kanıtı taşıyan kollar','none — yalnız holder_cluster']){
  assert.ok(s.includes(claim),'PYUSD source diagnostic missing '+claim);
}
assert.ok(!s.includes('Desteklenmeyen işlem sürümü'),'unsupported v1 was invented for PYUSD');

out=evaluate({});
assert.equal(out.source().length,0,'missing evidence may not generate a healthy verdict');

out=evaluate({investigation_coverage:{arms:[{
  module_id:'<img src=x onerror=alert(1)>',execution_status:'completed',
  evidence_status:'partial_rpc_evidence_deadline',
}]},full_scan_live_evidence:{status:'partial_timeout',rpc_failures:1}});
s=out.content();
assert.ok(!s.includes('<img'),'malicious module ID was not normalized');
console.log('Owner source health accepted: 11/14 vs 1/4; RPC error scope; subcheck unknown; module-scoped confidence; missing data; escaping.');
