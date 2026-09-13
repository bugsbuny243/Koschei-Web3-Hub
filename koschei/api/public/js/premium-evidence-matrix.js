(()=>{
'use strict';
const result=document.getElementById('result');
if(!result)return;
const esc=value=>String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
const STATE_ORDER=['verified','observed','window_open','arm_pending','not_applicable'];
const STATE_LABEL={verified:'Verified',observed:'Observed',window_open:'Monitoring',arm_pending:'Missing',not_applicable:'N/A'};
const stateOf=node=>STATE_ORDER.find(state=>node.classList.contains(state))||'observed';
const authorityMatch=label=>/authority|mint|freeze|owner control|delegate|admin|upgrade/i.test(label);
let lastSignature='';
let scheduled=false;

function readSignals(){
  return [...result.querySelectorAll('.public-signal')].map(node=>{
    const label=node.querySelector('b')?.textContent?.trim()||'';
    const value=node.querySelector('em')?.textContent?.trim()||'';
    const details=[...node.querySelectorAll('small')].map(x=>x.textContent.trim()).filter(Boolean);
    return {label,value,details,state:stateOf(node)};
  }).filter(row=>row.label);
}
function render(){
  scheduled=false;
  const rows=readSignals();
  const signature=JSON.stringify(rows);
  if(signature===lastSignature)return;
  lastSignature=signature;
  const existing=result.querySelector('[data-premium-evidence-matrix]');
  if(existing)existing.remove();
  if(!rows.length)return;
  const counts=Object.fromEntries(STATE_ORDER.map(state=>[state,rows.filter(row=>row.state===state).length]));
  const authority=rows.filter(row=>authorityMatch(row.label));
  const matrix=document.createElement('section');
  matrix.className='premium-evidence-matrix';
  matrix.dataset.premiumEvidenceMatrix='1';
  matrix.innerHTML=`
    <div class="pem-head">
      <div><span>Evidence matrix</span><h3>What is known, what is observed, what remains unresolved.</h3></div>
      <b>${rows.length} evidence rows</b>
    </div>
    <div class="pem-states">${STATE_ORDER.map(state=>`<div data-state="${state}"><span>${STATE_LABEL[state]}</span><strong>${counts[state]}</strong></div>`).join('')}</div>
    <div class="pem-grid">
      <div class="pem-ledger">
        <div class="pem-subhead"><span>Evidence ledger</span><b>Source-bound result states</b></div>
        ${rows.slice(0,12).map(row=>`<div class="pem-row" data-state="${row.state}"><i></i><span><b>${esc(row.label)}</b><small>${esc(row.details.join(' · '))}</small></span><em>${esc(row.value||'—')}</em></div>`).join('')}
      </div>
      <div class="pem-authority">
        <div class="pem-subhead"><span>Authority surface</span><b>Only attached authority evidence</b></div>
        ${authority.length?authority.slice(0,8).map((row,index)=>`<div class="pem-authority-node" data-state="${row.state}"><span>${String(index+1).padStart(2,'0')}</span><div><b>${esc(row.label)}</b><small>${esc(row.value||STATE_LABEL[row.state])}</small></div></div>`).join(''):`<div class="pem-empty"><b>No authority-specific evidence row is attached.</b><span>This is an evidence gap, not proof of immutability or safety.</span></div>`}
      </div>
    </div>
    <p class="pem-foot">Observed ≠ verified · Missing evidence ≠ safe · UI projection does not upgrade backend trust.</p>`;
  const card=result.querySelector('.public-investigation-card')||result.firstElementChild;
  if(card)card.insertAdjacentElement('afterend',matrix);else result.prepend(matrix);
}
function schedule(){if(scheduled)return;scheduled=true;queueMicrotask(render)}
const observer=new MutationObserver(mutations=>{
  if(mutations.every(m=>m.target.closest?.('[data-premium-evidence-matrix]')))return;
  schedule();
});
observer.observe(result,{childList:true,subtree:true,characterData:true});
render();
})();
