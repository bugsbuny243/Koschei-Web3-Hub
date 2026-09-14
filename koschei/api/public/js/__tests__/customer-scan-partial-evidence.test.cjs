const {test}=require('node:test');
const assert=require('node:assert/strict');
const vm=require('node:vm');
const fs=require('node:fs');
const path=require('node:path');

const source=fs.readFileSync(path.join(__dirname,'..','customer-scan-entry.js'),'utf8');
const context=vm.createContext({window:{},URLSearchParams,location:{search:'',pathname:'/scan'}});
vm.runInContext(source,context);
const router=context.window.KoscheiScanEntry;
const address='0x1111111111111111111111111111111111111111';
const request=router.resolve(address,'base-mainnet');

function envelope(){
  return {schema_version:'koschei-customer-scan-v1',result:{
    target:{raw:address,network_hint:'base-mainnet'},
    status:'observed',verdict:'review',evidence_status:'observed',
    trust:{claimed:false,observed:true,authorized:false,available:false,verified:false,finalized:false},
    reasons:['READ_ONLY_NETWORK_OBSERVATION'],evidence_refs:['kis_fixture']
  }};
}

test('current EVM partial evidence requires both authority-unavailable markers',()=>{
  const value=envelope();
  value.result.reasons.push('EVM_AUTHORITY_PROBE_UNAVAILABLE','PARTIAL_EVIDENCE_EVM_AUTHORITY_UNAVAILABLE');
  assert.equal(router.matchesResult(value,request),true);

  const missingMarker=envelope();
  missingMarker.result.reasons.push('EVM_AUTHORITY_PROBE_UNAVAILABLE');
  assert.equal(router.matchesResult(missingMarker,request),false);
});

test('partial marker cannot coexist with attached EVM authority',()=>{
  const value=envelope();
  value.result.reasons.push('EVM_AUTHORITY_PROBE_UNAVAILABLE','PARTIAL_EVIDENCE_EVM_AUTHORITY_UNAVAILABLE');
  value.result.evm_authority={
    network:'base-mainnet',spender:address,
    trust:{claimed:false,observed:true,authorized:false,available:false,verified:false,finalized:false}
  };
  assert.equal(router.matchesResult(value,request),false);
});

test('EVM partial markers are rejected on a non-EVM result',()=>{
  const sol='So11111111111111111111111111111111111111112';
  const solRequest=router.resolve(sol,'solana-mainnet');
  const value={schema_version:'koschei-customer-scan-v1',result:{
    target:{raw:sol,network_hint:'solana-mainnet'},status:'observed',verdict:'review',evidence_status:'observed',
    trust:{claimed:false,observed:true,authorized:false,available:false,verified:false,finalized:false},
    reasons:['EVM_AUTHORITY_PROBE_UNAVAILABLE','PARTIAL_EVIDENCE_EVM_AUTHORITY_UNAVAILABLE'],evidence_refs:['solana:fixture']
  }};
  assert.equal(router.matchesResult(value,solRequest),false);
});
