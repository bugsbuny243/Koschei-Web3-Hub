const {test}=require('node:test');
const assert=require('node:assert/strict');
const vm=require('node:vm');
const fs=require('node:fs');
const path=require('node:path');

const source=fs.readFileSync(path.join(__dirname,'..','investigation-share.js'),'utf8');
function load(){
  const scope=vm.createContext({
    window:{open:()=>null},
    document:{readyState:'loading',addEventListener(){},getElementById(){return null;}},
    location:{origin:'https://koschei.test',pathname:'/scan'},
    URLSearchParams,
    MutationObserver:class{observe(){}}
  });
  vm.runInContext(source,scope);
  return scope.window.KoscheiInvestigationShare;
}

const evm='0x1111111111111111111111111111111111111111';

test('Base address share preserves target and network in the public result URL',()=>{
  const share=load();
  const url=new URL(share.publicResultURL(evm,'address','base-mainnet'));
  assert.equal(url.pathname,'/scan');
  assert.equal(url.searchParams.get('mode'),'address');
  assert.equal(url.searchParams.get('target'),evm);
  assert.equal(url.searchParams.get('network'),'base-mainnet');
  assert.equal(url.searchParams.get('source'),'x_share');
});

test('chain-aware share copy reports observed evidence without manufacturing safety',()=>{
  const share=load();
  const text=share.buildText({target:evm,kind:'address',network:'base-mainnet',networkLabel:'Base',evidence_status:'observed',status:'observed',verdict:'review',reasons:[]});
  assert.match(text,/Koschei Web3 · Base/);
  assert.match(text,/Evidence: OBSERVED/);
  assert.match(text,/Verdict: REVIEW/);
  assert.match(text,/#Base/);
  assert.match(text,/Missing evidence is not proof of safety\./);
  assert.doesNotMatch(text,/\bSAFE\b/);
});

test('partial EVM authority evidence is explicit in X copy',()=>{
  const share=load();
  const text=share.buildText({target:evm,kind:'address',network:'ethereum-mainnet',evidence_status:'observed',status:'observed',verdict:'review',reasons:['EVM_AUTHORITY_PROBE_UNAVAILABLE','PARTIAL_EVIDENCE_EVM_AUTHORITY_UNAVAILABLE']});
  assert.match(text,/Koschei Web3 · Ethereum/);
  assert.match(text,/Coverage: PARTIAL/);
  assert.match(text,/authority evidence unavailable/);
  assert.match(text,/#Ethereum/);
});

test('legacy Solana token links stay on the token route',()=>{
  const share=load();
  const mint='So11111111111111111111111111111111111111112';
  const url=new URL(share.publicResultURL(mint,'token','solana-mainnet'));
  assert.equal(url.pathname,'/scan/'+mint);
  const text=share.buildText({target:mint,kind:'token',network:'solana-mainnet',status:'evidence_pending',grade:'B'});
  assert.match(text,/Koschei Web3 · Solana/);
  assert.match(text,/Coverage: PARTIAL/);
  assert.match(text,/#Solana/);
});

test('X sharing remains a user-reviewed intent and never an API post',()=>{
  const share=load();
  const intent=share.buildIntent({target:evm,kind:'address',network:'base-mainnet',evidence_status:'observed',status:'observed',verdict:'review'});
  const url=new URL(intent);
  assert.equal(url.origin,'https://x.com');
  assert.equal(url.pathname,'/intent/tweet');
  assert.ok(url.searchParams.get('text'));
  assert.match(url.searchParams.get('url'),/network=base-mainnet/);
});
