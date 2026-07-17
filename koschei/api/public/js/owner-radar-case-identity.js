(()=>{
  'use strict';
  if(window.__ownerRadarCaseIdentityInstalled)return;
  window.__ownerRadarCaseIdentityInstalled=true;

  const esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  const norm=value=>String(value||'').trim().toLowerCase();
  const short=(value,length=34)=>{const text=String(value||'');return text.length>length?`${text.slice(0,14)}…${text.slice(-10)}`:text||'—'};
  const dt=value=>{const date=value?new Date(value):null;return !date||Number.isNaN(date.getTime())?'—':new Intl.DateTimeFormat('tr-TR',{dateStyle:'short',timeStyle:'medium'}).format(date)};
  const activeByRoot=new WeakMap();
  const priority=['liquidity_removal','creator_sell_acceleration','coordinated_holder_exit','dominant_holder_exit','mint_inflation','freeze_abuse'];
  const labels={
    liquidity_removal:'Likidite kontrol maruziyeti',
    creator_sell_acceleration:'Creator satış hızlanması',
    coordinated_holder_exit:'Koordineli holder çıkış sinyali',
    dominant_holder_exit:'Baskın holder çıkış kapasitesi',
    mint_inflation:'Mint yetkisi maruziyeti',
    freeze_abuse:'Freeze yetkisi maruziyeti'
  };

  function rootFor(value){return typeof value==='string'?document.getElementById(value):value}
  function caseFinding(data){
    const paths=Array.isArray(data?.threat_anticipation?.pathways)?data.threat_anticipation.pathways:[];
    const active=id=>paths.find(path=>String(path?.id||'')===id&&['open','observed','watch'].includes(String(path?.status||'').toLowerCase()));
    const id=priority.find(active);
    return id?labels[id]:'Kanıt kapsamı ve imzalı sonuç';
  }
  function decorate(root,data,target,scanId){
    root.querySelector('[data-radar-case-identity]')?.remove();
    const final=data?.final_verdict||{};
    const signature=final.signature||data?.signature||'';
    const identity=document.createElement('article');
    identity.className='card radar-case-file radar-case-identity';
    identity.dataset.radarCaseIdentity='1';
    identity.innerHTML=`<div class="card-head"><div><span class="eyebrow">AKTİF VAKA · YENİ TARAMA</span><h2>${esc(caseFinding(data))}</h2></div><span class="badge ok">HEDEF EŞLEŞTİ</span></div><code class="case-target">${esc(target)}</code><div class="case-meta"><span>Üretim: ${esc(dt(data?.generated_at||final.generated_at))}</span><span>İmza: ${esc(short(signature))}</span><span>İstek: ${esc(short(scanId,22))}</span></div>`;
    root.prepend(identity);
    root.dataset.caseTarget=target;
    root.dataset.caseSignature=signature;
  }

  function install(){
    const kit=window.OwnerRadarKit;
    if(!kit||kit.__caseIdentityBound||typeof kit.scan!=='function')return false;
    const baseScan=kit.scan;
    kit.scan=async function(target,rootId){
      const root=rootFor(rootId);
      if(!root)throw new Error('Radar sonuç alanı bulunamadı.');
      const clean=String(target||'').trim();
      if(!clean)throw new Error('Tarama hedefi boş olamaz.');
      const scanId=globalThis.crypto?.randomUUID?.()||`scan-${Date.now()}-${Math.random().toString(16).slice(2)}`;
      activeByRoot.set(root,scanId);
      root.dataset.activeScanId=scanId;
      root.dataset.caseTarget=clean;
      const data=await baseScan(clean,rootId);
      if(activeByRoot.get(root)!==scanId)throw new Error('Eski tarama yanıtı yeni vakanın üzerine yazılmadı.');
      const returned=String(data?.target||data?.wallet||'').trim();
      if(returned&&norm(returned)!==norm(clean)){
        root.innerHTML='<div class="card error-state"><div><b>Vaka hedefi uyuşmadı.</b><span>Eski veya farklı hedefe ait yanıt ekrana basılmadı.</span></div></div>';
        throw new Error(`Vaka hedefi uyuşmadı: istek ${clean}, yanıt ${returned}`);
      }
      decorate(root,data,clean,scanId);
      return data;
    };
    kit.__caseIdentityBound=true;
    window.OwnerRadarKit=kit;
    return true;
  }

  if(!install()){
    const timer=setInterval(()=>{if(install())clearInterval(timer)},100);
    setTimeout(()=>clearInterval(timer),5000);
  }
})();
