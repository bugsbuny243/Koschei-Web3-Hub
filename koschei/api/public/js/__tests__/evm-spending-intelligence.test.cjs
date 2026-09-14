const {test}=require('node:test');
const assert=require('node:assert/strict');
const vm=require('node:vm');
const fs=require('node:fs');
const path=require('node:path');

const source=fs.readFileSync(path.join(__dirname,'..','evm-spending-intelligence.js'),'utf8');
const token='0x1111111111111111111111111111111111111111';
const owner='0x2222222222222222222222222222222222222222';
const spender='0x3333333333333333333333333333333333333333';

class Node {
  constructor(){this.value='';this.textContent='';this.hidden=true;this.disabled=false;this.innerHTML='';this.dataset={};this.events={};}
  addEventListener(name,fn){(this.events[name]??=[]).push(fn);}
  dispatch(name){for(const fn of this.events[name]||[])fn({preventDefault(){}});}
}

function envelope(overrides={}){
  return {schema_version:'koschei-approval-scan-v1',result:{
    network:'ethereum-mainnet',token,owner,spender,amount:'0',unlimited:false,
    evidence_status:'observed',live_availability:'checked',
    trust:{claimed:false,observed:true,authorized:false,available:false,verified:false,finalized:false},
    reasons:['CURRENT_ALLOWANCE_OBSERVED','APPROVAL_PROVENANCE_NOT_INFERRED'],
    ...overrides
  }};
}

function harness(fetchImpl){
  const ids=['evmSpendingForm','evmSpendingNetwork','evmSpendingToken','evmSpendingOwner','evmSpendingSpender','evmSpendingSubmit','evmSpendingStatus','evmSpendingResult'];
  const nodes=Object.fromEntries(ids.map(id=>[id,new Node()]));
  nodes.evmSpendingNetwork.value='ethereum-mainnet';
  nodes.evmSpendingToken.value=token;
  nodes.evmSpendingOwner.value=owner;
  nodes.evmSpendingSpender.value=spender;
  const timers=new Map();let timerID=0;
  const context=vm.createContext({
    document:{getElementById:id=>nodes[id]},fetch:fetchImpl,AbortController,Error,String,Boolean,JSON,
    setTimeout(fn,delay){timers.set(++timerID,{fn,delay});return timerID;},
    clearTimeout(id){timers.delete(id);}
  });
  vm.runInContext(source,context);
  return {nodes,timers,submit(){nodes.evmSpendingForm.dispatch('submit');}};
}

const response=(status,data)=>({status,ok:status>=200&&status<300,json:async()=>data});
const settle=()=>new Promise(resolve=>setImmediate(resolve));

test('observed zero allowance renders only when tuple and evidence binding are complete',async()=>{
  const calls=[];
  const h=harness(async(url,options)=>{calls.push({url,options});return response(200,envelope());});
  h.submit();await settle();
  assert.equal(calls.length,1);
  assert.equal(calls[0].url,'/api/scan/approval');
  assert.deepEqual(JSON.parse(calls[0].options.body),{network:'ethereum-mainnet',token,owner,spender});
  assert.equal(calls[0].options.cache,'no-store');
  assert.equal(calls[0].options.credentials,'same-origin');
  assert.equal(h.nodes.evmSpendingResult.hidden,false);
  assert.match(h.nodes.evmSpendingResult.innerHTML,/Current allowance/);
  assert.match(h.nodes.evmSpendingResult.innerHTML,/<strong>0<\/strong>/);
  assert.match(h.nodes.evmSpendingStatus.textContent,/Current allowance observed/);
  assert.equal(h.nodes.evmSpendingSubmit.disabled,false);
});

test('malformed HTTP 200 is withheld instead of becoming zero allowance',async()=>{
  const h=harness(async()=>response(200,{}));
  h.submit();await settle();
  assert.equal(h.nodes.evmSpendingResult.hidden,true);
  assert.match(h.nodes.evmSpendingStatus.textContent,/approval evidence schema mismatch/);
  assert.equal(h.nodes.evmSpendingSubmit.disabled,false);
});

test('stale or foreign approval schema is withheld even when result shape looks valid',async()=>{
  const stale=envelope();stale.schema_version='koschei-approval-scan-v0';
  const h=harness(async()=>response(200,stale));
  h.submit();await settle();
  assert.equal(h.nodes.evmSpendingResult.hidden,true);
  assert.match(h.nodes.evmSpendingStatus.textContent,/approval evidence schema mismatch/);
});

test('wrong chain or subject tuple is withheld',async()=>{
  for(const bad of [
    {network:'base-mainnet'},
    {token:'0x4444444444444444444444444444444444444444'},
    {owner:'0x5555555555555555555555555555555555555555'},
    {spender:'0x6666666666666666666666666666666666666666'}
  ]){
    const h=harness(async()=>response(200,envelope(bad)));
    h.submit();await settle();
    assert.equal(h.nodes.evmSpendingResult.hidden,true);
    assert.match(h.nodes.evmSpendingStatus.textContent,/mismatch/);
  }
});

test('incomplete or over-promoted evidence is withheld',async()=>{
  for(const bad of [
    {evidence_status:'unverified'},
    {live_availability:'partial'},
    {amount:undefined},
    {trust:{observed:false,verified:false,authorized:false,finalized:false}},
    {trust:{observed:true,verified:true,authorized:false,finalized:false}},
    {trust:{observed:true,verified:false,authorized:true,finalized:false}},
    {trust:{observed:true,verified:false,authorized:false,finalized:true}}
  ]){
    const h=harness(async()=>response(200,envelope(bad)));
    h.submit();await settle();
    assert.equal(h.nodes.evmSpendingResult.hidden,true);
    assert.equal(h.nodes.evmSpendingStatus.dataset.state,'error');
  }
});

test('provider error never renders stale allowance',async()=>{
  const h=harness(async()=>response(502,{error:'evm_allowance_probe_unavailable'}));
  h.nodes.evmSpendingResult.hidden=false;
  h.nodes.evmSpendingResult.innerHTML='<b>stale</b>';
  h.submit();await settle();
  assert.equal(h.nodes.evmSpendingResult.hidden,true);
  assert.match(h.nodes.evmSpendingStatus.textContent,/evm_allowance_probe_unavailable/);
  assert.equal(h.nodes.evmSpendingSubmit.disabled,false);
});

test('approval transport aborts at 15 seconds and restores retry controls',async()=>{
  const h=harness((url,options)=>new Promise((resolve,reject)=>options.signal.addEventListener('abort',()=>reject(new Error('aborted')))));
  h.submit();
  const timer=[...h.timers.values()][0];
  assert.equal(timer.delay,15000);
  timer.fn();await settle();
  assert.equal(h.nodes.evmSpendingResult.hidden,true);
  assert.match(h.nodes.evmSpendingStatus.textContent,/timed out after 15 seconds/);
  assert.equal(h.nodes.evmSpendingSubmit.disabled,false);
  assert.equal(h.nodes.evmSpendingSubmit.textContent,'Read spending authority');
});
