(()=>{
'use strict';
if(window.__ownerActorIntelligenceInstalled)return;
const kit=window.OwnerRadarKit;
if(!kit?.scan)return;
window.__ownerActorIntelligenceInstalled=true;
const originalScan=kit.scan.bind(kit);
const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const arr=v=>Array.isArray(v)?v:[];
const obj=v=>v&&typeof v==='object'&&!Array.isArray(v)?v:{};
const short=v=>{const s=String(v||'');return s.length>20?s.slice(0,8)+'…'+s.slice(-6):s||'—'};
const num=v=>new Intl.NumberFormat('tr-TR',{maximumFractionDigits:4}).format(Number(v||0));
const statusLabel=v=>({
  verified_linked_actor_network:'BAĞLANTILI AKTÖR AĞI DOĞRULANDI',
  verified_actor_observation_no_cross_link:'AKTÖR İNCELENDİ · GÜÇLÜ ÇAPRAZ BAĞ YOK',
  partial_actor_observation:'AKTÖR KANITI KISMİ',
  actor_evidence_unavailable:'AKTÖR KANITI YOK'
})[String(v||'')]||String(v||'KANIT BEKLİYOR').replaceAll('_',' ').toUpperCase();
const relationLabel=v=>({
  funded_creator:'Creator fonlandı',
  creator_token_outflow_recipient:'Creator token gönderdi',
  funded_top_holder:'Top holder fonlandı',
  holder_to_holder:'Holder → holder transferi',
  external_token_recipient:'Token çıkış recipient’i',
  dex_program_exit_context:'DEX/pool çıkış rotası'
})[String(v||'')]||String(v||'').replaceAll('_',' ');
const solscanAddress=a=>`https://solscan.io/account/${encodeURIComponent(a)}`;
const solscanTx=s=>`https://solscan.io/tx/${encodeURIComponent(s)}`;
function rootFor(v){return typeof v==='string'?document.getElementById(v):v}
function evidenceText(link){const evidence=arr(link.evidence);return evidence.length?evidence[0]:''}
function linkCard(link){
  const signature=String(link.signature||'');
  const amount=link.amount_sol?`${num(link.amount_sol)} SOL`:link.amount_token?`${num(link.amount_token)} token`:'';
  return `<article class="actor-link-card">
    <div class="actor-link-main"><a href="${solscanAddress(link.from)}" target="_blank" rel="noopener">${esc(short(link.from))}</a><span>${esc(relationLabel(link.relation))}</span><a href="${solscanAddress(link.to)}" target="_blank" rel="noopener">${esc(short(link.to))}</a></div>
    <div class="actor-link-meta">${amount?`<b>${esc(amount)}</b>`:''}${link.slot?`<span>Slot ${esc(link.slot)}</span>`:''}${signature?`<a href="${solscanTx(signature)}" target="_blank" rel="noopener">İmzayı aç ↗</a>`:''}</div>
    ${evidenceText(link)?`<p>${esc(evidenceText(link))}</p>`:''}
  </article>`
}
function render(root,payload){
  const a=obj(payload.actor_intelligence),coverage=obj(a.coverage),links=arr(a.links).filter(x=>!['controls_token_balance','created_or_deployed','observed_launch_relation'].includes(String(x.relation||'')));
  const creator=String(a.creator_wallet||'');
  const section=root.querySelector('[data-actor-security]')||document.createElement('section');
  section.dataset.actorSecurity='1';section.className='actor-security-panel';
  const findings=arr(a.findings);
  section.innerHTML=`
    <div class="actor-security-head">
      <div><span class="actor-kicker">SOLANA DOLANDIRICILIK AKTÖR İSTİHBARATI</span><h2>Bunlar kim ve birbirlerine nasıl bağlı?</h2><p>Adres listesi değil; creator, funder, Top-holder ve token çıkışları arasındaki yalnız doğrulanmış zincir üstü bağlar.</p></div>
      <span class="actor-status ${esc(a.confidence||'none')}">${esc(statusLabel(a.status))}</span>
    </div>
    <div class="actor-metrics">
      <div><span>Creator / deployer</span><b>${creator?esc(short(creator)):'DOĞRULANAMADI'}</b>${creator?`<a href="${solscanAddress(creator)}" target="_blank" rel="noopener">Solscan’da aç ↗</a>`:'<small>Kök aktör yoksa kimlik kararı yok</small>'}</div>
      <div><span>Önceki launch</span><b>${esc(a.previous_launch_count||0)}</b><small>Aynı creator ile Koschei gözlemi</small></div>
      <div><span>Çapraz aktör bağı</span><b>${esc(coverage.cross_actor_links||0)}</b><small>Funding, recipient veya holder flow</small></div>
      <div><span>Davranış kapsamı</span><b>${esc(coverage.holder_wallets_analyzed||0)}/${esc(coverage.holder_wallets_requested||0)}</b><small>Parsed işlem üreten holder wallet</small></div>
      <div><span>Creator işlemi</span><b>${esc(coverage.creator_transactions_checked||0)}</b><small>Başarılı parsed transaction</small></div>
      <div><span>Creator ↔ Top holder</span><b>${esc(a.creator_linked_top_holder_count||0)}</b><small>Token çıkışıyla eşleşen holder</small></div>
    </div>
    ${findings.length?`<div class="actor-findings">${findings.map((x,i)=>`<div><span>${i+1}</span><p>${esc(x)}</p></div>`).join('')}</div>`:''}
    <div class="actor-links-head"><h3>Doğrulanmış bağlantılar</h3><span>${links.length} kanıt bağı</span></div>
    ${links.length?`<div class="actor-links-grid">${links.map(linkCard).join('')}</div>`:`<div class="actor-empty"><b>Çapraz aktör bağlantısı doğrulanmadı.</b><p>Bu, bağlantı olmadığı anlamına gelmez. Mevcut imza ve parsed transaction penceresi sonuç üretmedi.</p></div>`}
    <div class="actor-action"><b>Sonraki güvenlik adımı</b><p>${esc(a.recommended_action||'Kanıt kapsamını genişlet.')}</p></div>`;
  if(!section.isConnected)root.prepend(section);
}
function loading(root){
  const old=root.querySelector('[data-actor-security]');if(old)old.remove();
  const section=document.createElement('section');section.dataset.actorSecurity='1';section.className='actor-security-panel actor-loading';section.innerHTML='<b>Aktör ağı çıkarılıyor…</b><span>Creator, funder, Top-holder ve çıkış imzaları birleştiriliyor.</span>';root.prepend(section);
}
function failure(root,message){
  const section=root.querySelector('[data-actor-security]');if(!section)return;
  section.className='actor-security-panel actor-failure';section.innerHTML=`<b>Aktör istihbaratı tamamlanamadı.</b><span>${esc(message)}</span>`;
}
async function loadActor(target,root){
  loading(root);
  const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),40000);
  try{
    const response=await fetch(`/api/owner/actor-intelligence?target=${encodeURIComponent(target)}&network=solana-mainnet`,{credentials:'same-origin',signal:controller.signal});
    let data={};try{data=await response.json()}catch{}
    if(!response.ok||data.ok===false)throw new Error(data.message||data.error||`Aktör isteği başarısız (${response.status})`);
    render(root,data);
  }catch(error){failure(root,error?.name==='AbortError'?'Aktör analizi 40 saniyede tamamlanamadı.':(error?.message||'Bilinmeyen hata.'))}
  finally{clearTimeout(timer)}
}
async function scan(target,rootId){
  const data=await originalScan(target,rootId);
  const root=rootFor(rootId);if(root)await loadActor(target,root);
  return data;
}
window.OwnerRadarKit={...kit,scan};
})();
