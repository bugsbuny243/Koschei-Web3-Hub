const {test} = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');
const source = fs.readFileSync(path.join(__dirname,'..','owner-global-campaigns.js'),'utf8');
class Element {
  constructor(){this.children=[];this.textContent='';this.disabled=false;this.events={};}
  append(...children){this.children.push(...children);}
  replaceChildren(...children){this.children=children;}
  addEventListener(event,fn){this.events[event]=fn;}
  set innerHTML(_){throw new Error('Untrusted campaign data entered HTML');}
}
const settle = () => new Promise(resolve=>setImmediate(resolve));
function harness(response){
  const nodes=Object.fromEntries(['campaign-status','campaign-runtime','campaign-list','campaign-refresh'].map(id=>[id,new Element()]));
  const calls=[];
  vm.runInNewContext(source,{document:{getElementById:id=>nodes[id],createElement:()=>new Element()},AbortSignal,
    fetch:async(url,options)=>{calls.push({url,options});return response;}});
  return {nodes,calls};
}
function fixture(campaigns=[]){
  return {version:'koschei.global-campaign-runtime.v1',runtime:{enabled:true,pending:0,failed:0,processed:campaigns.length},campaigns};
}
test('owner denial exposes no stale campaign or queue data',async()=>{
  const h=harness({ok:false,status:404});await settle();
  assert.match(h.nodes['campaign-status'].textContent,/Sign in/);
  assert.equal(h.nodes['campaign-list'].children.length,0);
  assert.equal(h.nodes['campaign-runtime'].children.length,0);
  assert.equal(h.calls[0].options.credentials,'same-origin');
  assert.equal(h.calls[0].options.cache,'no-store');
});
test('unavailable storage remains unknown',async()=>{
  const h=harness({ok:false,status:503});await settle();
  assert.match(h.nodes['campaign-status'].textContent,/unavailable.*unknown/);
  assert.equal(h.nodes['campaign-list'].children.length,0);
});
test('empty campaigns do not invent linked evidence',async()=>{
  const h=harness({ok:true,json:async()=>fixture()});await settle();
  assert.match(h.nodes['campaign-status'].textContent,/No verified linked evidence/);
  assert.equal(h.nodes['campaign-list'].children.length,0);
});
test('campaign source strings stay text and containment stays unverified',async()=>{
  const ref='<img src=x onerror=alert(1)>';
  const snapshot={campaign:{campaign_ref:ref,revision:1,state:'emerging',networks:['fixture-network']},command:{response_state:'monitoring',containment_verified:false,missing_evidence:['threat_family_inputs_not_connected']}};
  const h=harness({ok:true,json:async()=>fixture([snapshot])});await settle();
  const article=h.nodes['campaign-list'].children[0];
  assert.equal(article.children[0].textContent,ref);
  assert.match(JSON.stringify(article),/Unverified/);
  assert.match(JSON.stringify(article),/threat_family_inputs_not_connected/);
});
test('unexpected execution authority hides the entire projection',async()=>{
  const snapshot={campaign:{campaign_ref:'fixture'},command:{response_state:'executed',containment_verified:true}};
  const h=harness({ok:true,json:async()=>fixture([snapshot])});await settle();
  assert.equal(h.nodes['campaign-list'].children.length,0);
  assert.equal(h.nodes['campaign-runtime'].children.length,0);
  assert.match(h.nodes['campaign-status'].textContent,/Unexpected response authority/);
});
