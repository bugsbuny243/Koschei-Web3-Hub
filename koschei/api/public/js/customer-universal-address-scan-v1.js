(()=>{
'use strict';
if(window.__koscheiUniversalAddressScanV1)return;
window.__koscheiUniversalAddressScanV1=true;

const EVM_NETWORKS=[
  ['ethereum-mainnet','Ethereum'],
  ['base-mainnet','Base'],
  ['arbitrum-mainnet','Arbitrum'],
  ['optimism-mainnet','Optimism']
];
const esc=value=>String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
const yesNo=value=>value===true?'YES':value===false?'NO':'—';
const short=value=>{const s=String(value??'');return s.length>34?`${s.slice(0,15)}…${s.slice(-11)}`:s||'—'};

function classify(value){
  const v=value.trim();
  if(/^0x[0-9a-fA-F]{40}$/.test(v))return{family:'evm',label:'EVM address',networks:EVM_NETWORKS};
  if(/^(bc1|[13])[a-zA-HJ-NP-Z0-9]{20,90}$/i.test(v))return{family:'bitcoin',label:'Bitcoin address',networks:[['bitcoin-mainnet','Bitcoin']]};
  if(/^[1-9A-HJ-NP-Za-km-z]{32,44}$/.test(v))return{family:'solana',label:'Solana address',networks:[['solana-mainnet','Solana']]};
  return{family:'unknown',label:'Unknown address',networks:[]};
}

async function scanNetwork(target,network,label){
  try{
    const response=await fetch('/api/scan',{method:'POST',headers:{'Content-Type':'application/json','Accept':'application/json'},body:JSON.stringify({target,network}),cache:'no-store',credentials:'same-origin'});
    const data=await response.json().catch(()=>({}));
    return{network,label,http:response.status,ok:response.ok,data};
  }catch(error){
    return{network,label,http:0,ok:false,data:{error:error?.message||'network_error'}};
  }
}

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
    const message=entry.data?.error||`HTTP ${entry.http||'unavailable'}`;
    return `<article class="cus-network-card is-unavailable"><div class="cus-card-head"><div><span>${esc(entry.label)}</span><b>Evidence unavailable</b></div><em>${esc(String(entry.http||'OFFLINE'))}</em></div><p>${esc(message)}</p></article>`;
  }
  const authority=result.evm_authority;
  const reasons=Array.isArray(result.reasons)?result.reasons:[];
  return `<article class="cus-network-card">
    <div class="cus-card-head"><div><span>${esc(entry.label)}</span><b>${esc(customerSummary(result))}</b></div><em>${esc(String(result.evidence_status||'unknown').toUpperCase())}</em></div>
    <div class="cus-target">${esc(result.target?.raw||'')}</div>
    <details class="cus-details"><summary>Technical evidence</summary>
      ${renderTrust(result.trust||{})}
      ${authority?`<div class="cus-authority"><div class="cus-subhead"><span>Authority surface</span><b>Observed evidence</b></div>${authorityRows(authority)}</div>`:''}
      <div class="cus-reasons"><div class="cus-subhead"><span>Reason codes</span><b>${reasons.length}</b></div>${reasons.length?reasons.slice(0,12).map(reason=>`<code>${esc(reason)}</code>`).join(''):'<span class="cus-empty">No reason code attached.</span>'}</div>
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

function install(){
  if(!location.pathname.startsWith('/scan'))return;
  const anchor=document.querySelector('.universe-gateway');
  if(!anchor||document.getElementById('customerUniversalScan'))return;
  const legacyHero=document.querySelector('.hero');
  const legacyEVM=document.querySelector('.evm-desk');
  if(legacyHero)legacyHero.hidden=true;
  if(legacyEVM)legacyEVM.hidden=true;

  const section=document.createElement('section');
  section.id='customerUniversalScan';
  section.className='customer-universal-scan';
  section.innerHTML=`
    <div class="cus-copy">
      <span class="eyebrow">ADDRESS SCAN</span>
      <h2>Paste an address.<br>Koschei handles the rest.</h2>
      <p>Paste a wallet or contract address. Koschei identifies the address family and checks the supported evidence sources automatically.</p>
    </div>
    <form class="cus-form" id="customerUniversalScanForm">
      <label for="customerUniversalTarget">Wallet or contract address</label>
      <div class="cus-input-wrap"><input id="customerUniversalTarget" autocomplete="off" spellcheck="false" placeholder="Paste an address"><button type="submit" class="primary" id="customerUniversalSubmit">Scan address</button></div>
      <div class="cus-hint"><span>EVM</span><span>Solana</span><span>Bitcoin</span></div>
    </form>
    <div class="cus-status" id="customerUniversalStatus">Ready for an address.</div>
    <div class="cus-results-wrap" id="customerUniversalResultsWrap" hidden><div id="customerUniversalOverview"></div><div class="cus-results" id="customerUniversalResults"></div></div>
    <button class="cus-advanced" id="customerAdvancedToggle" type="button" aria-expanded="false">Advanced tools</button>`;
  anchor.insertAdjacentElement('afterend',section);

  const form=section.querySelector('#customerUniversalScanForm');
  const input=section.querySelector('#customerUniversalTarget');
  const submit=section.querySelector('#customerUniversalSubmit');
  const status=section.querySelector('#customerUniversalStatus');
  const wrap=section.querySelector('#customerUniversalResultsWrap');
  const overview=section.querySelector('#customerUniversalOverview');
  const results=section.querySelector('#customerUniversalResults');
  const advanced=section.querySelector('#customerAdvancedToggle');

  advanced.addEventListener('click',()=>{
    const open=advanced.getAttribute('aria-expanded')!=='true';
    advanced.setAttribute('aria-expanded',String(open));
    advanced.textContent=open?'Hide advanced tools':'Advanced tools';
    if(legacyHero)legacyHero.hidden=!open;
    if(legacyEVM)legacyEVM.hidden=!open;
  });

  async function runScan(){
    const target=input.value.trim();
    const classification=classify(target);
    if(!target){status.textContent='Paste an address first.';input.focus();return;}
    if(classification.family==='unknown'){
      status.textContent='Address format was not recognized. Check the address and try again.';
      wrap.hidden=true;
      return;
    }
    submit.disabled=true;
    submit.textContent='Scanning…';
    wrap.hidden=true;
    overview.innerHTML='';
    results.innerHTML='';
    status.textContent=classification.family==='evm'?'EVM address detected. Checking supported EVM networks…':`${classification.label} detected. Collecting read-only evidence…`;
    try{
      const entries=await Promise.all(classification.networks.map(([network,label])=>scanNetwork(target,network,label)));
      overview.innerHTML=summaryBlock(classification,entries);
      results.innerHTML=entries.map(resultCard).join('');
      wrap.hidden=false;
      const live=entries.filter(x=>x.ok&&x.data?.result).length;
      status.textContent=live?`Scan complete. ${live} evidence source${live===1?'':'s'} returned a result.`:'No configured evidence source returned a result. Koschei did not convert missing evidence into a safety claim.';
    }finally{
      submit.disabled=false;
      submit.textContent='Scan address';
    }
  }

  form.addEventListener('submit',event=>{event.preventDefault();runScan();});

  const params=new URLSearchParams(location.search);
  const initial=params.get('target');
  if(initial){input.value=initial;queueMicrotask(runScan);}
}

document.readyState==='loading'?document.addEventListener('DOMContentLoaded',install,{once:true}):install();
})();
