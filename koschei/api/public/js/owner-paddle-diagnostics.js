(()=>{
'use strict';

const esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
const badge=status=>{
  const value=String(status||'unknown').toLowerCase();
  const tone=value==='configured'?'ok':value.includes('ready')?'warn':'bad';
  return `<span class="badge ${tone}">${esc(status||'unknown')}</span>`;
};

let loading=false;
let lastRendered='';

async function loadPaddleDiagnostics(){
  const page=document.getElementById('page-revenue');
  const content=document.getElementById('revenueContent');
  if(!page||!content||!page.classList.contains('active')||loading)return;
  const grid=content.querySelector('.grid');
  if(!grid)return;

  loading=true;
  try{
    const response=await fetch('/api/owner/payment-health',{credentials:'same-origin',headers:{Accept:'application/json'}});
    if(!response.ok)return;
    const data=await response.json();
    const paddle=data?.paddle||{};
    const missing=Array.isArray(paddle.missing_fields)?paddle.missing_fields:[];
    const fingerprint=JSON.stringify(paddle);
    if(fingerprint===lastRendered&&document.getElementById('paddleRuntimeDiagnostics'))return;
    lastRendered=fingerprint;

    document.getElementById('paddleRuntimeDiagnostics')?.remove();
    const card=document.createElement('article');
    card.id='paddleRuntimeDiagnostics';
    card.className='card span-12';
    card.innerHTML=`
      <div class="card-head">
        <div>
          <span class="eyebrow">Paddle runtime tanısı</span>
          <h2>Railway env görünürlüğü</h2>
          <p class="small muted">Secret değerleri gösterilmez; yalnız uygulamanın ilgili alanı görüp görmediği gösterilir.</p>
        </div>
        ${badge(paddle.status)}
      </div>
      <div class="metadata">
        <div><label>API key</label><b>${paddle.api_key_configured?'Hazır':'Görünmüyor'}</b></div>
        <div><label>Webhook secret</label><b>${paddle.webhook_configured?'Hazır':'Görünmüyor'}</b></div>
        <div><label>Starter price</label><b>${paddle.starter_price_configured?'Hazır':'Görünmüyor'}</b></div>
        <div><label>Professional price</label><b>${paddle.professional_price_configured?'Hazır':'Görünmüyor'}</b></div>
        <div><label>Enterprise price</label><b>${paddle.enterprise_price_configured?'Hazır':'Görünmüyor'}</b></div>
        <div><label>Public app URL</label><b>${paddle.public_app_url_configured?'Hazır':'Görünmüyor'}</b></div>
        <div><label>Checkout</label><b>${paddle.checkout_ready?'Hazır':'Hazır değil'}</b></div>
        <div><label>Webhook otomasyonu</label><b>${paddle.automation_ready?'Hazır':'Hazır değil'}</b></div>
      </div>
      ${missing.length
        ? `<div class="error-box" style="margin-top:12px">Uygulamanın eksik gördüğü alanlar: ${esc(missing.join(', '))}</div>`
        : '<div class="success-box" style="margin-top:12px">Zorunlu Paddle runtime alanlarının tamamı görülüyor. Client token ve product ID, mevcut server-side checkout mimarisinde zorunlu değil.</div>'}
    `;

    const firstWide=[...grid.children].find(element=>element.classList.contains('span-12'));
    if(firstWide)grid.insertBefore(card,firstWide);else grid.appendChild(card);
  }catch(error){
    console.warn('Paddle diagnostics unavailable',error);
  }finally{
    loading=false;
  }
}

const observer=new MutationObserver(()=>loadPaddleDiagnostics());
observer.observe(document.documentElement,{subtree:true,childList:true,attributes:true,attributeFilter:['class']});
document.addEventListener('click',event=>{
  if(event.target.closest('[data-nav="revenue"]')||event.target.closest('#refreshButton'))setTimeout(loadPaddleDiagnostics,250);
});
setInterval(loadPaddleDiagnostics,15000);
})();
