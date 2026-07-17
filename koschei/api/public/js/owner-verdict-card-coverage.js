(()=>{
  'use strict';
  const kit=window.OwnerRadarKit;
  if(!kit||window.__ownerVerdictCoverageInstalled)return;
  window.__ownerVerdictCoverageInstalled=true;
  const rootFor=value=>typeof value==='string'?document.getElementById(value):value;
  const esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  function attach(root,payload){
    root=rootFor(root);if(!root||!window.KoscheiVerdictCard)return;
    const card=root.querySelector('#verdict-card'),host=card?.querySelector('.vc-header>div:last-child');
    if(!host)return;
    host.querySelector('.vc-coverage')?.remove();
    const vm=window.KoscheiVerdictCard.mapVerdictCard(payload,{lang:'tr'});
    if(!vm?.coverage?.text)return;
    host.insertAdjacentHTML('beforeend',`<div class="vc-coverage">${esc(vm.coverage.text)}</div>`);
  }
  const baseRender=kit.renderUnified;
  const renderUnified=(root,payload)=>{const result=baseRender?.(root,payload);attach(root,payload);return result};
  const baseScan=kit.scan;
  const scan=async(target,rootId)=>{const payload=await baseScan(target,rootId);attach(rootId,payload);return payload};
  window.OwnerRadarKit={...kit,renderUnified,scan};
})();
