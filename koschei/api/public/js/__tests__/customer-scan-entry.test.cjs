const {test}=require('node:test');
const assert=require('node:assert/strict');
const vm=require('node:vm');
const fs=require('node:fs');
const path=require('node:path');
const source=name=>fs.readFileSync(path.join(__dirname,'..',name),'utf8');
const evm='0x1111111111111111111111111111111111111111';
const sol='So11111111111111111111111111111111111111112';
const bitcoin='1BoatSLRHtKNngkdXEeobR76b53LETtpyT';
const scope=vm.createContext({window:{},URLSearchParams,location:{search:'',pathname:'/scan'}});
vm.runInContext(source('customer-scan-entry.js'),scope);
const router=scope.window.KoscheiScanEntry;

test('EVM addresses require an explicit network and do not fan out',()=>{
 assert.equal(router.resolve(evm).needsNetwork,true);
 for(const network of ['ethereum-mainnet','base-mainnet','arbitrum-mainnet','optimism-mainnet']){
  const url=new URL(router.url(evm,network).url,'https://example.test');
  assert.equal(url.searchParams.get('network'),network);
  assert.equal(url.searchParams.get('target'),evm);
  assert.equal(url.searchParams.get('mode'),'address');
 }
});
test('ambiguous base58 requires chain context; Solana and Bitcoin are never silently interchanged',()=>{
 assert.equal(router.resolve(bitcoin).needsNetwork,true);
 assert.equal(router.resolve(bitcoin,'bitcoin-mainnet').family,'bitcoin');
 assert.equal(router.resolve('11111111111111111111111111111111','solana-mainnet').family,'solana');
 assert.equal(router.resolve(sol).network,'solana-mainnet');
 assert.ok(router.resolve(evm,'solana-mainnet').error);
 assert.ok(router.resolve(sol,'base-mainnet').error);
 assert.ok(router.resolve(evm,'unsupported-chain').error);
});
test('invalid addresses, transaction hashes and secret-shaped input are not submitted as addresses',()=>{
 for(const value of ['', '0x123', 'https://example.test', '0x'+'a'.repeat(64), 'word '.repeat(24), '<img src=x onerror=alert(1)>'])assert.ok(router.resolve(value,'ethereum-mainnet').error);
});
test('legacy token, deep and preflight links keep their selected tool',()=>{
 assert.equal(router.isAddressView('?target='+evm,'/scan'),true);
 assert.equal(router.isAddressView('?mode=address&target='+evm,'/scan'),true);
 for(const mode of ['token','deep','transaction'])assert.equal(router.isAddressView('?mode='+mode,'/scan'),false);
 assert.equal(router.isAddressView('','/scan/'+sol),false);
 assert.equal(router.isAddressView('?mint='+sol,'/scan'),false);
});
function envelope(target=evm,network='base-mainnet'){
 return {schema_version:'koschei-customer-scan-v1',result:{target:{raw:target,network_hint:network},status:'observed',verdict:'review',evidence_status:'observed',trust:{claimed:false,observed:true,authorized:false,available:false,verified:false,finalized:false},reasons:['TEST_FIXTURE'],evidence_refs:['fixture:test']}};
}
test('result identity binds schema, exact address and chain',()=>{
 const request=router.resolve(evm,'base-mainnet');
 assert.equal(router.matchesResult(envelope(),request),true);
 assert.equal(router.matchesResult(envelope(sol),request),false);
 assert.equal(router.matchesResult(envelope(evm,'ethereum-mainnet'),request),false);
 assert.equal(router.matchesResult({...envelope(),schema_version:'unknown'},request),false);
 assert.equal(router.matchesResult({result:null},request),false);
});
test('result maturity binds status, verdict, evidence status and trust vector',()=>{
 const request=router.resolve(evm,'base-mainnet');
 const mutate=fn=>{const value=structuredClone(envelope());fn(value.result);return value;};
 assert.equal(router.matchesResult(mutate(r=>{r.evidence_status='unverified';}),request),false);
 assert.equal(router.matchesResult(mutate(r=>{r.trust.observed=false;}),request),false);
 assert.equal(router.matchesResult(mutate(r=>{r.trust.verified=true;r.trust.observed=false;r.evidence_status='verified';r.status='evidence_ready';}),request),false);
 assert.equal(router.matchesResult(mutate(r=>{r.trust.finalized=true;r.trust.verified=false;r.evidence_status='finalized';r.status='evidence_ready';}),request),false);
 assert.equal(router.matchesResult(mutate(r=>{r.verdict='unknown';}),request),false);
 assert.equal(router.matchesResult(mutate(r=>{r.evidence_refs=[];}),request),false);
 const verified=mutate(r=>{r.status='evidence_ready';r.evidence_status='verified';r.trust.verified=true;});
 assert.equal(router.matchesResult(verified,request),true);
 const limited=mutate(r=>{r.status='insufficient_evidence';r.verdict='unknown';r.evidence_status='unverified';r.trust.observed=false;r.evidence_refs=undefined;});
 assert.equal(router.matchesResult(limited,request),true);
});

// Minimal DOM hosts exercise the real controller with asynchronous transports.
// These are explicit test fixtures, never included in the product bundle.
class Node {
 constructor(){this.value='';this.textContent='';this.hidden=true;this.disabled=false;this.innerHTML='';this.attrs={};this.events={};this.children=[];}
 addEventListener(name,fn){(this.events[name]??=[]).push(fn);}
 dispatch(name){for(const fn of this.events[name]||[])fn({preventDefault(){}});}
 setAttribute(name,value){this.attrs[name]=value;}
 removeAttribute(name){delete this.attrs[name];}
 focus(){this.focused=true;}
 replaceChildren(...nodes){this.children=nodes;}
}
function harness(fetch,search=''){
 const ids=['customerUniversalScan','customerUniversalScanForm','customerUniversalTarget','customerUniversalNetwork','customerUniversalSubmit','customerUniversalCancel','customerUniversalStatus','customerUniversalResultsWrap','customerUniversalOverview','customerUniversalResults','customerUniversalRecovery','advancedTools'];
 const nodes=Object.fromEntries(ids.map(id=>[id,new Node()]));
 const timers=new Map();let timerID=0;
 const window={addEventListener(){}};
 const context=vm.createContext({window,URLSearchParams,AbortController,SyntaxError,Error,location:{search,pathname:'/scan'},history:{replaceState(){}},setTimeout(fn){timers.set(++timerID,fn);return timerID;},clearTimeout(id){timers.delete(id);},fetch,document:{readyState:'complete',getElementById:id=>nodes[id],querySelectorAll:()=>[],createElement:()=>new Node()}});
 vm.runInContext(source('customer-scan-entry.js'),context);
 vm.runInContext(source('customer-universal-address-scan-v1.js'),context);
 const submit=(target=evm,network='base-mainnet')=>{nodes.customerUniversalTarget.value=target;nodes.customerUniversalNetwork.value=network;nodes.customerUniversalScanForm.dispatch('submit');};
 return {nodes,submit,timers};
}
const settle=()=>new Promise(resolve=>setImmediate(resolve));
const response=(status,data)=>({status,ok:status>=200&&status<300,json:async()=>data});
test('one submit sends one chain-bound request and displays observed evidence honestly',async()=>{
 const calls=[];const h=harness(async(url,options)=>{calls.push({url,options});return response(200,envelope());});
 h.submit();await settle();
 assert.equal(calls.length,1);assert.equal(calls[0].url,'/api/scan');
 assert.deepEqual(JSON.parse(calls[0].options.body),{target:evm,network:'base-mainnet'});
 assert.equal(h.nodes.customerUniversalResultsWrap.hidden,false);
 assert.match(h.nodes.customerUniversalResults.innerHTML,/OBSERVED/);
 assert.match(h.nodes.customerUniversalResults.innerHTML,/fixture:test/);
 assert.doesNotMatch(h.nodes.customerUniversalOverview.innerHTML,/SAFE/);
 assert.equal(h.nodes.customerUniversalSubmit.disabled,false);
});
test('ambiguous EVM deep link waits for network without starting a request',async()=>{
 let calls=0;const h=harness(async()=>{calls++;return response(200,envelope());},'?target='+evm);
 await settle();assert.equal(calls,0);assert.match(h.nodes.customerUniversalStatus.textContent,/Select the network/);
});
test('explicit preflight deep link exposes its form and never starts an address scan',async()=>{
 let calls=0;const h=harness(async()=>{calls++;},'?mode=transaction&target='+evm);
 await settle();assert.equal(calls,0);assert.equal(h.nodes.advancedTools.open,true);
});
test('a stale response cannot overwrite an edited target',async()=>{
 let complete;const h=harness(()=>new Promise(resolve=>{complete=resolve;}));h.submit();
 h.nodes.customerUniversalTarget.value=sol;h.nodes.customerUniversalTarget.dispatch('input');
 complete(response(200,envelope()));await settle();
 assert.equal(h.nodes.customerUniversalResultsWrap.hidden,true);
 assert.equal(h.nodes.customerUniversalSubmit.disabled,false);
});
test('mismatched response is withheld rather than rendered',async()=>{
 const h=harness(async()=>response(200,envelope(sol)));h.submit();await settle();
 assert.equal(h.nodes.customerUniversalResultsWrap.hidden,true);
 assert.match(h.nodes.customerUniversalStatus.textContent,/does not match/);
});
test('impossible trust maturity is withheld rather than rendered',async()=>{
 const bad=envelope();bad.result.trust.observed=false;bad.result.trust.verified=true;bad.result.evidence_status='verified';bad.result.status='evidence_ready';
 const h=harness(async()=>response(200,bad));h.submit();await settle();
 assert.equal(h.nodes.customerUniversalResultsWrap.hidden,true);
 assert.match(h.nodes.customerUniversalStatus.textContent,/does not match/);
});
test('cancel restores controls and prevents a late result',async()=>{
 let complete;const h=harness(()=>new Promise(resolve=>{complete=resolve;}));h.submit();h.nodes.customerUniversalCancel.dispatch('click');
 complete(response(200,envelope()));await settle();assert.equal(h.nodes.customerUniversalSubmit.disabled,false);
 assert.equal(h.nodes.customerUniversalResultsWrap.hidden,true);assert.match(h.nodes.customerUniversalStatus.textContent,/canceled/);
});
for(const code of [401,402,403,429,503])test(`HTTP ${code} offers honest recovery without manufacturing a result`,async()=>{
 const h=harness(async()=>response(code,{error:'fixture_error'}));h.submit();await settle();
 assert.match(h.nodes.customerUniversalResults.innerHTML,/Evidence unavailable/);
 assert.match(h.nodes.customerUniversalOverview.innerHTML,/LIMITED EVIDENCE/);
 assert.equal(h.nodes.customerUniversalSubmit.disabled,false);
 if(code===401)assert.match(h.nodes.customerUniversalRecovery.children[0].href,/^\/login\?next=/);
 if([402,403].includes(code))assert.equal(h.nodes.customerUniversalRecovery.children[0].href,'/account');
});
test('network timeout restores retry controls',async()=>{
 const h=harness((url,options)=>new Promise((resolve,reject)=>options.signal.addEventListener('abort',()=>reject(new Error('aborted')))));
 h.submit();for(const fn of h.timers.values())fn();await settle();
 assert.equal(h.nodes.customerUniversalSubmit.disabled,false);assert.match(h.nodes.customerUniversalStatus.textContent,/15 seconds/);
});
test('malformed JSON is an error, never an empty success',async()=>{
 const h=harness(async()=>({ok:true,status:200,json:async()=>{throw new SyntaxError('invalid JSON');}}));h.submit();await settle();
 assert.match(h.nodes.customerUniversalStatus.textContent,/unreadable/);assert.equal(h.nodes.customerUniversalResultsWrap.hidden,true);
});

test('legacy Solana bootstrap does not issue a second request for the universal address route',()=>{
 for(const search of ['?mode=address&network=base-mainnet&target='+evm,'?target='+sol]){
  let calls=0;const nodes=new Map();
  const document={getElementById(id){if(!nodes.has(id))nodes.set(id,new Node());return nodes.get(id);},querySelector:()=>({}),querySelectorAll:()=>[]};
  const context=vm.createContext({window:{},document,URLSearchParams,location:{pathname:'/scan',search},history:{replaceState(){}},setTimeout,clearTimeout,AbortController,fetch:async()=>{calls++;return response(200,{});}});
  vm.runInContext(source('public-solana-scan.js'),context);
  assert.equal(calls,0,'address entry must not run the Solana token endpoint, even if the shared helper failed to load');
 }
});

test('shared transport preserves full token collector time while keeping ordinary APIs bounded',async()=>{
 const delays=[];
 const document={readyState:'loading',addEventListener(){},querySelector:()=>({})};
 const window={location:{origin:'https://example.test'},fetch:async()=>response(200,{}),setTimeout(fn,delay){delays.push(delay);return delays.length;},clearTimeout(){}};
 const context=vm.createContext({window,document,URL,AbortController});
 vm.runInContext(source('koschei-global-shell.js'),context);
 await window.fetch('/api/token/scan');assert.equal(delays.at(-1),210000);
 await window.fetch('/api/scan');assert.equal(delays.at(-1),15000);
 await window.fetch('/health');assert.equal(delays.at(-1),10000);
});
