(()=>{
  'use strict';

  const kit=window.OwnerRadarKit;
  if(!kit||window.__ownerArvisRuntimeFixInstalled)return;
  window.__ownerArvisRuntimeFixInstalled=true;
  const render=kit.render.bind(kit);

  function rootFor(value){return typeof value==='string'?document.getElementById(value):value}
  function errorCard(root,message,target){
    root.innerHTML='';
    const card=document.createElement('div');card.className='card error-state';
    const copy=document.createElement('div');
    const title=document.createElement('b');title.textContent='Tam tarama tamamlanamadı.';
    const detail=document.createElement('span');detail.textContent=message;
    const retry=document.createElement('button');retry.className='btn small';retry.type='button';retry.textContent='Tekrar dene';retry.onclick=()=>scan(target,root);
    copy.append(title,detail);card.append(copy,retry);root.append(card);
  }

  async function scan(target,rootId){
    const root=rootFor(rootId);
    if(!root)throw new Error('Tarama alanı bulunamadı.');
    root.innerHTML='<div class="card loading">ARVIS zincir, holder, cluster, piyasa ve transaction kanıtlarını topluyor… Bu işlem 2 dakikaya kadar sürebilir.</div>';
    const controller=new AbortController();
    const timer=setTimeout(()=>controller.abort(),180000);
    try{
      const response=await fetch('/api/owner/arvis/scan',{
        method:'POST',credentials:'same-origin',signal:controller.signal,
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({target,network:'solana-mainnet'})
      });
      let data={};
      try{data=await response.json()}catch{}
      if(!response.ok||data.ok===false){
        const error=new Error(data.message||data.detail||data.error||`İstek başarısız (${response.status})`);
        error.status=response.status;throw error;
      }
      render(root,data);
      return data;
    }catch(error){
      const message=error?.name==='AbortError'?'Tam tarama 180 saniyede tamamlanamadı. RPC veya piyasa sağlayıcısı gecikiyor.':(error?.message||'Tam tarama başarısız oldu.');
      errorCard(root,message,target);
      throw error;
    }finally{clearTimeout(timer)}
  }

  window.OwnerRadarKit={...kit,scan};
})();
