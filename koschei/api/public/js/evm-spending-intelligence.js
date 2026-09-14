(()=>{
'use strict';
const form=document.getElementById('evmSpendingForm');
if(!form)return;
const network=document.getElementById('evmSpendingNetwork');
const token=document.getElementById('evmSpendingToken');
const owner=document.getElementById('evmSpendingOwner');
const spender=document.getElementById('evmSpendingSpender');
const submit=document.getElementById('evmSpendingSubmit');
const status=document.getElementById('evmSpendingStatus');
const result=document.getElementById('evmSpendingResult');
const esc=value=>String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
const short=value=>{const text=String(value||'');return text.length>26?`${text.slice(0,12)}…${text.slice(-10)}`:text};
const validAddress=value=>/^0x[0-9a-fA-F]{40}$/.test(String(value||'').trim());
const yesNo=value=>value?'YES':'NO';
const norm=value=>String(value||'').trim().toLowerCase();

function trustCells(trust={}){
  const axes=[['Claimed','claimed'],['Observed','observed'],['Authorized','authorized'],['Available','available'],['Verified','verified'],['Finalized','finalized']];
  return axes.map(([label,key])=>`<div data-on="${trust[key]?'true':'false'}"><span>${label}</span><b>${trust[key]?'YES':'NO'}</b></div>`).join('');
}
function authorityHTML(authority){
  if(!authority)return `<div class="esi-empty"><b>Spender authority unavailable.</b><span>The allowance observation remains valid. Missing authority evidence is not proof that the spender is immutable or safe.</span></div>`;
  const rows=[
    ['Contract code',authority.contract_code_state],
    ['Code hash',authority.contract_code_sha256],
    ['EIP-7702',authority.delegation_state],
    ['Delegate',authority.delegation_target],
    ['ERC-1967',authority.proxy_state],
    ['Implementation',authority.proxy_implementation],
    ['Admin',authority.proxy_admin],
    ['Beacon',authority.proxy_beacon],
    ['Conflicting slots',yesNo(Boolean(authority.conflicting_slots))]
  ].filter(([,value])=>value!==undefined&&value!==null&&value!=='');
  return rows.map(([label,value],index)=>`<div class="esi-authority-row"><span>${String(index+1).padStart(2,'0')}</span><div><b>${esc(label)}</b><small title="${esc(value)}">${esc(short(value))}</small></div></div>`).join('');
}
function validateObservedAllowance(payload,expected){
  if(payload?.schema_version!=='koschei-approval-scan-v1')throw new Error('approval evidence schema mismatch');
  const data=payload?.result;
  if(!data||typeof data!=='object')throw new Error('approval evidence missing');
  if(norm(data.network)!==norm(expected.network))throw new Error('approval evidence network mismatch');
  if(norm(data.token)!==norm(expected.token))throw new Error('approval evidence token mismatch');
  if(norm(data.owner)!==norm(expected.owner))throw new Error('approval evidence owner mismatch');
  if(norm(data.spender)!==norm(expected.spender))throw new Error('approval evidence spender mismatch');
  if(norm(data.evidence_status)!=='observed'||norm(data.live_availability)!=='checked')throw new Error('approval evidence incomplete');
  if(typeof data.amount!=='string'||!/^\d+$/.test(data.amount))throw new Error('approval amount unavailable');
  if(data.trust?.observed!==true)throw new Error('approval observation trust missing');
  if(data.trust?.verified===true||data.trust?.authorized===true||data.trust?.finalized===true)throw new Error('approval evidence over-promoted');
  return data;
}
function render(payload){
  const data=payload.result;
  const trust=data.trust||{};
  const reasons=Array.isArray(data.reasons)?data.reasons:[];
  const authority=data.spender_authority||null;
  result.hidden=false;
  result.innerHTML=`
    <div class="esi-summary">
      <div><span>Current allowance</span><strong>${esc(data.amount)}</strong><small>raw ERC-20 uint256 amount</small></div>
      <div data-alert="${data.unlimited?'true':'false'}"><span>Unlimited</span><strong>${data.unlimited?'YES':'NO'}</strong><small>exact max uint256 detection</small></div>
      <div><span>Evidence</span><strong>${esc(String(data.evidence_status).toUpperCase())}</strong><small>${esc(data.live_availability)}</small></div>
    </div>
    <div class="esi-identity">
      <div><span>Token</span><code>${esc(data.token)}</code></div>
      <div><span>Owner</span><code>${esc(data.owner)}</code></div>
      <div><span>Spender</span><code>${esc(data.spender)}</code></div>
    </div>
    <div class="esi-grid">
      <section><div class="esi-subhead"><span>Trust vector</span><b>Allowance observation only</b></div><div class="esi-trust">${trustCells(trust)}</div></section>
      <section><div class="esi-subhead"><span>Spender authority</span><b>Current authority surface</b></div><div class="esi-authority">${authorityHTML(authority)}</div></section>
    </div>
    <div class="esi-reasons"><div class="esi-subhead"><span>Reason codes</span><b>${reasons.length} attached</b></div><div>${reasons.map(reason=>`<code>${esc(reason)}</code>`).join('')||'<span>No reason code attached.</span>'}</div></div>
    <p class="esi-foot">CURRENT ALLOWANCE ≠ APPROVAL PROVENANCE · APPROVAL ≠ SAFE SPENDER · SAME ADDRESS ≠ SAME CODE · NO ERC-1967 SLOT ≠ IMMUTABLE</p>`;
}

form.addEventListener('submit',async event=>{
  event.preventDefault();
  const values={network:network.value,token:token.value.trim(),owner:owner.value.trim(),spender:spender.value.trim()};
  if(!validAddress(values.token)||!validAddress(values.owner)||!validAddress(values.spender)){
    status.textContent='Token, owner and spender must each be a 20-byte EVM address.';
    status.dataset.state='error';
    result.hidden=true;
    return;
  }
  const controller=new AbortController();
  const timer=setTimeout(()=>controller.abort(),15000);
  submit.disabled=true;
  submit.textContent='Reading allowance…';
  status.textContent='Reading current allowance and spender authority with read-only RPC calls…';
  status.dataset.state='loading';
  result.hidden=true;
  try{
    const response=await fetch('/api/scan/approval',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(values),cache:'no-store',credentials:'same-origin',signal:controller.signal});
    const data=await response.json().catch(()=>({}));
    if(!response.ok)throw new Error(data.error||`HTTP ${response.status}`);
    validateObservedAllowance(data,values);
    render(data);
    status.textContent='Current allowance observed. Creation mechanism and owner intent were not inferred.';
    status.dataset.state='ready';
  }catch(error){
    result.hidden=true;
    const message=controller.signal.aborted?'approval evidence timed out after 15 seconds':(error?.message||'unknown error');
    status.textContent=`Spending intelligence unavailable: ${message}`;
    status.dataset.state='error';
  }finally{
    clearTimeout(timer);
    submit.disabled=false;
    submit.textContent='Read spending authority';
  }
});
})();
