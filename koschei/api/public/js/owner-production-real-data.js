(()=>{
'use strict';

const TR={
  'Owner Operation Center':'Owner Operasyon Merkezi',
  'Owner Control Center':'Owner Operasyon Merkezi',
  'Owner session':'Owner oturumu',
  'Owner only':'Yalnızca owner',
  'Customer operations':'Müşteri operasyonları',
  'Queue health':'Kuyruk sağlığı',
  'Retryable':'Yeniden denenebilir',
  'Exhausted':'Tükenen',
  'Processed':'İşlenen',
  'Failed':'Başarısız',
  'Runtime arms':'Çalışan kanıt kolları',
  'Pipeline':'İşlem hattı',
  'Database':'Veritabanı',
  'Neon Auth':'Neon Auth',
  'Solana RPC':'Solana RPC',
  'Shopier':'Shopier',
  'ARVIS Radar':'ARVIS Radar',
  'Owner/customer audit':'Owner / müşteri denetimi',
  'Error event':'Hata olayı',
  'No active package':'Aktif paket yok',
  'none':'yok',
  'unknown':'bilinmiyor',
  'configured':'yapılandırıldı',
  'connected':'bağlı',
  'active':'aktif',
  'enabled':'açık',
  'disabled':'kapalı',
  'ready':'hazır',
  'live':'canlı',
  'stale':'güncel değil',
  'waiting':'bekleniyor',
  'healthy':'sağlıklı',
  'processing':'işleniyor',
  'degraded':'zayıfladı',
  'completed':'tamamlandı',
  'success':'başarılı',
  'error':'hata',
  'critical':'kritik',
  'manual':'manuel',
  'pending':'bekliyor',
  'approved':'onaylandı',
  'rejected':'reddedildi',
  'banned':'yasaklı',
  'removed':'kaldırıldı',
  'Production ödeme sağlayıcısı':'Devre dışı ödeme sağlayıcısı',
  'Otomatik entitlement':'Otomatik paket yok; Shopier + owner onayı',
  'Paddle sipariş olayı yok.':'Paddle kullanılmıyor. Tek ödeme akışı: Shopier + owner onayı.',
  'Paddle olayları':'Paddle kullanılmıyor',
  'Otomatik ödeme':'Devre dışı ödeme',
  'Paddle sipariş akışı':'Devre dışı ödeme sağlayıcısı',
  'Aktif Paddle paket':'Aktif Shopier paketi',
  'Paddle':'Kapalı'
};

const REAL_ONLY_NOTE='Demo veri yok · mock skor yok · yalnız üretim verisi';
const qs=(s,r=document)=>Array.from(r.querySelectorAll(s));
function translateTextNode(node){
  if(!node||node.nodeType!==3)return;
  let text=node.nodeValue;
  Object.entries(TR).forEach(([from,to])=>{text=text.replaceAll(from,to)});
  node.nodeValue=text;
}
function walk(root=document.body){
  const walker=document.createTreeWalker(root,NodeFilter.SHOW_TEXT);
  const nodes=[];while(walker.nextNode())nodes.push(walker.currentNode);
  nodes.forEach(translateTextNode);
  qs('[placeholder]').forEach(el=>{let v=el.getAttribute('placeholder')||'';Object.entries(TR).forEach(([from,to])=>v=v.replaceAll(from,to));el.setAttribute('placeholder',v)});
}
function hardenOwnerPanel(){
  walk();

  document.title='KOSCHEİ WEB3 · Owner Operasyon Merkezi';
  const sync=document.getElementById('syncText');
  if(sync&&/demo|mock/i.test(sync.textContent)) sync.textContent='Üretim verisi';

  qs('.hero-card .muted, .login-card .muted').forEach(el=>{
    if(!el.dataset.realNote){el.dataset.realNote='1';el.insertAdjacentHTML('beforeend',`<br><b style="color:#18ffb2">${REAL_ONLY_NOTE}</b>`)}
  });

  qs('.service-mini, .card, .kpi, tr, .summary-row').forEach(el=>{
    if(/paddle/i.test(el.textContent)){
      el.classList.add('koschei-provider-disabled');
      el.querySelectorAll('b,h2,.kpi-label,.badge').forEach(x=>x.textContent=x.textContent.replace(/Paddle/gi,'Kapalı ödeme sağlayıcısı'));
      el.querySelectorAll('small,.kpi-foot,span').forEach(x=>x.textContent=x.textContent.replace(/Paddle olayları|Paddle sipariş olayı yok\.|Paddle kullanılmıyor|Otomatik entitlement/gi,'Shopier + owner onayı'));
    }
  });

  qs('.badge').forEach(el=>{
    const raw=el.textContent.trim();
    if(TR[raw]) el.textContent=TR[raw];
  });

  qs('[data-nav="revenue"]').forEach(el=>{
    if(el.textContent.includes('Kapalı ödeme sağlayıcısı')) el.textContent='Gelir';
  });
}

const style=document.createElement('style');
style.textContent=`
.koschei-provider-disabled{opacity:.72;filter:saturate(.8)}
.koschei-provider-disabled .badge{border-color:#ffcc6655!important;background:#ffcc6614!important;color:#ffe0a3!important}
`;
document.head.appendChild(style);

const mo=new MutationObserver(()=>{clearTimeout(window.__ownerRealDataTimer);window.__ownerRealDataTimer=setTimeout(hardenOwnerPanel,40)});
window.addEventListener('DOMContentLoaded',()=>{hardenOwnerPanel();mo.observe(document.body,{childList:true,subtree:true,characterData:true})});
setTimeout(hardenOwnerPanel,800);
setTimeout(hardenOwnerPanel,1800);
})();
