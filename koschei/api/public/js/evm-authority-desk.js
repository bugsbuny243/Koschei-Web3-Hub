(()=>{
'use strict';
const form=document.getElementById('evmAuthorityForm');
const result=document.getElementById('evmAuthorityResult');
const status=document.getElementById('evmAuthorityStatus');
if(!form||!result||!status)return;
const address=document.getElementById('evmAuthorityAddress');
const network=document.getElementById('evmAuthorityNetwork');
const submit=document.getElementById('evmAuthoritySubmit');
const esc=value=>String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
const short=value=>{const s=String(value??'');return s.length>26?`${s.slice(0,12)}…${s.slice(-10)}`:s};
const truth=value=>value===true?'YES':value===false?'NO':'UNKNOWN';
const tone=value=>value===true?'verified':value===false?'muted':'unknown';
const row=(label,value,state='observed')=>`<div class="evm-desk-row" data-state="${state}"><span>${esc(label)}</span><b>${esc(value||'—')}</b></div>`;
function trustHTML(trust={}){
  const axes=[['Claimed',trust.claimed],['Observed',trust.observed],['Authorized',trust.authorized],['Available',trust.available],['Verified',trust.verified],['Finalized',trust.finalized]];
  return `<div class="evm-trust-grid">${axes.map(([label,value])=>`<div data-state="${tone(value)}"><span>${label}</span><strong>${truth(value)}</strong></div>`).join('')}</div>`;
}
function authorityHTML(authority){
  if(!authority)return `<div class="evm-desk-empty"><b>No authority snapshot attached.</b><span>The scan may still contain observed address evidence. Missing authority evidence is not proof of immutability.</span></div>`;
  const nodes=[
    ['Contract code',authority.contract_code_state],
    ['Code hash',authority.contract_code_sha256?short(authority.contract_code_sha256):'not attached'],
    ['EIP-7702',authority.delegation_state],
    ['Delegate target',authority.delegation_target?short(authority.delegation_target):'not observed'],
    ['ERC-1967',authority.proxy_state],
    ['Implementation',authority.proxy_implementation?short(authority.proxy_implementation):'not observed'],
    ['Admin',authority.proxy_admin?short(authority.proxy_admin):'not observed'],
    ['Beacon',authority.proxy_beacon?short(authority.proxy_beacon):'not observed'],
    ['Conflicting slots',authority.conflicting_slots?'observed':'not observed']
  ];
  return `<div class="evm-authority-path">${nodes.map((item,index)=>`<div class="evm-authority-node"><i>${String(index+1).padStart(2,'0')}</i><span><b>${esc(item[0])}</b><small>${esc(item[1]||'—')}</small></span></div>`).join('')}</div>`;
}
function render(payload){
  const data=payload?.result||{};
  const authority=data.evm_authority;
  const reasons=Array.isArray(data.reasons)?data.reasons:[];
  result.hidden=false;
  result.innerHTML=`<div class="evm-desk-result-head"><div><span>EVM evidence dossier</span><h3>${esc((data.target?.network_hint||network.value).replaceAll('-',' '))}</h3></div><b>${esc(String(data.status||'unknown').toUpperCase())}</b></div>
  <div class="evm-desk-target">${esc(data.target?.raw||address.value)}</div>
  <div class="evm-desk-columns">
    <section><div class="evm-desk-subhead"><span>Trust vector</span><b>Backend state only</b></div>${trustHTML(data.trust||{})}<p class="evm-desk-note">Observed evidence does not imply authorization, verification, finality or safety.</p></section>
    <section><div class="evm-desk-subhead"><span>Authority graph</span><b>Read-only observations</b></div>${authorityHTML(authority)}</section>
  </div>
  <section class="evm-desk-reasons"><div class="evm-desk-subhead"><span>Reason codes</span><b>${reasons.length} attached</b></div>${reasons.length?reasons.map(reason=>row(reason,'OBSERVED','observed')).join(''):`<div class="evm-desk-empty"><b>No reason code attached.</b><span>This does not upgrade the result to safe.</span></div>`}</section>
  <p class="evm-desk-foot">READ-ONLY · OBSERVED ≠ VERIFIED · NO ERC-1967 SLOT ≠ IMMUTABLE · AUTHORITY EVIDENCE ≠ SAFE SPENDER</p>`;
}
form.addEventListener('submit',async event=>{
  event.preventDefault();
  const target=address.value.trim();
  if(!/^0x[0-9a-fA-F]{40}$/.test(target)){
    status.textContent='Enter a valid 20-byte EVM address.';status.dataset.state='error';return;
  }
  submit.disabled=true;status.textContent='Collecting read-only chain and authority evidence…';status.dataset.state='working';result.hidden=true;
  try{
    const response=await fetch('/api/scan',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({target,network:network.value})});
    const payload=await response.json().catch(()=>({}));
    if(!response.ok)throw new Error(payload.error||`HTTP ${response.status}`);
    render(payload);status.textContent='Observed evidence loaded. Review authority and trust boundaries below.';status.dataset.state='ready';
  }catch(error){
    status.textContent=`Evidence unavailable: ${error.message||'unknown error'}`;status.dataset.state='error';
  }finally{submit.disabled=false;}
});
})();