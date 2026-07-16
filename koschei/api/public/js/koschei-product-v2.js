(()=>{
  'use strict';
  if(window.__koscheiProductV2)return;
  window.__koscheiProductV2=true;
  const ready=fn=>document.readyState==='loading'?document.addEventListener('DOMContentLoaded',fn,{once:true}):fn();
  const esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot',"'":'&#39;'}[char]));

  function installReveal(){
    const nodes=[...document.querySelectorAll('[data-reveal]')];
    if(!nodes.length)return;
    if(!('IntersectionObserver'in window)){nodes.forEach(node=>node.classList.add('is-visible'));return;}
    const observer=new IntersectionObserver(entries=>entries.forEach(entry=>{
      if(!entry.isIntersecting)return;
      entry.target.classList.add('is-visible');
      observer.unobserve(entry.target);
    }),{rootMargin:'0px 0px -8% 0px',threshold:.08});
    nodes.forEach(node=>observer.observe(node));
  }

  async function hydrateHealth(){
    const indicators=[...document.querySelectorAll('[data-koschei-live]')];
    if(!indicators.length)return;
    try{
      const response=await fetch('/health',{cache:'no-store',credentials:'same-origin'});
      if(!response.ok)throw new Error('health_unavailable');
      const data=await response.json().catch(()=>({}));
      const arvis=data.arvis||{};
      const status=String(arvis.pipeline_status||arvis.status||data.status||'ready').toLowerCase();
      const isLive=['ready','healthy','live','connected','ok','manual'].some(value=>status.includes(value));
      indicators.forEach(node=>{
        node.classList.toggle('is-live',isLive);
        node.textContent=isLive?'ARVIS üretim hattı hazır':'ARVIS durumu kontrol ediliyor';
      });
    }catch{
      indicators.forEach(node=>{node.textContent='Canlı durum alınamadı';node.classList.remove('is-live')});
    }
  }

  function installFormState(){
    document.querySelectorAll('form').forEach(form=>form.addEventListener('submit',()=>{
      document.body.classList.add('is-processing');
      window.setTimeout(()=>document.body.classList.remove('is-processing'),6000);
    }));
  }

  function installExternalSafety(){
    document.querySelectorAll('a[target="_blank"]').forEach(link=>{
      if(!link.rel.includes('noopener'))link.rel=(link.rel+' noopener noreferrer').trim();
    });
  }

  function installCurrentNav(){
    const current=(location.pathname||'/').replace(/\.html$/,'').replace(/\/$/,'')||'/';
    document.querySelectorAll('.koschei-global-nav a,.nav a').forEach(link=>{
      const path=(new URL(link.href,location.origin).pathname||'/').replace(/\.html$/,'').replace(/\/$/,'')||'/';
      if(path===current)link.setAttribute('aria-current','page');
    });
  }

  ready(()=>{installReveal();hydrateHealth();installFormState();installExternalSafety();installCurrentNav()});
})();
