/* Shared address routing. Format detection is a hint; the server validates evidence. */
(()=>{
'use strict';
const networks=Object.freeze([
  ['solana-mainnet','Solana','solana'],['ethereum-mainnet','Ethereum','evm'],
  ['base-mainnet','Base','evm'],['arbitrum-mainnet','Arbitrum','evm'],
  ['optimism-mainnet','Optimism','evm'],['polygon-mainnet','Polygon','evm'],
  ['bnb-mainnet','BNB Smart Chain','evm'],['avalanche-mainnet','Avalanche C-Chain','evm'],
  ['bitcoin-mainnet','Bitcoin','bitcoin']
]);
function classify(value){
  const target=String(value||'').trim();
  if(/^0x[0-9a-fA-F]{64}$/.test(target))return 'evm_transaction';
  if(/^0x[0-9a-fA-F]{40}$/.test(target))return 'evm';
  // Legacy Bitcoin addresses overlap Solana's alphabet. Never infer a chain
  // from their prefix alone; an explicit network disambiguates them.
  if(/^(?:bc1|BC1)[a-zA-Z0-9]{20,87}$/.test(target))return 'bitcoin';
  if(/^[13][1-9A-HJ-NP-Za-km-z]{25,34}$/.test(target))return 'ambiguous';
  if(/^[1-9A-HJ-NP-Za-km-z]{32,44}$/.test(target))return 'solana';
  return 'unknown';
}
function resolve(value,network=''){
  const target=String(value||'').trim(),family=classify(target);
  if(!target)return {error:'Paste a public wallet or contract address.'};
  if(target.length>256)return {error:'Enter a complete wallet, contract address, or transaction hash.'};
  if(network==='bitcoin-mainnet'&&/^[0-9a-fA-F]{64}$/.test(target))return {target:target.toLowerCase(),network,label:'Bitcoin',family:'bitcoin',kind:'transaction'};
  if(family==='unknown')return {error:'Enter a complete wallet, contract address, or transaction hash.'};
  const selected=networks.find(item=>item[0]===network);
  if(network&&!selected)return {error:'Select a supported network.'};
  if(!selected&&(family==='evm'||family==='evm_transaction'||family==='ambiguous'))return {error:'Select the network for this target. Its format alone does not identify the chain.',needsNetwork:true};
  const resolved=selected||networks.find(item=>item[2]===family);
  if(family==='evm_transaction'){
    if(resolved[2]!=='evm')return {error:'EVM transaction hashes require a supported EVM network.'};
    return {target,network:resolved[0],label:resolved[1],family:'evm',kind:'transaction'};
  }
  if(family==='ambiguous'?!['solana','bitcoin'].includes(resolved[2]):resolved[2]!==family)return {error:'The address format does not match the selected network.'};
  return {target,network:resolved[0],label:resolved[1],family:resolved[2],kind:'address'};
}
function url(value,network){
  const request=resolve(value,network);
  if(request.error)return request;
  return {...request,url:'/scan?'+new URLSearchParams({mode:request.kind==='transaction'?'lookup':'address',target:request.target,network:request.network}).toString()};
}
function isAddressView(search=location.search,pathname=location.pathname){
  const params=new URLSearchParams(search);
  if(params.get('mode')==='address'||params.get('mode')==='lookup')return true;
  return !params.has('mode')&&!params.has('mint')&&!pathname.startsWith('/scan/');
}
function matchesResult(data,request){
  const result=data?.result,target=result?.target;
  return data?.schema_version==='koschei-customer-scan-v1'&&
    target?.raw===request.target&&target?.network_hint===request.network&&
    typeof result.trust==='object'&&result.trust!==null&&
    typeof result.status==='string'&&Array.isArray(result.evidence_refs||[]);
}
window.KoscheiScanEntry=Object.freeze({networks,classify,resolve,url,isAddressView,matchesResult});
})();
