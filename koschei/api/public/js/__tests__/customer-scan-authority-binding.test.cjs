const {test}=require('node:test');
const assert=require('node:assert/strict');
const vm=require('node:vm');
const fs=require('node:fs');
const path=require('node:path');

const source=fs.readFileSync(path.join(__dirname,'..','customer-scan-entry.js'),'utf8');
const evm='0x1111111111111111111111111111111111111111';
const scope=vm.createContext({window:{},URLSearchParams,location:{search:'',pathname:'/scan'}});
vm.runInContext(source,scope);
const router=scope.window.KoscheiScanEntry;
const request=router.resolve(evm,'base-mainnet');
const observedTrust=()=>({claimed:false,observed:true,authorized:false,available:false,verified:false,finalized:false});
function envelope(authority){
  return {schema_version:'koschei-customer-scan-v1',result:{
    target:{raw:evm,network_hint:'base-mainnet'},status:'observed',verdict:'review',evidence_status:'observed',
    trust:observedTrust(),reasons:['READ_ONLY_NETWORK_OBSERVATION'],evidence_refs:['fixture:evm'],
    ...(authority===undefined?{}:{evm_authority:authority})
  }};
}
function authority(overrides={}){
  return {network:'base-mainnet',spender:evm,contract_code_state:'contract_code_observed',proxy_state:'erc1967_authority_not_observed',trust:observedTrust(),reasons:['READ_ONLY_SPENDER_AUTHORITY_OBSERVATION'],...overrides};
}

test('universal EVM scan accepts authority only when nested subject and network are bound',()=>{
  assert.equal(router.matchesResult(envelope(authority()),request),true);
  assert.equal(router.matchesResult(envelope(authority({network:'ethereum-mainnet'})),request),false);
  assert.equal(router.matchesResult(envelope(authority({spender:'0x2222222222222222222222222222222222222222'})),request),false);
});

test('nested authority must remain observed-only evidence',()=>{
  assert.equal(router.matchesResult(envelope(authority({trust:{claimed:false,observed:false,authorized:false,available:false,verified:false,finalized:false}})),request),false);
  assert.equal(router.matchesResult(envelope(authority({trust:{claimed:false,observed:true,authorized:true,available:false,verified:false,finalized:false}})),request),false);
  assert.equal(router.matchesResult(envelope(authority({trust:{claimed:false,observed:true,authorized:false,available:false,verified:true,finalized:false}})),request),false);
});

test('non-EVM result cannot attach EVM authority evidence',()=>{
  const sol='11111111111111111111111111111111';
  const solRequest=router.resolve(sol,'solana-mainnet');
  const data={schema_version:'koschei-customer-scan-v1',result:{target:{raw:sol,network_hint:'solana-mainnet'},status:'observed',verdict:'review',evidence_status:'observed',trust:observedTrust(),reasons:['FIXTURE'],evidence_refs:['fixture:sol'],evm_authority:authority({network:'solana-mainnet',spender:sol})}};
  assert.equal(router.matchesResult(data,solRequest),false);
});
