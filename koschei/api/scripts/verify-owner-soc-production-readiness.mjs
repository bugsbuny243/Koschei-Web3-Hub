#!/usr/bin/env node
// Owner-only, GET-only production observation. This cannot prove fresh customer
// billing, a case write/read-back, ledger-wide integrity, or external WORM.
const readinessPath='/api/owner/soc/readiness';
const candidatesPath='/api/owner/soc/verdict-candidates';
const verifiedState='bounded_historical_signatures_verified_not_end_to_end_accepted';
const candidateSchema='koschei.owner-soc-signed-candidates.v1';
const receiptRef=/^unified-verdict-ledger:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}:[A-Za-z0-9_-]{86}$/;

function validOrigin(raw) {
  const url=new URL(raw);
  const local=['127.0.0.1','localhost','[::1]'].includes(url.hostname);
  if ((url.protocol!=='https:' && !(local && url.protocol==='http:')) ||
      url.username || url.password || url.pathname!=='/' || url.search || url.hash) {
    throw new Error('KOSCHEI_BASE_URL must be a clean HTTPS origin (loopback HTTP permitted for tests)');
  }
  return url.origin;
}
async function boundedJSON(fetchImpl,base,path,secret) {
  let response;
  try {
    response=await fetchImpl(base+path,{
      method:'GET',redirect:'error',cache:'no-store',
      headers:{'Accept':'application/json','Cache-Control':'no-store','x-koschei-secret':secret},
      signal:AbortSignal.timeout(8000)
    });
  } catch {throw new Error('owner_response_transport_unavailable');}
  if(response.status!==200)throw new Error('owner_'+(path===readinessPath?'readiness':'candidates')+'_http_'+response.status);
  if(!String(response.headers.get('content-type')||'').toLowerCase().includes('application/json')) {
    throw new Error('owner_response_content_type_invalid');
  }
  if(!response.body)throw new Error('owner_response_body_missing');
  const bytes=[];let total=0;
  try {
    for await (const chunk of response.body) {
      total+=chunk.byteLength;
      if(total>256*1024)throw new Error('owner_response_exceeds_size_limit');
      bytes.push(chunk);
    }
  } catch(err) {
    if(err?.message==='owner_response_exceeds_size_limit')throw err;
    throw new Error('owner_response_stream_unavailable');
  }
  try{return JSON.parse(Buffer.concat(bytes.map(chunk=>Buffer.from(chunk))).toString('utf8'));}
  catch{throw new Error('owner_response_json_invalid');}
}
const safeErrors=new Set([
 'owner_response_transport_unavailable','owner_response_stream_unavailable',
 'owner_response_exceeds_size_limit','owner_response_body_missing',
 'owner_response_content_type_invalid','owner_response_json_invalid'
]);
function safeError(err,phase){
  const code=String(err?.message||'');
  if(safeErrors.has(code)||/^owner_(readiness|candidates)_http_\d{3}$/.test(code))return code;
  return 'owner_'+phase+'_unavailable';
}
function boundedCount(n,max){return Number.isInteger(n)&&n>=0&&n<=max;}

export async function observeOwnerSOC({baseURL,ownerSecret,fetchImpl=fetch}={}) {
  if(!ownerSecret||!String(ownerSecret).trim())throw new Error('KOSCHEI_OWNER_SECRET is required');
  const base=validOrigin(baseURL||'https://tradepigloball.co');
  const report={
    schema_version:'koschei.owner-soc-production-read-observation.v1',
    state:'not_accepted',historical_signatures:'unavailable',production_e2e:'not_accepted',
    candidate_sample:'unavailable',sampled_receipts:0,verified_receipts:0,
    sampled_token_candidates:0,linked_token_candidates:0,sampled_soc_cases:0,
    blockers:[],scope:'owner_only_read_only_latest_20_receipts_and_token_candidates',
    boundary:'A bounded historical read never proves a fresh customer scan, customer debit, SOC case write/read-back, all-ledger coverage, or external WORM retention'
  };
  let readiness;
  try{readiness=await boundedJSON(fetchImpl,base,readinessPath,ownerSecret);}
  catch(err){report.blockers.push(safeError(err,'readiness'));return report;}
  const data=readiness?.readiness,receipt=data?.signed_verdict_receipts;
  if(readiness?.ok!==true||data?.schema_version!=='koschei.soc-runtime-readiness.v1'||!receipt||
      !boundedCount(receipt.sampled_receipts,20)||!boundedCount(receipt.verified_receipts,20)||
      !boundedCount(data.sampled_cases,50)){
    report.blockers.push('owner_readiness_contract_invalid');return report;
  }
  report.sampled_soc_cases=data.sampled_cases;
  report.sampled_receipts=receipt.sampled_receipts;
  report.verified_receipts=receipt.verified_receipts;
  if(receipt.state==='bounded_historical_signatures_verified_not_end_to_end_accepted' &&
     receipt.ledger_table_readable===true && receipt.independent_trust_loaded===true &&
     receipt.sampled_receipts>0 && receipt.sampled_receipts===receipt.verified_receipts){
    report.historical_signatures='bounded_verified';
  }else{
    report.blockers.push('historical_signature_readiness_not_verified');
  }
  let candidates;
  try{candidates=await boundedJSON(fetchImpl,base,candidatesPath,ownerSecret);}
  catch(err){report.blockers.push(safeError(err,'candidates'));return report;}
  if(candidates?.ok!==true||candidates.schema_version!==candidateSchema||
     candidates.limit!==20||!Array.isArray(candidates.items)||
     !boundedCount(candidates.count,20)||candidates.count!==candidates.items.length||
     !candidates.items.every(item=>item&&receiptRef.test(item.evidence_ref)&&
        typeof item.network==='string'&&item.network.length>0&&
        typeof item.target==='string'&&item.target.length>0&&
        typeof item.verdict==='string'&&item.verdict.length>0&&
        typeof item.linked_to_soc==='boolean')){
    report.blockers.push('owner_candidates_contract_invalid');return report;
  }
  report.sampled_token_candidates=candidates.count;
  report.linked_token_candidates=candidates.items.filter(item=>item.linked_to_soc).length;
  report.candidate_sample=candidates.count>0?'bounded_verified':'no_recent_token_candidates';
  if(!candidates.count)report.blockers.push('no_recent_token_candidates');
  if(!report.linked_token_candidates)report.blockers.push('no_linked_recent_token_candidates');
  if(report.historical_signatures==='bounded_verified'&&report.candidate_sample==='bounded_verified'){
    report.state='bounded_historical_read_verified_not_end_to_end_accepted';
  }
  return report;
}
if(process.argv[1]&&import.meta.url===new URL('file://'+process.argv[1]).href){
  try{
    const report=await observeOwnerSOC({
      baseURL:process.env.KOSCHEI_BASE_URL,
      ownerSecret:process.env.KOSCHEI_OWNER_SECRET
    });
    process.stdout.write(JSON.stringify(report)+'\n');
    if(report.state!=='bounded_historical_read_verified_not_end_to_end_accepted')process.exitCode=1;
  }catch{
    // Never echo transport exceptions: they may contain credentials.
    process.stderr.write('Owner SOC read-only observation unavailable: invalid configuration or transport\n');
    process.exitCode=1;
  }
}
