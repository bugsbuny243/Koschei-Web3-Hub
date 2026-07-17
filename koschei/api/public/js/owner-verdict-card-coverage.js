(()=>{
  'use strict';
  const kit=window.OwnerRadarKit;
  if(!kit||window.__ownerVerdictCoverageInstalled)return;
  window.__ownerVerdictCoverageInstalled=true;
  const rootFor=value=>typeof value==='string'?document.getElementById(value):value;
  const esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  const short=value=>{const text=String(value||'');return text.length>18?`${text.slice(0,8)}…${text.slice(-6)}`:text};
  function refsHTML(refs={}){const values=[];(refs.wallets||[]).forEach(value=>values.push(['wallet',value]));(refs.accounts||[]).forEach(value=>values.push(['account',value]));(refs.transactions||[]).forEach(item=>values.push([item.signature?'tx':'slot',item.signature||String(item.slot||''),item.slot]));(refs.evidence_keys||[]).forEach(value=>values.push(['evidence',value]));if(!values.length)return'';return`<div class="vc-row-refs">${values.slice(0,8).map(([kind,value,slot])=>`<button type="button" data-copy="${esc(value)}">${esc(kind)} · ${esc(short(value))}${slot?` · ${esc(slot)}`:''}</button>`).join('')}</div>`}
  function attach(root,payload){
    root=rootFor(root);if(!root||!window.KoscheiVerdictCard)return;
    const card=root.querySelector('#verdict-card'),host=card?.querySelector('.vc-header>div:last-child');if(!host)return;
    host.querySelector('.vc-coverage')?.remove();const vm=window.KoscheiVerdictCard.mapVerdictCard(payload,{lang:'tr'});if(!vm?.coverage?.text)return;
    host.insertAdjacentHTML('beforeend',`<div class="vc-coverage">${esc(vm.coverage.text)}</div>`);
    card.querySelectorAll('.vc-row').forEach((element,index)=>{element.querySelector('.vc-row-refs')?.remove();const row=vm.checklist[index];if(row)element.insertAdjacentHTML('beforeend',refsHTML(row.refs))});
    card.querySelectorAll('[data-copy]').forEach(button=>button.addEventListener('click',async event=>{event.preventDefault();event.stopPropagation();try{await navigator.clipboard.writeText(button.dataset.copy||'');const old=button.textContent;button.textContent='Kopyalandı';setTimeout(()=>button.textContent=old,1200)}catch{}}));
  }
  const baseRender=kit.renderUnified;const renderUnified=(root,payload)=>{const result=baseRender?.(root,payload);attach(root,payload);return result};
  const baseScan=kit.scan;const scan=async(target,rootId)=>{const payload=await baseScan(target,rootId);attach(rootId,payload);return payload};
  window.OwnerRadarKit={...kit,renderUnified,scan};
})();
