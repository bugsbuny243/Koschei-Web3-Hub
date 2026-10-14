const {test}=require('node:test');
const assert=require('node:assert/strict');
const vm=require('node:vm');
const fs=require('node:fs');
const path=require('node:path');
const source=fs.readFileSync(path.join(__dirname,'..','owner-unified-radar.js'),'utf8');
class Element {
 constructor(){this.events={};this.children=[];this.innerHTML='';this.textContent='';this.disabled=false;}
 addEventListener(name,fn){this.events[name]=fn;}
 appendChild(child){this.children.push(child);}
 querySelector(key){return this.nodes[key];}
}
function harness(responses){
 const nodes=Object.fromEntries(['page','section','load','controls','result','form','readiness','button','network','kind','target'].map(k=>[k,new Element()]));
 nodes.section.nodes={'[data-load]':nodes.load,'[data-controls]':nodes.controls,'[data-result]':nodes.result};
 nodes.controls.nodes={'form':nodes.form,'[data-readiness]':nodes.readiness};
 nodes.form.nodes={'button':nodes.button};nodes.form.elements={network:nodes.network,kind:nodes.kind,target:nodes.target};
 nodes.network.value='ethereum-mainnet';nodes.kind.value='wallet';nodes.target.value='0x1111111111111111111111111111111111111111';
 const calls=[];
 vm.runInNewContext(source,{window:{OwnerRadarKit:{}},document:{readyState:'complete',getElementById:id=>id==='page-arvis'?nodes.page:null,createElement:()=>nodes.section},Intl,AbortController,setTimeout,clearTimeout,
 fetch:async(url,options)=>{calls.push({url,options});return responses.shift();}});
 return {nodes,calls};
}
const capability={ok:true,json:async()=>({capabilities:[{network:'ethereum-mainnet',target_classes:['wallet'],sources:[],archival_requirements:[]}],runtime:[{capability:{network:'ethereum-mainnet'},runtime_status:'provider_configuration_required'}]})};
test('owner radar does not fetch until explicitly loaded and reports missing configuration',async()=>{
 const h=harness([capability]);assert.equal(h.calls.length,0);
 await h.nodes.load.events.click();assert.equal(h.calls[0].options.credentials,'same-origin');
 assert.match(h.nodes.readiness.textContent,/provider_configuration_required/);
 assert.match(h.nodes.readiness.textContent,/henüz doğrulanmadı/);
});
test('partial evidence keeps WITHHOLD and escapes provider controlled text',async()=>{
 const h=harness([capability,{ok:true,json:async()=>({collector_status:'partial_investigation',verdict:'withhold',limitations:[{code:'unknown',reason:'<img src=x onerror=alert(1)>'}],evidence:[],provider_request_count:3})}]);
 await h.nodes.load.events.click();await h.nodes.form.events.submit({preventDefault(){}});
 assert.match(h.nodes.result.innerHTML,/withhold/);assert.match(h.nodes.result.innerHTML,/&lt;img/);assert.doesNotMatch(h.nodes.result.innerHTML,/<img/);
 assert.equal(h.nodes.button.disabled,false);
 assert.deepEqual(JSON.parse(h.calls[1].options.body),{network:'ethereum-mainnet',kind:'wallet',target:h.nodes.target.value});
});
test('authentication failure does not render a scan form',async()=>{
 const h=harness([{ok:false}]);await h.nodes.load.events.click();assert.equal(h.nodes.controls.innerHTML,'');assert.match(h.nodes.result.textContent,/Owner oturumu/);
});
test('control history and archive gaps remain visible without upgrading the verdict',async()=>{
 const h=harness([capability,{ok:true,json:async()=>({collector_status:'partial_investigation',verdict:'withhold',limitations:[{code:'control_history_slot_unknown',reason:'admin historical state unavailable'}],evidence:[{status:'observed',block_or_slot:'0x300',source:'rpc.fixture.invalid',attributes:{relation:'evm-control-history-implementation',from_block:'0x100',changed_between_snapshots:true,previous_address:'fixture-before',current_address:'fixture-after',intermediate_changes:'unknown'}}],provider_request_count:8})}]);
 await h.nodes.load.events.click();await h.nodes.form.events.submit({preventDefault(){}});
 for(const text of ['evm-control-history-implementation','fixture-before','fixture-after','0x100','0x300','control_history_slot_unknown','withhold'])assert.ok(h.nodes.result.innerHTML.includes(text),text);
 assert.doesNotMatch(h.nodes.result.innerHTML,/VERIFIED|SAFE/);
});
