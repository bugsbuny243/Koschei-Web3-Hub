import test from 'node:test';
import assert from 'node:assert/strict';
import {observeOwnerSOC} from './verify-owner-soc-production-readiness.mjs';

const signature='a'.repeat(86);
const reference='unified-verdict-ledger:11111111-2222-4333-8444-555555555555:'+signature;
const readiness={ok:true,readiness:{
  schema_version:'koschei.soc-runtime-readiness.v1',sampled_cases:0,
  signed_verdict_receipts:{
    state:'bounded_historical_signatures_verified_not_end_to_end_accepted',
    ledger_table_readable:true,independent_trust_loaded:true,
    sampled_receipts:20,verified_receipts:20
  }
}};
const candidates={ok:true,schema_version:'koschei.owner-soc-signed-candidates.v1',
  limit:20,count:1,items:[{
    network:'solana-mainnet',target:'test-token',verdict:'withhold',
    evidence_ref:reference,linked_to_soc:false
  }]
};
function mock({r=readiness,c=candidates,status=200,contentType='application/json',big=false}={}){
  const calls=[];
  return {calls,fetchImpl:async(url,options)=>{
    calls.push({url,options});
    if(status!==200)return new Response('unauthorized',{status,headers:{'Content-Type':contentType}});
    const value=url.endsWith('/readiness')?r:c;
    return new Response(big?'z'.repeat(270000):JSON.stringify(value),{
      status,headers:{'Content-Type':contentType}
    });
  }};
}
const baseURL='https://example.com',ownerSecret='hidden-operator-secret';
test('GET only, bounded signature evidence, never end-to-end acceptance',async()=>{
  const m=mock(),result=await observeOwnerSOC({baseURL,ownerSecret,fetchImpl:m.fetchImpl});
  assert.equal(result.state,'bounded_historical_read_verified_not_end_to_end_accepted');
  assert.equal(result.production_e2e,'not_accepted');
  assert.equal(result.historical_signatures,'bounded_verified');
  assert.equal(result.linked_token_candidates,0);
  assert.deepEqual(result.blockers,['no_linked_recent_token_candidates']);
  assert.equal(m.calls.length,2);
  for(const call of m.calls){
    assert.equal(call.options.method,'GET');
    assert.equal(call.options.redirect,'error');
    assert.equal(call.options.headers['x-koschei-secret'],ownerSecret);
  }
  assert(!JSON.stringify(result).includes(ownerSecret));
  assert(!JSON.stringify(result).includes(reference));
});
test('a linked candidate is still not full production acceptance',async()=>{
  const m=mock({c:{...candidates,items:[{...candidates.items[0],linked_to_soc:true}]}});
  const result=await observeOwnerSOC({baseURL,ownerSecret,fetchImpl:m.fetchImpl});
  assert.equal(result.linked_token_candidates,1);
  assert.equal(result.production_e2e,'not_accepted');
});
test('unknown historical public-key trust fails closed',async()=>{
  const m=mock({r:{...readiness,readiness:{...readiness.readiness,
    signed_verdict_receipts:{...readiness.readiness.signed_verdict_receipts,independent_trust_loaded:false}
  }}});
  const result=await observeOwnerSOC({baseURL,ownerSecret,fetchImpl:m.fetchImpl});
  assert.equal(result.state,'not_accepted');
  assert.equal(result.historical_signatures,'unavailable');
});
test('forged receipt reference is rejected without leaking subject',async()=>{
  const m=mock({c:{...candidates,items:[{...candidates.items[0],evidence_ref:'wrong-ref'}]}});
  const result=await observeOwnerSOC({baseURL,ownerSecret,fetchImpl:m.fetchImpl});
  assert(result.blockers.includes('owner_candidates_contract_invalid'));
  assert(!JSON.stringify(result).includes('wrong-ref'));
});
test('unauthorized readiness never becomes an empty healthy list',async()=>{
  const m=mock({status:401});
  const result=await observeOwnerSOC({baseURL,ownerSecret,fetchImpl:m.fetchImpl});
  assert.equal(result.state,'not_accepted');
  assert.deepEqual(result.blockers,['owner_readiness_http_401']);
  assert.equal(m.calls.length,1);
});
test('oversized and wrong content type responses fail closed',async()=>{
  const huge=mock({big:true});
  const first=await observeOwnerSOC({baseURL,ownerSecret,fetchImpl:huge.fetchImpl});
  assert.equal(first.state,'not_accepted');
  assert(first.blockers.includes('owner_response_exceeds_size_limit'));
  const html=mock({contentType:'text/html'});
  const second=await observeOwnerSOC({baseURL,ownerSecret,fetchImpl:html.fetchImpl});
  assert.deepEqual(second.blockers,['owner_response_content_type_invalid']);
});
test('unsafe origins and missing credentials never send requests',async()=>{
  const m=mock();
  for(const bad of ['http://remote.example','https://example.com/path',
                    'https://example.com?secret=abc','https://person:password@example.com']){
    await assert.rejects(observeOwnerSOC({baseURL:bad,ownerSecret,fetchImpl:m.fetchImpl}));
  }
  await assert.rejects(observeOwnerSOC({baseURL,ownerSecret:'',fetchImpl:m.fetchImpl}));
  assert.equal(m.calls.length,0);
});
test('empty token sample is explicitly unaccepted',async()=>{
  const m=mock({c:{...candidates,count:0,items:[]}});
  const result=await observeOwnerSOC({baseURL,ownerSecret,fetchImpl:m.fetchImpl});
  assert.equal(result.candidate_sample,'no_recent_token_candidates');
  assert.equal(result.production_e2e,'not_accepted');
});
test('transport exceptions cannot disclose credentials',async()=>{
  const result=await observeOwnerSOC({baseURL,ownerSecret,
    fetchImpl:async()=>{throw new Error('transport leaked '+ownerSecret);}
  });
  assert.deepEqual(result.blockers,['owner_response_transport_unavailable']);
  assert(!JSON.stringify(result).includes(ownerSecret));
});
