(()=>{
  'use strict';
  const kit=window.OwnerRadarKit;
  if(!kit||window.__ownerUnifiedRadarInstalled)return;
  window.__ownerUnifiedRadarInstalled=true;
  const esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  const arr=value=>Array.isArray(value)?value:[];
  const obj=value=>value&&typeof value==='object'&&!Array.isArray(value)?value:{};
  const num=value=>new Intl.NumberFormat('tr-TR',{maximumFractionDigits:4}).format(Number(value||0));
  const money=value=>Number.isFinite(Number(value))?new Intl.NumberFormat('en-US',{style:'currency',currency:'USD',maximumFractionDigits:2}).format(Number(value)):'—';
  const short=(value,length=40)=>{const text=String(value||'');return text.length>length?`${text.slice(0,length-10)}…${text.slice(-7)}`:text||'—'};
  const dt=value=>{if(!value)return'—';const date=new Date(value);return Number.isNaN(date.getTime())?'—':new Intl.DateTimeFormat('tr-TR',{dateStyle:'short',timeStyle:'short'}).format(date)};
  const rootFor=value=>typeof value==='string'?document.getElementById(value):value;
  const tone=value=>{const text=String(value||'').toLowerCase();if(text==='verified'||text==='ok'||text==='b')return'ok';if(text==='unverified'||text==='failed'||text==='d')return'bad';return'warn'};
  const badge=(value,label=value)=>`<span class="badge ${tone(value)}">${esc(String(label||'').toUpperCase())}</span>`;
  const metricValue=value=>{
    if(value===null||value===undefined)return'—';
    if(typeof value==='boolean')return value?'EVET':'HAYIR';
    if(typeof value==='number')return num(value);
    return String(value);
  };
  const gradeLabel=grade=>grade&&grade!=='-'?`GRADE ${grade}`:'GRADE YOK';

  function renderVerdict(verdict){
    verdict=obj(verdict);
    const rules=arr(verdict.triggered_rules),watch=arr(verdict.watch_flags);
    return`<article class="card" style="border-color:#18ffb255"><div class="card-head"><div><span class="eyebrow">TEK RADAR · DETERMINİSTİK FINAL</span><h2>${esc(gradeLabel(verdict.grade))}</h2><p class="muted">${esc(verdict.verdict||'no_grade_trigger')} · ${esc(verdict.ruleset_version||'ruleset yok')}</p></div>${badge(verdict.signed?'verified':'observed',verdict.signed?'İMZALI':'KARAR BEKLİYOR')}</div><div class="warning-box"><b>Sayısal final skor kapalıdır.</b><br>Grade yalnız aşağıdaki açık kurallardan çıkar. AI grade vermez; yalnız kuralları anlatabilir.</div>${rules.length?`<div class="clean-list section-gap">${rules.map(rule=>`<div class="summary-row"><span class="mono">${esc(rule.rule_id)}</span><b style="text-align:left">${esc(rule.title||rule.summary)}<small class="muted">${esc(rule.summary||'')}</small></b>${badge(rule.evidence_status||'observed')}</div>`).join('')}</div>`:'<div class="empty section-gap">Grade değiştiren kural tetiklenmedi. Bu durum güvenli veya A anlamına gelmez.</div>'}${watch.length?`<details class="owner-details section-gap"><summary><span><b>Watch flag</b><small>INFERRED bulgular grade düşürmez.</small></span><span>⌄</span></summary><div class="clean-list section-gap">${watch.map(rule=>`<div class="summary-row"><span class="mono">${esc(rule.rule_id)}</span><b>${esc(rule.summary||rule.title)}</b>${badge('inferred')}</div>`).join('')}</div></details>`:''}<div class="metadata section-gap"><div><label>Ruleset</label><b>${esc(verdict.ruleset_version||'—')}</b></div><div><label>Actor ruleset</label><b>${esc(verdict.actor_ruleset_version||'—')}</b></div><div><label>İmza</label><b class="mono">${esc(short(verdict.signature,48))}</b></div><div><label>Üretim zamanı</label><b>${dt(verdict.generated_at)}</b></div></div></article>`;
  }

  function renderBehavior(report){
    report=obj(report);
    const signals=arr(report.signals);
    if(!signals.length)return'';
    return`<article class="card"><div class="card-head"><div><span class="eyebrow">DAVRANIŞ KURALLARI · 4/4</span><h2>Hacim, likidite, creator satış ve holder çıkışı</h2><p class="muted">Ağırlıklı skor yok; her eşik açık ve versiyonludur.</p></div>${badge('observed',`${num(report.triggered_rule_count)} TETİK`)}</div><div class="grid compact-grid">${signals.map(signal=>`<div class="card"><div class="card-head"><div><span class="eyebrow mono">${esc(signal.rule_id)}</span><h3>${esc(signal.title)}</h3></div>${badge(signal.evidence_status,signal.triggered?'TETİKLENDİ':signal.evidence_status)}</div><p>${esc(signal.summary||'')}</p><div class="metadata">${Object.entries(obj(signal.metrics)).slice(0,8).map(([key,value])=>`<div><label>${esc(key.replaceAll('_',' '))}</label><b>${esc(metricValue(value))}</b></div>`).join('')}</div><div class="muted small section-gap">Kapsam: ${esc(signal.scope||'—')}</div>${arr(signal.signatures).length?`<div class="section-gap">${arr(signal.signatures).map(signature=>`<a class="mono" href="https://solscan.io/tx/${encodeURIComponent(signature)}" target="_blank" rel="noopener noreferrer">${esc(short(signature,34))}</a>`).join('<br>')}</div>`:''}${arr(signal.limitations).length?`<div class="warning-box section-gap">${arr(signal.limitations).map(esc).join(' · ')}</div>`:''}</div>`).join('')}</div></article>`;
  }

  function renderLegacy(legacy){
    legacy=obj(legacy);
    if(legacy.applicable===false)return`<article class="card"><div class="card-head"><div><span class="eyebrow">ESKİ 14 ARVIS KOLU</span><h2>Bu hedefte uygulanamaz</h2></div>${badge('observed','N/A')}</div><div class="warning-box">${esc(legacy.reason||'Token mint gereklidir.')}</div></article>`;
    const modules=arr(legacy.modules),holders=obj(legacy.holder_intelligence),market=obj(legacy.market),source=obj(legacy.source_context);
    return`<article class="card"><div class="card-head"><div><span class="eyebrow">ESKİ 14 ARVIS KOLU · TEK DOSYADA</span><h2>13 kanıt kolu + final kolu</h2><p class="muted">Eski modül sayıları yalnız iç uyumluluk verisidir; birleşik final verdict sayısızdır.</p></div>${badge('verified',`${num(modules.length)} KOL`)}</div><div class="grid compact-grid">${['volume_24h_usd','liquidity_usd','market_cap_usd'].map(key=>`<div class="card kpi"><div class="kpi-label">${esc(key.replaceAll('_',' '))}</div><div class="kpi-value tone-cyan">${money(market[key])}</div></div>`).join('')}<div class="card kpi"><div class="kpi-label">Top owner</div><div class="kpi-value tone-amber">${num(holders.top_owner_percentage)}%</div></div><div class="card kpi"><div class="kpi-label">Creator</div><div class="kpi-value mono" style="font-size:13px">${esc(short(source.creator_wallet,28))}</div></div></div><div class="clean-list section-gap">${modules.map(module=>`<details class="owner-details"><summary><span><b>${esc(module.module||module.module_id)}</b><small>${esc(module.verdict||'Kanıt sonucu')}</small></span>${badge(module.verified?'verified':'observed',module.verified?'VERIFIED':'EVIDENCE PENDING')}</summary><div class="section-gap">${arr(module.evidence).length?`<div class="clean-list">${arr(module.evidence).map((line,index)=>`<div class="summary-row"><span>E${index+1}</span><b style="text-align:left">${esc(line)}</b>${badge(module.verified?'verified':'observed')}</div>`).join('')}</div>`:'<div class="empty compact">Bu kol için doğrulanmış evidence satırı yok.</div>'}</div></details>`).join('')}</div></article>`;
  }

  function renderActor(root,payload){
    const actor=obj(payload.actor_investigation),dossier=obj(actor.dossier);
    if(!Object.keys(dossier).length)return;
    const holder=document.createElement('div');
    holder.className='section-gap';
    root.appendChild(holder);
    if(typeof kit.renderDefense==='function'){
      kit.renderDefense(holder,{schema_version:'koschei-actor-defense-v3',wallet:actor.wallet,dossier,rule_verdict:actor.rule_verdict,funding_origin:actor.funding_origin});
      return;
    }
    holder.innerHTML=`<article class="card"><div class="card-head"><div><span class="eyebrow">ACTOR INVESTIGATION</span><h2>${esc(short(actor.wallet,40))}</h2></div>${badge(obj(dossier.track).state||'observed')}</div></article>`;
  }

  function renderUnified(root,payload){
    root=rootFor(root);
    if(!root)return;
    root.innerHTML=`<div class="grid compact-grid"><div class="span-12">${renderVerdict(payload.final_verdict)}</div><div class="span-12">${renderBehavior(payload.behavior_signals)}</div><div class="span-12">${renderLegacy(payload.legacy_14_arm_radar)}</div></div>`;
    renderActor(root,payload);
  }

  async function scan(target,rootId){
    const root=rootFor(rootId);
    if(!root)throw new Error('Radar sonuç alanı bulunamadı.');
    root.innerHTML='<div class="card loading">Tek Radar; 14 ARVIS kolunu, actor investigation ve dört davranış kuralını manuel olarak çalıştırıyor…</div>';
    const controller=new AbortController();
    const timer=setTimeout(()=>controller.abort(),210000);
    try{
      const response=await fetch('/api/owner/radar/unified',{method:'POST',credentials:'same-origin',signal:controller.signal,headers:{'Content-Type':'application/json'},body:JSON.stringify({target,network:'solana-mainnet',live_evidence:true})});
      let data={};
      try{data=await response.json()}catch{}
      if(!response.ok||data.ok===false)throw new Error(data.message||data.detail||data.error||`İstek başarısız (${response.status})`);
      renderUnified(root,data);
      return data;
    }catch(error){
      const message=error?.name==='AbortError'?'Tek Radar 210 saniyede tamamlanamadı.':(error?.message||'Tek Radar başarısız oldu.');
      root.innerHTML=`<div class="card error-state"><div><b>Tek Radar tamamlanamadı.</b><span>${esc(message)}</span></div></div>`;
      throw error;
    }finally{clearTimeout(timer)}
  }

  window.OwnerRadarKit={...kit,scan,renderUnified};
})();
