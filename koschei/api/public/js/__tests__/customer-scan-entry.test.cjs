const {test}=require('node:test');
const assert=require('node:assert/strict');
const vm=require('node:vm');
const fs=require('node:fs');
const path=require('node:path');
const source=name=>fs.readFileSync(path.join(__dirname,'..',name),'utf8');
const evm='0x1111111111111111111111111111111111111111';
const evmTx='0x'+'a'.repeat(64);
const sol='So11111111111111111111111111111111111111112';
const bitcoin='1BoatSLRHtKNngkdXEeobR76b53LETtpyT';
const scope=vm.createContext({window:{},URLSearchParams,location:{search:'',pathname:'/scan'}});
vm.runInContext(source('customer-scan-entry.js'),scope);
const router=scope.window.KoscheiScanEntry;

test('EVM addresses require an explicit network and do not fan out',()=>{
 assert.equal(router.resolve(evm).needsNetwork,true);
 for(const network of ['ethereum-mainnet','base-mainnet','arbitrum-mainnet','optimism-mainnet','polygon-mainnet','bnb-mainnet','avalanche-mainnet']){
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
test('invalid addresses and secret-shaped input are not submitted as targets',()=>{
 for(const value of ['', '0x123', 'https://example.test', 'word '.repeat(24), '<img src=x onerror=alert(1)>'])assert.ok(router.resolve(value,'ethereum-mainnet').error);
});
test('EVM transaction hashes require explicit EVM network context and use lookup mode',()=>{
 assert.equal(router.resolve(evmTx).needsNetwork,true);
 const request=router.resolve(evmTx,'base-mainnet');
 assert.equal(request.kind,'transaction');assert.equal(request.family,'evm');
 assert.ok(router.resolve(evmTx,'solana-mainnet').error);
 const targetURL=new URL(router.url(evmTx,'base-mainnet').url,'https://example.test');
 assert.equal(targetURL.searchParams.get('mode'),'lookup');
 assert.equal(targetURL.searchParams.get('target'),evmTx);
 assert.equal(targetURL.searchParams.get('network'),'base-mainnet');
});
test('legacy token, deep and preflight links keep their selected tool',()=>{
 assert.equal(router.isAddressView('?target='+evm,'/scan'),true);
 assert.equal(router.isAddressView('?mode=address&target='+evm,'/scan'),true);
 assert.equal(router.isAddressView('?mode=lookup&target='+evmTx,'/scan'),true);
 for(const mode of ['token','deep','transaction'])assert.equal(router.isAddressView('?mode='+mode,'/scan'),false);
 assert.equal(router.isAddressView('','/scan/'+sol),false);
 assert.equal(router.isAddressView('?mint='+sol,'/scan'),false);
});
function envelope(target=evm,network='base-mainnet'){
 return {schema_version:'koschei-customer-scan-v1',result:{target:{raw:target,network_hint:network},status:'observed',evidence_status:'unverified',trust:{observed:true,verified:false,finalized:false},reasons:['TEST_FIXTURE'],evidence_refs:['fixture:test']}};
}
function transactionEnvelope(network='base-mainnet'){
 return {schema_version:'koschei-customer-scan-v1',result:{target:{raw:evmTx,network_hint:network},status:'observed',evidence_status:'observed',trust:{observed:true,verified:false,finalized:false},reasons:['READ_ONLY_EVM_TRANSACTION_OBSERVATION'],evidence_refs:['fixture:tx'],transaction_evidence:{transaction_hash:evmTx,block_or_slot:16,address:'0x1111111111111111111111111111111111111111',contract:'0x2222222222222222222222222222222222222222',state_change:'success',attributes:{execution_state:'success',receipt_status:'0x1',log_count:1}}}};
}
test('result identity binds schema, exact address and chain',()=>{
 const request=router.resolve(evm,'base-mainnet');
 assert.equal(router.matchesResult(envelope(),request),true);
 assert.equal(router.matchesResult(envelope(sol),request),false);
 assert.equal(router.matchesResult(envelope(evm,'ethereum-mainnet'),request),false);
 assert.equal(router.matchesResult({...envelope(),schema_version:'unknown'},request),false);
 assert.equal(router.matchesResult({result:null},request),false);
});

// Minimal DOM hosts exercise the real controller with asynchronous transports.
// These are explicit test fixtures, never included in the product bundle.
class Node {
 constructor(){this.value='';this.textContent='';this.hidden=true;this.disabled=false;this.innerHTML='';this.attrs={};this.events={};this.children=[];this.dataset={};}
 addEventListener(name,fn){(this.events[name]??=[]).push(fn);}
 dispatch(name){for(const fn of this.events[name]||[])fn({preventDefault(){}});}
 setAttribute(name,value){this.attrs[name]=value;}
 removeAttribute(name){delete this.attrs[name];}
 focus(){this.focused=true;}
 replaceChildren(...nodes){this.children=nodes;}
}
function harness(fetch,search='',shared=false){
 const ids=['customerUniversalScan','customerUniversalScanForm','customerUniversalTarget','customerUniversalNetwork','customerUniversalSubmit','customerUniversalCancel','customerUniversalStatus','customerUniversalResultsWrap','customerUniversalOverview','customerUniversalResults','customerUniversalRecovery','advancedTools'];
 if(shared)ids.push('scanForm','target','scanNetwork','submit','radarMode','kind','kindLabel','note','transaction','wallet','targetFields','transactionFields','modeSummary','empty','result','shareResult','openExplorer','evmSpendingFields','evmSpendingOwner','evmSpendingSpender','evmSpendingResult');
 const nodes=Object.fromEntries(ids.map(id=>[id,new Node()]));
 if(shared){
  nodes.radarMode.tagName='SELECT';nodes.scanNetwork.options=['','solana-mainnet','base-mainnet','ethereum-mainnet','bitcoin-mainnet'].map(value=>({value,disabled:false}));
  Object.defineProperty(nodes.scanNetwork,'selectedOptions',{get(){return nodes.scanNetwork.options.filter(option=>option.value===nodes.scanNetwork.value);}});
 }
 const timers=new Map();let timerID=0;
 const window={addEventListener(){},KoscheiAuth:{init:async()=>{},isLoggedIn:()=>true,apiCall:(url,options)=>fetch(url,options)}};
 const events={};
 const document={readyState:'complete',getElementById:id=>nodes[id],querySelector:()=>({}),querySelectorAll:()=>[],createElement:()=>new Node(),addEventListener(name,fn){(events[name]??=[]).push(fn);},dispatchEvent(event){for(const fn of events[event.type]||[])fn(event);}};
 const context=vm.createContext({window,URLSearchParams,AbortController,SyntaxError,Error,CustomEvent:class{constructor(type,options){this.type=type;this.detail=options?.detail;}},location:{search,pathname:'/scan'},history:{replaceState(){}},setTimeout(fn){timers.set(++timerID,fn);return timerID;},clearTimeout(id){timers.delete(id);},fetch,document});
 vm.runInContext(source('customer-scan-entry.js'),context);
 if(shared)vm.runInContext(source('public-solana-scan.js'),context);
 vm.runInContext(source('customer-universal-address-scan-v1.js'),context);
 const submit=(target=evm,network='base-mainnet')=>{nodes[shared?'target':'customerUniversalTarget'].value=target;nodes[shared?'scanNetwork':'customerUniversalNetwork'].value=network;nodes[shared?'scanForm':'customerUniversalScanForm'].dispatch('submit');};
 const mode=value=>{nodes.radarMode.value=value;nodes.radarMode.dispatch('change');};
 return {nodes,submit,timers,mode};
}
const settle=()=>new Promise(resolve=>setImmediate(resolve));
const response=(status,data)=>({status,ok:status>=200&&status<300,json:async()=>data});
test('one submit sends one chain-bound request and displays unverified evidence honestly',async()=>{
 const calls=[];const h=harness(async(url,options)=>{calls.push({url,options});return response(200,envelope());});
 h.submit();await settle();
 assert.equal(calls.length,1);assert.equal(calls[0].url,'/api/scan');
 assert.deepEqual(JSON.parse(calls[0].options.body),{target:evm,network:'base-mainnet'});
 assert.equal(h.nodes.customerUniversalResultsWrap.hidden,false);
 assert.match(h.nodes.customerUniversalResults.innerHTML,/UNVERIFIED/);
 assert.match(h.nodes.customerUniversalResults.innerHTML,/fixture:test/);
 assert.doesNotMatch(h.nodes.customerUniversalOverview.innerHTML,/SAFE/);
 assert.equal(h.nodes.customerUniversalSubmit.disabled,false);
});
test('EVM transaction lookup is shipped through the universal scan form and renders execution evidence',async()=>{
 const calls=[];const h=harness(async(url,options)=>{calls.push({url,options});return response(200,transactionEnvelope());});
 h.submit(evmTx,'base-mainnet');await settle();
 assert.equal(calls.length,1);assert.deepEqual(JSON.parse(calls[0].options.body),{target:evmTx,network:'base-mainnet'});
 assert.match(h.nodes.customerUniversalOverview.innerHTML,/transaction/i);
 assert.match(h.nodes.customerUniversalResults.innerHTML,/Transaction success/);
 assert.match(h.nodes.customerUniversalResults.innerHTML,/Execution/);
 assert.match(h.nodes.customerUniversalResults.innerHTML,/0x1/);
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
 assert.equal(h.nodes.customerUniversalSubmit.disabled,false);assert.match(h.nodes.customerUniversalStatus.textContent,/40 seconds/);
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
 await window.fetch('/api/scan');assert.equal(delays.at(-1),40000);
 await window.fetch('/health');assert.equal(delays.at(-1),10000);
});


test('one shared radar form sends address evidence once and suppresses the Solana token adapter',async()=>{
 const calls=[];const h=harness(async(url,options)=>{calls.push({url,options});return response(200,envelope());},'',true);
 h.submit();await settle();
 assert.equal(calls.length,1);assert.equal(calls[0].url,'/api/scan');
 assert.equal(h.nodes.customerUniversalResultsWrap.hidden,false);
});
test('the same form selects a full token investigation without a second address request',async()=>{
 const calls=[];const h=harness(async(url,options)=>{calls.push({url,options});return response(503,{error:'explicit_fixture_source_unavailable'});},'',true);
 h.mode('token');h.submit(sol,'solana-mainnet');await settle();
 assert.equal(calls.length,1);assert.equal(calls[0].url,'/api/token/scan');
 assert.deepEqual(JSON.parse(calls[0].options.body),{mint:sol,network:'solana-mainnet'});
 assert.equal(h.nodes.customerUniversalResultsWrap.hidden,true);
});
test('site selection uses the existing preflight adapter through the same form',async()=>{
 const calls=[];const h=harness(async(url,options)=>{calls.push({url,options});return response(503,{error:'explicit_fixture_source_unavailable'});},'',true);
 h.mode('quick');h.submit('https://example.test','solana-mainnet');await settle();
 assert.equal(calls.length,1);assert.equal(calls[0].url,'/api/arvis/preflight');
 assert.equal(JSON.parse(calls[0].options.body).kind,'site');
});
test('changing target type cancels address evidence and withholds a late response',async()=>{
 let complete;const h=harness(()=>new Promise(resolve=>{complete=resolve;}),'',true);
 h.submit();h.mode('token');complete(response(200,envelope()));await settle();
 assert.equal(h.nodes.customerUniversalResultsWrap.hidden,true);
 assert.equal(h.nodes.submit.disabled,false);
});
test('changing target type also withholds a late full token report',async()=>{
 let complete;const h=harness(()=>new Promise(resolve=>{complete=resolve;}),'',true);
 h.mode('token');h.submit(sol,'solana-mainnet');h.mode('address');
 complete(response(200,{investigation_report:{target:sol}}));await settle();
 assert.equal(h.nodes.result.hidden,true);assert.equal(h.nodes.result.innerHTML,'');
 assert.equal(h.nodes.submit.disabled,false);
});

// Missing evidence references are not silently substituted with an empty list.
test('result identity requires an explicit evidence_refs array',()=>{
 const request=router.resolve(evm,'base-mainnet');
 const withoutRefs=envelope();delete withoutRefs.result.evidence_refs;
 assert.equal(router.matchesResult(withoutRefs,request),false);
 assert.equal(router.matchesResult({...envelope(),result:{...envelope().result,evidence_refs:null}},request),false);
 assert.equal(router.matchesResult({...envelope(),result:{...envelope().result,evidence_refs:[]}},request),true);
});

test('malformed trust, status and evidence reference values fail closed',()=>{
 const request=router.resolve(evm,'base-mainnet');
 const base=envelope();
 assert.equal(router.matchesResult({...base,result:{...base.result,trust:[]}},request),false);
 assert.equal(router.matchesResult({...base,result:{...base.result,status:''}},request),false);
 assert.equal(router.matchesResult({...base,result:{...base.result,evidence_refs:[null]}},request),false);
 assert.equal(router.matchesResult({...base,result:{...base.result,evidence_refs:[{id:'fake'}]}},request),false);
 assert.equal(router.matchesResult({...base,result:{...base.result,evidence_refs:[]}},request),true);
});

test('blank evidence reference entries are rejected without requiring evidence to exist',()=>{
 const request=router.resolve(evm,'base-mainnet');
 const base=envelope();
 for(const ref of ['', '   ', '\n\t']){
  assert.equal(router.matchesResult({...base,result:{...base.result,evidence_refs:[ref]}},request),false);
 }
 assert.equal(router.matchesResult({...base,result:{...base.result,evidence_refs:[]}},request),true);
 assert.equal(router.matchesResult(envelope(),request),true);
});

test('a Solana address scan starts the retained ARVIS job without a second click',async()=>{
 const id='12345678-1234-1234-1234-123456789abc',calls=[];
 const full={final_verdict:{signed:false,grade:'-',limitation:'fixture_not_attested'},
   all_collector_rows:[{module:'fixture',provenance:{source:'synthetic',height:47}}],
   nested_technical_facts:{relation_history:[1,2,3]}};
 const h=harness(async(url,options)=>{
   calls.push({url,options});
   if(url==='/api/scan')return response(200,envelope(sol,'solana-mainnet'));
   if(url==='/api/v1/radar/jobs')return response(201,{job_id:id,poll_url:'/api/v1/radar/jobs/'+id,canonical_engine:true});
   if(url==='/api/v1/radar/jobs/'+id)return response(200,{id,job_type:'canonical_investigation',status:'completed',target:sol,network:'solana-mainnet',result:full});
   throw new Error('unexpected fixture route '+url);
 });
 h.submit(sol,'solana-mainnet');await settle();await settle();await settle();
 const button=h.nodes.customerUniversalRecovery.children[0];
 assert.equal(button.textContent,'Run durable ARVIS investigation');
 assert.deepEqual(calls.map(c=>c.url.split('?')[0]),['/api/scan','/api/v1/radar/jobs/','/api/v1/radar/jobs','/api/v1/radar/jobs/'+id],'scan, retained-job lookup, job creation and one status read');
 assert.deepEqual(JSON.parse(calls[2].options.body),{address:sol,network:'solana-mainnet',max_depth:3});
 assert.equal(h.nodes.customerUniversalRecovery.children[1].href,'/reports?job='+encodeURIComponent(id));
 assert.match(h.nodes.customerUniversalResults.innerHTML,/Complete retained ARVIS report/);
 assert.match(h.nodes.customerUniversalResults.innerHTML,/relation_history/);
 assert.match(h.nodes.customerUniversalResults.innerHTML,/fixture_not_attested/);
 assert.doesNotMatch(calls.map(c=>c.url).join('|'),/\/api\/owner\//);
});

test('non-Solana observed scan offers a distinctly limited native evidence job, not Solana grade',async()=>{
 const calls=[];
 const h=harness(async(url,options)=>{calls.push({url,options});return response(200,envelope(evm,'base-mainnet'));});
 h.submit(evm,'base-mainnet');await settle();
 assert.equal(h.nodes.customerUniversalRecovery.hidden,false);
 assert.equal(h.nodes.customerUniversalRecovery.children[0].textContent,'Run and save native evidence');
 assert.equal(calls.length,1,'no durable job is submitted without a customer click');
});

test('deep customer job rejects crossed target result and preserves the initial evidence',async()=>{
 const id='12345678-1234-1234-1234-123456789abc';
 const h=harness(async(url)=>{
   if(url==='/api/scan')return response(200,envelope(sol,'solana-mainnet'));
   if(url==='/api/v1/radar/jobs')return response(201,{job_id:id,poll_url:'/api/v1/radar/jobs/'+id,canonical_engine:true});
   return response(200,{id,job_type:'canonical_investigation',status:'completed',target:evm,network:'solana-mainnet',result:{fabricated_risk:'SAFE'}});
 });
 h.submit(sol,'solana-mainnet');await settle();
 h.nodes.customerUniversalRecovery.children[0].dispatch('click');await settle();await settle();
 assert.doesNotMatch(h.nodes.customerUniversalResults.innerHTML,/fabricated_risk/);
 assert.match(h.nodes.customerUniversalResults.innerHTML,/Evidence references/);
 assert.match(h.nodes.customerUniversalStatus.textContent,/history/);
});

test('EVM native job displays retained evidence without implying completed ARVIS grading',async()=>{
 const id='12345678-1234-1234-1234-123456789abc',calls=[];
 const native={schema_version:'koschei-canonical-native-investigation-v1',status:'evidence_only_partial',
   customer_scan:{network_evidence:{source:'fixture-evm',attributes:{chain_id_response_sha256:'synthetic-digest'}}},
   final_verdict:{grade:'-',signed:false,withheld:true,risk_level:'unknown'}};
 const h=harness(async(url,options)=>{
   calls.push({url,options});
   if(url==='/api/scan')return response(200,envelope(evm,'base-mainnet'));
   if(url==='/api/v1/radar/jobs')return response(201,{job_id:id,poll_url:'/api/v1/radar/jobs/'+id,canonical_engine:true});
   if(url==='/api/v1/radar/jobs/'+id)return response(200,{id,job_type:'canonical_investigation',status:'completed',target:evm,network:'base-mainnet',result:native});
   throw new Error('unexpected endpoint '+url);
 });
 h.submit(evm,'base-mainnet');await settle();
 h.nodes.customerUniversalRecovery.children[0].dispatch('click');await settle();await settle();
 assert.equal(calls.length,3);
 assert.deepEqual(JSON.parse(calls[1].options.body),{address:evm,network:'base-mainnet',max_depth:0});
 assert.match(h.nodes.customerUniversalResults.innerHTML,/chain_id_response_sha256/);
 assert.match(h.nodes.customerUniversalResults.innerHTML,/historical collectors and a signed grade are NOT claimed/);
 assert.doesNotMatch(calls.map(c=>c.url).join('|'),/\/api\/owner\//);
});

test('transaction evidence is retained by a customer-owned job without claiming a signed grade',async()=>{
 const id='12345678-1234-1234-1234-123456789abc';
 const native={schema_version:'koschei-canonical-native-investigation-v1',status:'evidence_only_partial',investigation_kind:'native_network_transaction_observation',transaction_evidence:{attributes:{receipt_status:'0x1'}},final_verdict:{signed:false,withheld:true,grade:'-'}};
 const calls=[];
 const h=harness(async(url,options)=>{
   calls.push({url,options});
   if(url==='/api/scan')return response(200,transactionEnvelope());
   if(url==='/api/v1/radar/jobs')return response(201,{job_id:id,poll_url:'/api/v1/radar/jobs/'+id,canonical_engine:true});
   if(url==='/api/v1/radar/jobs/'+id)return response(200,{id,job_type:'canonical_investigation',status:'completed',target:evmTx,network:'base-mainnet',result:native});
   throw Error('unexpected endpoint '+url);
 });
 h.submit(evmTx,'base-mainnet');await settle();
 const button=h.nodes.customerUniversalRecovery.children[0];
 assert.equal(button.textContent,'Run and save transaction evidence');
 button.dispatch('click');await settle();await settle();
 assert.equal(calls.length,3);
 assert.deepEqual(JSON.parse(calls[1].options.body),{address:evmTx,network:'base-mainnet',max_depth:0});
 assert.equal(h.nodes.customerUniversalRecovery.children[1].href,'/reports?job='+encodeURIComponent(id));
 assert.match(h.nodes.customerUniversalResults.innerHTML,/native_network_transaction_observation/);
 assert.doesNotMatch(calls.map(c=>c.url).join('|'),/\/api\/owner\//);
});

// Explicit synthetic fixture shaped like a retained Solana ARVIS report.
function readableFixture(){
 return {target:sol,network:'solana-mainnet',
  final_verdict:{grade:'B',signed:true,verdict:'compounding_rule',ruleset_version:'fixture-rules-v1',signature:'fixture-signature-value-not-real',generated_at:'2026-10-26T19:41:38Z',
   triggered_rules:[{rule_id:'ARD-C001',title:'Creator/deployer reuse',summary:'fixture creator linked to 125 tokens',evidence_status:'verified'},
    ...Array.from({length:5},(_,i)=>({rule_id:'ARD-C004',title:'Repeated direct transfer relation',summary:'fixture relation '+i,evidence_status:'verified'}))],
   watch_flags:[{rule_id:'ARD-W002',title:'Possible dust',summary:'fixture dust observation'}]},
  investor_protection_decision:{decision:'NOT_CLEARED',summary:'fixture: missing evidence is not converted into a safe result',critical_gaps:[],evidence_limits:[{capability:'fixture collector',reason:'fixture history bounded'}]},
  market:{available:true,name:'Fixture Cat',symbol:'FCAT',liquidity_usd:3694.49,market_cap_usd:2993,volume_24h_usd:0.29,price_usd:0.000003,provider:'fixture-market',observed_at:'2026-10-26T19:40:05Z'},
  shadow_rules:[{rule_id:'ARD-C006',title:'Verified serial launch factory',status:'triggered_shadow',triggered:true,summary:'fixture: 40 launches in one hour',evidence:[{mint:'fixture-mint'}],limitations:['Shadow rule: fixture limitation']}],
  investigation_coverage:{status:'partial_investigation',completed:12,capability_total:14,evidence_producing:12,evidence_pending:2,source_unavailable:0,critical_gaps:[],
   arms:[{module:'Pool Control Guardian',subchecks:{checks:{pool_reserves:'performed',pool_program_relation:'unknown'},required:['pool_reserves'],required_checks_status:'partial'}}]},
  modules:[],collector_only_marker:'fixture-raw-collector-row'};
}
test('a completed Solana report reads as decision, rules, shadow rules and coverage, not a raw dump',async()=>{
 const id='12345678-1234-1234-1234-123456789abc';
 const h=harness(async(url)=>{
   if(url==='/api/scan')return response(200,envelope(sol,'solana-mainnet'));
   if(url==='/api/v1/radar/jobs')return response(201,{job_id:id,poll_url:'/api/v1/radar/jobs/'+id,canonical_engine:true});
   return response(200,{id,job_type:'canonical_investigation',status:'completed',target:sol,network:'solana-mainnet',result:readableFixture()});
 });
 h.submit(sol,'solana-mainnet');for(let i=0;i<4;i++)await settle();
 const html=h.nodes.customerUniversalResults.innerHTML;
 assert.match(html,/Signed ARVIS grade B/);
 assert.match(html,/NOT CLEARED/);
 assert.match(html,/ARD-C004 · Repeated direct transfer relation/);
 assert.match(html,/2 more/,'long rule groups fold after three rows');
 assert.match(html,/Shadow rules · do not change the grade/);
 assert.match(html,/pool reserves: done/);
 assert.match(html,/pool program relation: unknown/);
 assert.match(html,/fixture history bounded/);
 assert.match(html,/Pre-check · live account observation/);
 assert.doesNotMatch(html,/fixture-raw-collector-row/,'the raw report is rendered only when the technical section is opened');
 assert.doesNotMatch(html,/\bSAFE\b/);
 assert.equal(h.nodes.customerUniversalOverview.innerHTML,'');
});
test('an unsigned report says no grade was issued instead of implying safety',async()=>{
 const id='12345678-1234-1234-1234-123456789abc',report=readableFixture();
 report.final_verdict={signed:false,grade:'-'};report.market={available:false};
 const h=harness(async(url)=>{
   if(url==='/api/scan')return response(200,envelope(sol,'solana-mainnet'));
   if(url==='/api/v1/radar/jobs')return response(201,{job_id:id,poll_url:'/api/v1/radar/jobs/'+id,canonical_engine:true});
   return response(200,{id,job_type:'canonical_investigation',status:'completed',target:sol,network:'solana-mainnet',result:report});
 });
 h.submit(sol,'solana-mainnet');for(let i=0;i<4;i++)await settle();
 const html=h.nodes.customerUniversalResults.innerHTML;
 assert.match(html,/No signed grade was issued/);
 assert.match(html,/Liquidity and price are UNKNOWN/);
 assert.match(html,/No grade-changing rule was triggered. This does not mean the token is safe/);
});
test('a repeated request that joins a running job says no extra output was used',async()=>{
 const id='12345678-1234-1234-1234-123456789abc';
 const h=harness(async(url)=>{
   if(url==='/api/scan')return response(200,envelope(sol,'solana-mainnet'));
   if(url==='/api/v1/radar/jobs')return response(200,{job_id:id,poll_url:'/api/v1/radar/jobs/'+id,canonical_engine:true,joined_existing_job:true,charged:false});
   return response(200,{id,job_type:'canonical_investigation',status:'running',progress:40,target:sol,network:'solana-mainnet'});
 });
 h.submit(sol,'solana-mainnet');await settle();await settle();
 assert.match(h.nodes.customerUniversalOverview.innerHTML,/INVESTIGATING/);
 assert.match(h.nodes.customerUniversalStatus.textContent,/no additional output was used/);
 assert.match(h.nodes.customerUniversalResults.innerHTML,/running \(40%\)/);
});

test('opening a Solana result link runs only the free pre-check and starts no paid job',async()=>{
 const calls=[];
 const h=harness(async(url,options)=>{calls.push(url);
   if(url.startsWith('/api/v1/radar/jobs/?'))return response(200,{history:[]});
   return response(200,envelope(sol,'solana-mainnet'));},'?mode=address&network=solana-mainnet&target='+sol);
 for(let i=0;i<3;i++)await settle();
 assert.deepEqual(calls,['/api/scan','/api/v1/radar/jobs/?target='+sol+'&network=solana-mainnet']);
 assert.equal(h.nodes.customerUniversalRecovery.children[0].textContent,'Run durable ARVIS investigation');
});
for(const [state,label] of [['failed',/INVESTIGATION FAILED/],['completed',/^$/]])test(`a ${state} job never leaves the INVESTIGATING headline behind`,async()=>{
 const id='12345678-1234-1234-1234-123456789abc';
 const h=harness(async(url)=>{
   if(url==='/api/scan')return response(200,envelope(sol,'solana-mainnet'));
   if(url==='/api/v1/radar/jobs')return response(201,{job_id:id,poll_url:'/api/v1/radar/jobs/'+id,canonical_engine:true});
   // A wallet report has no modules or coverage, so the readable renderer does not apply.
   return response(200,{id,job_type:'canonical_investigation',status:state,target:sol,network:'solana-mainnet',result:{wallet:sol,final_verdict:{grade:'-',signed:false}}});
 });
 h.submit(sol,'solana-mainnet');for(let i=0;i<4;i++)await settle();
 assert.doesNotMatch(h.nodes.customerUniversalOverview.innerHTML,/INVESTIGATING/);
 assert.match(h.nodes.customerUniversalOverview.innerHTML,label);
});

// Mobile browsers freeze or reload a backgrounded tab mid-investigation. The
// page must find the account's retained job again instead of paying twice.
const retainedID='87654321-4321-4321-4321-cba987654321';
function retainedHarness(item,search){
 const calls=[];
 const h=harness(async(url,options)=>{
   calls.push({url,method:options?.method||'GET'});
   if(url==='/api/scan')return response(200,envelope(sol,'solana-mainnet'));
   if(url.startsWith('/api/v1/radar/jobs/?'))return response(200,{history:item?[item]:[]});
   if(url==='/api/v1/radar/jobs/'+retainedID)return response(200,{id:retainedID,job_type:'canonical_investigation',status:item.status,progress:item.progress,target:sol,network:'solana-mainnet',result:item.status==='completed'?readableFixture():undefined});
   if(url==='/api/v1/radar/jobs')return response(201,{job_id:'12345678-1234-1234-1234-123456789abc',poll_url:'/api/v1/radar/jobs/12345678-1234-1234-1234-123456789abc',canonical_engine:true});
   return response(200,{id:'12345678-1234-1234-1234-123456789abc',job_type:'canonical_investigation',status:'running',progress:5,target:sol,network:'solana-mainnet'});
 },search);
 return {h,calls,posts:()=>calls.filter(c=>c.method==='POST'&&c.url==='/api/v1/radar/jobs').length};
}
const minutesAgo=n=>new Date(Date.now()-n*60000).toISOString();
const deepLink='?mode=address&network=solana-mainnet&target='+sol;
test('a reopened link follows the running investigation without a new charge',async()=>{
 const r=retainedHarness({id:retainedID,status:'running',progress:40,target:sol,network:'solana-mainnet',updated_at:minutesAgo(1)},deepLink);
 for(let i=0;i<5;i++)await settle();
 assert.equal(r.posts(),0);
 assert.ok(r.calls.some(c=>c.url==='/api/v1/radar/jobs/'+retainedID));
 assert.match(r.h.nodes.customerUniversalStatus.textContent,/no additional output was used/);
 assert.match(r.h.nodes.customerUniversalResults.innerHTML,/running \(40%\)/);
});
test('a reopened link shows the finished report for free and offers a fresh run',async()=>{
 const r=retainedHarness({id:retainedID,status:'completed',result_available:true,target:sol,network:'solana-mainnet',updated_at:minutesAgo(240)},deepLink);
 for(let i=0;i<5;i++)await settle();
 assert.equal(r.posts(),0);
 assert.match(r.h.nodes.customerUniversalResults.innerHTML,/Signed ARVIS grade B/);
 assert.match(r.h.nodes.customerUniversalStatus.textContent,/retained report from .*no output was used/);
 assert.equal(r.h.nodes.customerUniversalRecovery.children[0].textContent,'Run a fresh investigation');
});
test('an explicit scan reuses a report finished in the last 30 minutes',async()=>{
 const r=retainedHarness({id:retainedID,status:'completed',result_available:true,target:sol,network:'solana-mainnet',updated_at:minutesAgo(10)});
 r.h.submit(sol,'solana-mainnet');for(let i=0;i<5;i++)await settle();
 assert.equal(r.posts(),0);
 assert.match(r.h.nodes.customerUniversalResults.innerHTML,/Signed ARVIS grade B/);
});
test('an explicit scan starts a new investigation when the last report is older',async()=>{
 const r=retainedHarness({id:retainedID,status:'completed',result_available:true,target:sol,network:'solana-mainnet',updated_at:minutesAgo(45)});
 r.h.submit(sol,'solana-mainnet');for(let i=0;i<5;i++)await settle();
 assert.equal(r.posts(),1);
});
test('a fresh run from a reopened report is an explicit, charged request',async()=>{
 const r=retainedHarness({id:retainedID,status:'completed',result_available:true,target:sol,network:'solana-mainnet',updated_at:minutesAgo(5)},deepLink);
 for(let i=0;i<5;i++)await settle();
 r.h.nodes.customerUniversalRecovery.children[0].dispatch('click');for(let i=0;i<4;i++)await settle();
 assert.equal(r.posts(),1);
});

test('the readable report shows every section: shadow alert, threat paths, behavior rules, all modules and cited tokens',async()=>{
 const id='12345678-1234-1234-1234-123456789abc',report=readableFixture();
 const mint='So11111111111111111111111111111111111111112',sig='5'.repeat(88);
 report.shadow_rules[0]={...report.shadow_rules[0],verified_prior_tokens:36,inactive_or_dead:36,known_fate_tokens:36,max_launches_per_hour:30,creator_wallet:mint,
   evidence:[{mint,creation_signature:sig,created_on_chain_at:'2026-10-26T05:41:16Z',fate_status:'inactive_or_dead'},{mint:'javascript:alert(1)',creation_signature:'not-a-signature',fate_status:'inactive_or_dead'}]};
 report.final_verdict.triggered_rules[1].signatures=[sig,'<script>'];
 report.threat_anticipation={primary_exposure:'fixture exposure',exit_capacity:{available:true,owner_percentage:14.0154,dominant_owner_wallet:mint,owner_reference_usd_value:39.23,position_liquidity_multiple:0.01,capacity:'elevated',interpretation:'fixture interpretation'},
   pathways:[{id:'dominant_holder_exit',label:'Dominant-holder market exit',status:'open',summary:'fixture open path'},{id:'liquidity_removal',label:'Liquidity removal',status:'unknown',summary:'fixture unknown path'}],
   rug_pathway_assessment:{conclusion:'fixture pathway conclusion'},missing_evidence:['LP mint and LP token owner'],scenarios:[],watch_signals:[]};
 report.behavior_signals={signals:[{rule_id:'URD-C004',title:'Dominant-holder first observed exit',triggered:false,evidence_status:'unverified',summary:'fixture not evaluated'},{rule_id:'URD-C005',title:'Owner-resolved dominant concentration',triggered:false,evidence_status:'verified',summary:'fixture below threshold'}]};
 report.modules=[{module:'Pool Control Guardian',module_id:'raydium_pool_guardian',signed:true,verified:true,verdict:'fixture pool verdict',evidence:['fixture pool evidence line one','fixture pool evidence line two']},
   {module:'Claim Surface Risk',module_id:'claim_surface_risk',signed:false,verdict:'Real data unavailable.',evidence:[]}];
 report.investigation_coverage.arms[0].module_id='raydium_pool_guardian';
 const h=harness(async(url)=>{
   if(url==='/api/scan')return response(200,envelope(sol,'solana-mainnet'));
   if(url==='/api/v1/radar/jobs')return response(201,{job_id:id,poll_url:'/api/v1/radar/jobs/'+id,canonical_engine:true});
   return response(200,{id,job_type:'canonical_investigation',status:'completed',target:sol,network:'solana-mainnet',result:report});
 });
 h.submit(sol,'solana-mainnet');for(let i=0;i<5;i++)await settle();
 const html=h.nodes.customerUniversalResults.innerHTML;
 assert.match(html,/PATTERN DETECTED · GRADE NOT CHANGED/);
 assert.ok(html.indexOf('PATTERN DETECTED')<html.indexOf('Rules that set the grade'),'the triggered shadow rule is shown before the rule list');
 assert.match(html,/All 2 cited token creations/);
 assert.match(html,new RegExp('https://solscan.io/token/'+mint));
 assert.match(html,new RegExp('https://solscan.io/tx/'+sig));
 assert.doesNotMatch(html,/href="[^"]*javascript:/,'non-base58 values are never turned into links');
 assert.doesNotMatch(html,/<script/i,"report text is escaped, never injected as markup");
 assert.match(html,/How the risk could play out/);
 assert.match(html,/Dominant-holder market exit/);
 assert.match(html,/Evidence still missing for a decision:/);
 assert.match(html,/URD-C004 · Dominant-holder first observed exit<\/span><b>NOT EVALUATED/);
 assert.match(html,/All ARVIS modules \(2\)/);
 assert.match(html,/fixture pool evidence line two/);
 assert.match(html,/Claim Surface Risk · EVIDENCE PENDING/);
 assert.match(html,/This module produced no evidence lines/);
 assert.doesNotMatch(html,/\bSAFE\b/);
});

// The report leads with who is behind the target and what that wallet did.
function ownerFixture(){
 const creator='CreatorWa11etFixture1111111111111111111111',other='Counterparty1111111111111111111111111111111';
 const mintA='MintA1111111111111111111111111111111111111',mintB='MintB1111111111111111111111111111111111111',sig='4'.repeat(88);
 return {creator,other,mintA,mintB,sig,actor:{wallet:creator,
  current_creator_relation:{status:'verified',evidence:{program:'pump.fun'},target:{mint:sol,creator_wallet:creator,first_observed_at:'2026-09-15T16:43:01Z',creation_signature:sig,verification_status:'verified'}},
  dossier:{track:{created_token_count:145,first_seen_at:'2026-09-15T16:43:01Z',last_seen_at:'2026-10-27T01:16:05Z',dossier:{verified_creator_token_count:23}}},
  funding_origin:{status:'funding_not_observed',signatures_scanned:2000,boundary:{reason:'Funding origin was not reachable within the effective transaction parse limit.'}},
  external_discovery:{
   address_attribution:{status:'no_verified_attribution',entities:[]},
   behavior_summary:{primary_observations:['The exact address has verified creation evidence for 6 distinct token mint(s).']},
   creator_outcome_history:{verified_token_count:2,outcomes:[
     {mint:mintA,created_on_chain_at:'2026-10-26T05:41:16Z',fate_status:'inactive_or_dead',current_liquidity_usd:0,creation_signature:sig},
     {mint:mintB,created_on_chain_at:'2026-10-27T01:01:45Z',fate_status:'inactive_or_dead',current_liquidity_usd:0,creation_signature:sig}],limitations:['fixture outcome limitation']},
   created_mint_portfolio:{lifecycle_summary:{average_inactive_age_days:0.68}},
   behavior_timeline:{events:[
     {event_type:'mint_created',token_mint:mintA,observed_at:'2026-10-26T05:41:16Z',signature:sig},
     {event_type:'transfer_out',counterparty:other,amount_native:0.00175,asset_type:'SOL',observed_at:'2026-10-27T01:09:14Z',signature:sig}]},
   address_relationships:{relationships:[{address:other,relation:'outbound_direct_flow',outbound_transfers:3,inbound_transfers:0,assets:['SPL_TOKEN'],token_mints:[mintB],first_observed_at:'2026-10-27T01:09:14Z',last_observed_at:'2026-10-27T01:09:16Z',evidence_signatures:[sig]}]},
   behavior_patterns:{matches:[{pattern_id:'KOSCH-ADDR-BEH-001',label:'Repeated direct interaction with the same counterparty',triggered:true,explanation:'fixture explanation'},{pattern_id:'KOSCH-ADDR-BEH-002',label:'Bidirectional direct fund-flow relationship',triggered:false,status:'not_triggered'}]},
   address_history:{signatures_seen:500,history_complete:false,limitations:['fixture history limitation']}}}};
}
async function renderCompleted(result){
 const id='12345678-1234-1234-1234-123456789abc';
 const h=harness(async(url)=>{
   if(url==='/api/scan')return response(200,envelope(sol,'solana-mainnet'));
   if(url==='/api/v1/radar/jobs')return response(201,{job_id:id,poll_url:'/api/v1/radar/jobs/'+id,canonical_engine:true});
   return response(200,{id,job_type:'canonical_investigation',status:'completed',target:sol,network:'solana-mainnet',result});
 });
 h.submit(sol,'solana-mainnet');for(let i=0;i<5;i++)await settle();
 return h.nodes.customerUniversalResults.innerHTML;
}
test('a token report opens with who created it and everything that wallet has done',async()=>{
 const f=ownerFixture(),report=readableFixture();report.actor_investigation=f.actor;
 const html=await renderCompleted(report);
 assert.match(html,/Who is behind this token and what they have done/);
 assert.ok(html.indexOf('Who is behind this token')<html.indexOf('Signed ARVIS grade B'),'owner history comes before the grade');
 assert.match(html,new RegExp('https://solscan.io/account/'+f.creator));
 assert.match(html,/Tokens created \(indexed\)<\/span><b>145/);
 assert.match(html,/Tokens this wallet created \(2 verified of 145 indexed\)/);
 assert.match(html,/<b>2<\/b> inactive or dead/);
 assert.match(html,new RegExp('https://solscan.io/token/'+f.mintA));
 assert.match(html,/Activity timeline \(2 decoded events, newest first\)/);
 assert.ok(html.indexOf('Sent 0.00175 SOL')<html.indexOf('Created token'),'newest event first');
 assert.match(html,/Wallets it moved funds or tokens with \(1\)/);
 assert.match(html,/<b>UNKNOWN\.<\/b> Funding origin was not reachable/);
 assert.match(html,/No verified identity label exists/);
 assert.match(html,/Repeated direct interaction with the same counterparty<\/b> · OBSERVED/);
 assert.match(html,/partial; older activity may exist/);
 assert.match(html,/not proof of a rug pull/);
 assert.doesNotMatch(html,/\bSAFE\b/);
});
test('a wallet investigation is readable and leads with what the wallet has done',async()=>{
 const f=ownerFixture();
 const html=await renderCompleted({wallet:f.creator,target:f.creator,final_verdict:{grade:'-',signed:true},actor_investigation:{...f.actor,current_creator_relation:undefined}});
 assert.match(html,/What this wallet has done/);
 assert.match(html,/No signed grade was issued/);
 assert.doesNotMatch(html,/Market snapshot|Investigation coverage/,'sections a wallet report does not carry are left out');
 assert.doesNotMatch(html,/complete collector evidence/,'no raw dump for a wallet report');
});
