(()=>{
'use strict';
if(window.__koscheiUniversalAddressScanV1)return;
window.__koscheiUniversalAddressScanV1=true;

const esc=value=>String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
const yesNo=value=>value===true?'YES':value===false?'NO':'UNKNOWN';
const short=value=>String(value??'')||'—';
const errorCopy={401:'Sign in to continue this analysis.',402:'Professional access is required for this request.',403:'Your account cannot access this request.',429:'Too many requests. Wait a moment, then try again.',503:'This evidence source is currently unavailable. Try again later.'};

function authorityRows(authority){
  if(!authority)return'<div class="cus-empty">No authority snapshot was attached for this network.</div>';
  const rows=[
    ['Contract code',authority.contract_code_state],
    ['Code hash',authority.contract_code_sha256],
    ['EIP-7702',authority.delegation_state],
    ['Delegate target',authority.delegation_target],
    ['ERC-1967',authority.proxy_state],
    ['Implementation',authority.proxy_implementation],
    ['Admin',authority.proxy_admin],
    ['Beacon',authority.proxy_beacon],
    ['Conflicting slots',authority.conflicting_slots===true?'YES':authority.conflicting_slots===false?'NO':'—']
  ];
  return rows.filter(([,value])=>value!==undefined&&value!==null&&value!=='').map(([key,value])=>`<div class="cus-row"><span>${esc(key)}</span><b title="${esc(value)}">${esc(short(value))}</b></div>`).join('');
}

function renderTrust(trust={}){
  const axes=[['Claimed',trust.claimed],['Observed',trust.observed],['Authorized',trust.authorized],['Available',trust.available],['Verified',trust.verified],['Finalized',trust.finalized]];
  return `<div class="cus-trust">${axes.map(([name,value])=>`<div data-on="${value===true?'1':'0'}"><span>${esc(name)}</span><b>${yesNo(value)}</b></div>`).join('')}</div>`;
}

function customerSummary(result){
  if(!result)return'Evidence unavailable';
  if(result.evm_authority){
    const a=result.evm_authority;
    if(a.delegation_state==='delegation_observed')return'EIP-7702 delegation observed';
    if(a.proxy_state==='erc1967_authority_observed')return'Upgradeable authority observed';
    if(a.contract_code_state==='contract_code_observed')return'Contract code observed';
  }
  if(result.trust?.observed===true)return'On-chain state observed';
  return String(result.status||'Evidence returned').replaceAll('_',' ');
}

function verdictSummary(classification,entries){
  const live=entries.filter(x=>x.ok&&x.data?.result);
  if(!live.length){
    return{tone:'limited',label:'LIMITED EVIDENCE',title:'No live evidence source returned a result.',copy:'Koschei could not complete a live observation for this address. Missing evidence is not treated as a safety signal.'};
  }
  const results=live.map(x=>x.data.result);
  const authorities=results.map(x=>x.evm_authority).filter(Boolean);
  const delegated=authorities.some(a=>a.delegation_state==='delegation_observed');
  const upgradeable=authorities.some(a=>a.proxy_state==='erc1967_authority_observed');
  const conflicting=authorities.some(a=>a.conflicting_slots===true);
  if(conflicting){
    return{tone:'review',label:'REVIEW AUTHORITY',title:'Conflicting proxy authority slots were observed.',copy:'The address has a control surface that deserves review. This observation does not prove malicious intent.'};
  }
  if(delegated){
    return{tone:'review',label:'REVIEW AUTHORITY',title:'EIP-7702 delegation was observed.',copy:'The address delegates execution authority. Review the delegate target and attached evidence before relying on the address.'};
  }
  if(upgradeable){
    return{tone:'review',label:'REVIEW AUTHORITY',title:'Upgradeable authority was observed.',copy:'ERC-1967 authority is present on at least one observed network. Upgradeability is a control surface, not an automatic malicious verdict.'};
  }
  const observed=results.some(x=>x.trust?.observed===true);
  if(observed){
    return{tone:'observed',label:'OBSERVED',title:'Live on-chain state was observed.',copy:classification.family==='evm'?'No supported authority signal was attached to the returned observations. This is not proof that the address is immutable or safe.':'Koschei returned live read-only evidence. Review the evidence details before making a security decision.'};
  }
  return{tone:'limited',label:'LIMITED EVIDENCE',title:'Evidence returned without a completed observation.',copy:'Koschei did not promote incomplete evidence into a safety claim.'};
}

function resultCard(entry){
  const result=entry.data?.result;
  if(!entry.ok||!result){
    const message=errorCopy[entry.http]||entry.data?.error||'The evidence request could not be completed. Try again.';
    return `<article class="cus-network-card is-unavailable"><div class="cus-card-head"><div><span>${esc(entry.label)}</span><b>Evidence unavailable</b></div><em>${esc(String(entry.http||'OFFLINE'))}</em></div><p>${esc(message)}</p></article>`;
  }
  const authority=result.evm_authority;
  const reasons=Array.isArray(result.reasons)?result.reasons:[];
  return `<article class="cus-network-card">
    <div class="cus-card-head"><div><span>${esc(entry.label)}</span><b>${esc(customerSummary(result))}</b></div><em>${esc(String(result.evidence_status||'unknown').toUpperCase())}</em></div>
    <div class="cus-target">${esc(result.target?.raw||'')}</div>
    <div class="actions"><button type="button" class="btn" data-cus-share-x>Share result on X</button></div>
    <details class="cus-details"><summary>Technical evidence</summary>
      ${renderTrust(result.trust||{})}
      ${authority?`<div class="cus-authority"><div class="cus-subhead"><span>Authority surface</span><b>Observed evidence</b></div>${authorityRows(authority)}</div>`:''}
      <div class="cus-reasons"><div class="cus-subhead"><span>Reason codes</span><b>${reasons.length}</b></div>${reasons.length?reasons.map(reason=>`<code>${esc(reason)}</code>`).join(''):'<span class="cus-empty">No reason code attached.</span>'}</div>
      <div class="cus-reasons"><div class="cus-subhead">Evidence references</div>${(result.evidence_refs||[]).map(ref=>`<code>${esc(ref)}</code>`).join('')||'<span class="cus-empty">No reference was attached.</span>'}</div>
      <details class="cus-details"><summary>Complete response</summary><pre class="cus-raw">${esc(JSON.stringify(entry.data,null,2))}</pre></details>
    </details>
  </article>`;
}

function summaryBlock(classification,entries){
  const live=entries.filter(x=>x.ok&&x.data?.result);
  const unavailable=entries.length-live.length;
  const authorityCount=live.filter(x=>Boolean(x.data?.result?.evm_authority)).length;
  const observed=live.filter(x=>x.data?.result?.trust?.observed===true).length;
  const verdict=verdictSummary(classification,entries);
  return `<section class="cus-customer-verdict" data-tone="${esc(verdict.tone)}">
      <span>${esc(verdict.label)}</span><h3>${esc(verdict.title)}</h3><p>${esc(verdict.copy)}</p>
    </section>
    <section class="cus-overview">
      <div><span>Detected</span><b>${esc(classification.label)}</b></div>
      <div><span>Evidence returned</span><b>${live.length}</b></div>
      <div><span>Observed networks</span><b>${observed}</b></div>
      <div><span>Unavailable</span><b>${unavailable}</b></div>
      ${classification.family==='evm'?`<div><span>Authority snapshots</span><b>${authorityCount}</b></div>`:''}
    </section>`;
}

function sharePayload(request,result){
  const share=window.KoscheiInvestigationShare;
  return{
    target:request.target,
    kind:'address',
    network:request.network,
    networkLabel:request.label,
    evidence_status:result.evidence_status,
    status:result.status,
    verdict:result.verdict,
    reasons:Array.isArray(result.reasons)?result.reasons.slice():[],
    url:share?.publicResultURL?share.publicResultURL(request.target,'address',request.network):undefined
  };
}

function install(){
  const section=document.getElementById('customerUniversalScan'),routing=window.KoscheiScanEntry;
  if(!section||!routing)return;
  const $=id=>document.getElementById(id);
  const form=$('customerUniversalScanForm'),input=$('customerUniversalTarget'),network=$('customerUniversalNetwork');
  const submit=$('customerUniversalSubmit'),cancel=$('customerUniversalCancel'),status=$('customerUniversalStatus');
  const wrap=$('customerUniversalResultsWrap'),overview=$('customerUniversalOverview'),results=$('customerUniversalResults');
  const recovery=$('customerUniversalRecovery'),advanced=$('advancedTools');
  let pending=null,generation=0,lastSharePayload=null;

  function stop(){
    generation++;
    pending?.abort();pending=null;
    form.removeAttribute('aria-busy');submit.disabled=false;submit.textContent='Analyze address';cancel.hidden=true;
  }
  function invalidate(){
    stop();lastSharePayload=null;wrap.hidden=true;recovery.hidden=true;
    status.textContent='Ready to analyze the current address and network.';
    input.removeAttribute('aria-invalid');
  }
  input.addEventListener('input',invalidate);network.addEventListener('change',invalidate);
  cancel.addEventListener('click',()=>{stop();lastSharePayload=null;wrap.hidden=true;status.textContent='Analysis canceled. You can start another request.';});
  results.addEventListener('click',event=>{
    const button=event.target?.closest?.('[data-cus-share-x]');
    if(!button||!lastSharePayload)return;
    window.KoscheiInvestigationShare?.open?.(lastSharePayload);
  });
  window.addEventListener('pagehide',stop);

  async function runScan(){
    stop();lastSharePayload=null;wrap.hidden=true;recovery.hidden=true;
    const request=routing.resolve(input.value,network.value);
    if(request.error){
      status.textContent=request.error;input.setAttribute('aria-invalid','true');
      (request.needsNetwork?network:input).focus();return;
    }
    input.removeAttribute('aria-invalid');network.value=request.network;
    const current=++generation,controller=new AbortController();pending=controller;
    const timer=setTimeout(()=>controller.abort(),15000);
    submit.disabled=true;submit.textContent='Analyzing…';cancel.hidden=false;form.setAttribute('aria-busy','true');
    status.textContent=`Collecting read-only evidence on ${request.label}…`;
    history.replaceState({},'',routing.url(request.target,request.network).url);
    try{
      const response=await fetch('/api/scan',{method:'POST',headers:{'Content-Type':'application/json','Accept':'application/json'},body:JSON.stringify({target:request.target,network:request.network}),cache:'no-store',credentials:'same-origin',signal:controller.signal});
      const data=await response.json();
      if(current!==generation)return;
      if(response.ok&&!routing.matchesResult(data,request))throw new Error('The returned evidence does not match this address and network. The result was withheld.');
      const entry={network:request.network,label:request.label,http:response.status,ok:response.ok,data};
      overview.innerHTML=summaryBlock({family:request.family,label:request.label+' address'},[entry]);
      results.innerHTML=resultCard(entry);wrap.hidden=false;
      if(response.ok&&data?.result)lastSharePayload=sharePayload(request,data.result);
      status.textContent=response.ok?'Analysis complete. Review the findings, limits and technical evidence below.':(errorCopy[response.status]||'Analysis could not complete. Review the source error below and try again.');
      if([401,402,403].includes(response.status)){
        const link=document.createElement('a');
        link.href=response.status===401?'/login?next='+encodeURIComponent(routing.url(request.target,request.network).url):'/account';
        link.textContent=response.status===401?'Sign in to continue':'Review account access';
        recovery.replaceChildren(link);recovery.hidden=false;
      }
      if(response.ok&&request.family==='solana'){
        const link=document.createElement('a');link.href='/arvis-chat?'+new URLSearchParams({target:request.target,network:request.network});
        link.textContent='Continue with ARVIS investigation';recovery.replaceChildren(link);recovery.hidden=false;
      }
    }catch(error){
      if(current!==generation)return;
      lastSharePayload=null;
      status.textContent=controller.signal.aborted?'The evidence service did not respond within 15 seconds. Try again.':(error instanceof SyntaxError?'The service returned an unreadable response. Try again.':error.message||'Connection failed. Try again.');
      wrap.hidden=true;
    }finally{
      clearTimeout(timer);
      if(current===generation){pending=null;submit.disabled=false;submit.textContent='Analyze address';cancel.hidden=true;form.removeAttribute('aria-busy');}
    }
  }
  form.addEventListener('submit',event=>{event.preventDefault();runScan();});
  const params=new URLSearchParams(location.search),addressView=routing.isAddressView();
  if(advanced){
    advanced.open=!addressView;
    document.querySelectorAll('[data-scan-mode]').forEach(button=>button.addEventListener('click',()=>{stop();lastSharePayload=null;advanced.open=true;}));
  }
  const initial=params.get('target');
  if(addressView&&initial){
    input.value=initial;network.value=params.get('network')||'';
    if(params.get('network')&&!routing.networks.some(item=>item[0]===params.get('network'))){status.textContent='The link contains an unsupported network. Select a network to continue.';return;}
    runScan();
  }
}

document.readyState==='loading'?document.addEventListener('DOMContentLoaded',install,{once:true}):install();
})();
