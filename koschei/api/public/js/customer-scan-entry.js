/* Shared address routing. Format detection is a hint; the server validates evidence. */
(()=>{
'use strict';
const networks=Object.freeze([
  ['solana-mainnet','Solana','solana'],['ethereum-mainnet','Ethereum','evm'],
  ['base-mainnet','Base','evm'],['arbitrum-mainnet','Arbitrum','evm'],
  ['optimism-mainnet','Optimism','evm'],['bitcoin-mainnet','Bitcoin','bitcoin']
]);
function classify(value){
  const target=String(value||'').trim();
  if(/^0x[0-9a-fA-F]{40}$/.test(target))return 'evm';
  if(/^(?:bc1|BC1)[a-zA-Z0-9]{20,87}$/.test(target))return 'bitcoin';
  if(/^[13][1-9A-HJ-NP-Za-km-z]{25,34}$/.test(target))return 'ambiguous';
  if(/^[1-9A-HJ-NP-Za-km-z]{32,44}$/.test(target))return 'solana';
  return 'unknown';
}
function resolve(value,network=''){
  const target=String(value||'').trim(),family=classify(target);
  if(!target)return {error:'Paste a public wallet or contract address.'};
  if(target.length>256||family==='unknown')return {error:'Enter a complete wallet or contract address. Use the dedicated tools below for token investigations or transaction preflight.'};
  const selected=networks.find(item=>item[0]===network);
  if(network&&!selected)return {error:'Select a supported network.'};
  if(!selected&&(family==='evm'||family==='ambiguous'))return {error:'Select the network for this address. Its format alone does not identify the chain.',needsNetwork:true};
  const resolved=selected||networks.find(item=>item[2]===family);
  if(family==='ambiguous'?!['solana','bitcoin'].includes(resolved[2]):resolved[2]!==family)return {error:'The address format does not match the selected network.'};
  return {target,network:resolved[0],label:resolved[1],family:resolved[2]};
}
function url(value,network){
  const request=resolve(value,network);
  if(request.error)return request;
  return {...request,url:'/scan?'+new URLSearchParams({mode:'address',target:request.target,network:request.network}).toString()};
}
function isAddressView(search=location.search,pathname=location.pathname){
  const params=new URLSearchParams(search);
  if(params.get('mode')==='address')return true;
  return !params.has('mode')&&!params.has('mint')&&!pathname.startsWith('/scan/');
}
function evidenceStatusForTrust(trust){
  if(!trust||typeof trust!=='object')return null;
  for(const key of ['claimed','observed','authorized','available','verified','finalized'])if(typeof trust[key]!=='boolean')return null;
  if(trust.verified&&!trust.observed)return null;
  if(trust.finalized&&!trust.verified)return null;
  if(trust.finalized&&trust.verified&&trust.observed)return 'finalized';
  if(trust.verified&&trust.observed)return 'verified';
  if(trust.observed)return 'observed';
  if(trust.claimed)return 'claimed';
  return 'unverified';
}
function authorityMatchesRequest(authority,request){
  if(authority===undefined||authority===null)return true;
  if(request.family!=='evm'||typeof authority!=='object')return false;
  if(String(authority.network||'').trim()!==request.network)return false;
  if(String(authority.spender||'').trim().toLowerCase()!==request.target.toLowerCase())return false;
  const trust=authority.trust;
  if(evidenceStatusForTrust(trust)!=='observed')return false;
  if(trust.authorized||trust.verified||trust.finalized)return false;
  return true;
}
function partialEvidenceMatches(result,request){
  const reasons=Array.isArray(result?.reasons)?result.reasons:[];
  const authorityUnavailable=reasons.includes('EVM_AUTHORITY_PROBE_UNAVAILABLE');
  const partialMarker=reasons.includes('PARTIAL_EVIDENCE_EVM_AUTHORITY_UNAVAILABLE');
  if(!authorityUnavailable&&!partialMarker)return true;
  if(request.family!=='evm'||result.evm_authority!==undefined&&result.evm_authority!==null)return false;
  return authorityUnavailable&&partialMarker;
}
function matchesResult(data,request){
  const result=data?.result,target=result?.target,trust=result?.trust;
  if(data?.schema_version!=='koschei-customer-scan-v1'||target?.raw!==request.target||target?.network_hint!==request.network)return false;
  const evidenceStatus=evidenceStatusForTrust(trust);
  if(!evidenceStatus||result?.evidence_status!==evidenceStatus)return false;
  if(!['needs_context','insufficient_evidence','observed','evidence_ready'].includes(result?.status))return false;
  if(!['unknown','review'].includes(result?.verdict))return false;
  const refs=result?.evidence_refs;
  if(refs!==undefined&&!Array.isArray(refs))return false;
  if((result.status==='observed'||result.status==='evidence_ready')&&(!Array.isArray(refs)||refs.length===0))return false;
  if(result.status==='observed'&&(!trust.observed||trust.verified||result.verdict!=='review'))return false;
  if(result.status==='evidence_ready'&&(!trust.verified||!trust.observed||result.verdict!=='review'))return false;
  if((result.status==='needs_context'||result.status==='insufficient_evidence')&&(trust.observed||result.verdict!=='unknown'))return false;
  if(!authorityMatchesRequest(result.evm_authority,request))return false;
  if(!partialEvidenceMatches(result,request))return false;
  return true;
}
window.KoscheiScanEntry=Object.freeze({networks,classify,resolve,url,isAddressView,matchesResult});
})();
