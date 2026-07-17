(()=>{
  'use strict';
  const kit=window.OwnerRadarKit;
  if(!kit||window.__ownerFullScanTruthUIInstalled)return;
  window.__ownerFullScanTruthUIInstalled=true;
  const esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  const arr=value=>Array.isArray(value)?value:[];
  const obj=value=>value&&typeof value==='object'&&!Array.isArray(value)?value:{};
  const num=value=>new Intl.NumberFormat('tr-TR',{maximumFractionDigits:4}).format(Number(value||0));
  const money=value=>Number.isFinite(Number(value))?new Intl.NumberFormat('en-US',{style:'currency',currency:'USD',maximumFractionDigits:2}).format(Number(value)):'—';
  const rootFor=value=>typeof value==='string'?document.getElementById(value):value;
  const moduleExecution=module=>{const signals=obj(module?.signals),status=String(signals.execution_status||'').toLowerCase();if(['completed','not_applicable','evidence_pending','source_unavailable','insufficient_evidence'].includes(status))return status;if(module?.signed)return'completed';return'evidence_pending'};
  const statusTR=value=>({completed:'TAMAMLANDI',not_applicable:'UYGULANAMAZ',evidence_pending:'KANIT EKSİK',source_unavailable:'KAYNAK HATASI',insufficient_evidence:'YETERSİZ KANIT',verified:'DOĞRULANDI',observed:'GÖZLENDİ',unverified:'DOĞRULANMADI',open:'AÇIK',closed:'KAPALI',unknown:'BİLİNMİYOR',limited:'SINIRLI',not_observed:'GÖZLENMEDİ'}[String(value||'').toLowerCase()]||String(value||'').replaceAll('_',' ').toUpperCase());
  const short=(value,length=34)=>{const text=String(value||'');return text.length>length?`${text.slice(0,length-10)}…${text.slice(-7)}`:text||'—'};
  const badge=(state,label=statusTR(state))=>`<span class="badge ${['verified','completed','closed','not_observed'].includes(String(state||'').toLowerCase())?'ok':['source_unavailable','insufficient_evidence','open'].includes(String(state||'').toLowerCase())?'bad':'warn'}">${esc(label)}</span>`;
  function cardByEyebrow(root,needle){return[...root.querySelectorAll('.card')].find(card=>String(card.querySelector('.eyebrow')?.textContent||'').includes(needle));}
  function renderCapabilities(payload){
    const legacy=obj(payload.legacy_14_arm_radar),modules=arr(legacy.modules),coverage=obj(legacy.investigation_coverage),market=obj(legacy.market),holder=obj(legacy.holder_intelligence);
    if(!modules.length)return'';
    const complete=modules.filter(module=>moduleExecution(module)==='completed');
    const gaps=modules.filter(module=>moduleExecution(module)!=='completed');
    const facts=complete.map(module=>{const evidence=arr(module.evidence),signals=obj(module.signals);const detail=evidence[0]||module.verdict||signals.summary||'Collector tamamlandı.';return`<div class="summary-row"><span class="mono">${esc(module.module_id||'collector')}</span><b style="text-align:left">${esc(module.module||module.module_id)}<small class="muted">${esc(short(detail,120))}</small></b>${badge(module.verified?'verified':'completed',module.verified?'DOĞRULANDI':'TAMAMLANDI')}</div>`}).join('');
    const gapPills=gaps.map(module=>`<span>${esc(module.module||module.module_id)} · ${esc(statusTR(moduleExecution(module)))}</span>`).join('');
    return`<article class="card compact-capability-card"><div class="card-head"><div><span class="eyebrow">ARVIS COLLECTOR SONUCU</span><h2>${esc(coverage.completed||complete.length)} collector tamamlandı</h2><p class="muted">Yalnız veri üreten çalışmalar açılır; çalışmayan modüller tek satırda listelenir.</p></div>${badge(coverage.status||'observed',`${coverage.evidence_producing||0} KANIT ÜRETEN`)}</div><div class="grid compact-grid section-gap"><div class="card kpi"><div class="kpi-label">24H hacim</div><div class="kpi-value tone-cyan">${money(market.volume_24h_usd)}</div></div><div class="card kpi"><div class="kpi-label">Likidite</div><div class="kpi-value tone-cyan">${money(market.liquidity_usd)}</div></div><div class="card kpi"><div class="kpi-label">Market cap</div><div class="kpi-value tone-cyan">${money(market.market_cap_usd)}</div></div><div class="card kpi"><div class="kpi-label">Top owner</div><div class="kpi-value tone-amber">${num(holder.top_owner_percentage)}%</div></div></div>${facts?`<div class="clean-list section-gap">${facts}</div>`:''}${gaps.length?`<div class="compact-gap-list"><b>Eksik veya uygulanamaz collector (${gaps.length})</b>${gapPills}</div>`:''}</article>`;
  }
  function renderBehaviorFacts(payload){
    const report=obj(payload.behavior_signals),signals=arr(report.signals);if(!signals.length)return'';
    const facts=signals.filter(signal=>String(signal.evidence_status||'').toLowerCase()!=='unverified');
    const gaps=signals.filter(signal=>String(signal.evidence_status||'').toLowerCase()==='unverified');
    const rows=facts.map(signal=>{const metrics=Object.entries(obj(signal.metrics)).filter(([,value])=>value!==null&&value!==undefined&&value!=='').slice(0,4);return`<div class="summary-row"><span class="mono">${esc(signal.rule_id)}</span><b style="text-align:left">${esc(signal.title)}<small class="muted">${metrics.map(([key,value])=>`${key.replaceAll('_',' ')}=${typeof value==='number'?num(value):String(value)}`).join(' · ')||'ölçüm yok'}</small></b>${badge(signal.evidence_status,signal.triggered?'TETİKLENDİ':statusTR(signal.evidence_status))}</div>`}).join('');
    return`<article class="card compact-behavior-card"><div class="card-head"><div><span class="eyebrow">DAVRANIŞ KURALLARI · ÖLÇÜM TABLOSU</span><h2>${facts.length} kanıtlı/gözlenen kural satırı</h2><p class="muted">Uzun açıklama yerine ölçüm, eşik ve kanıt durumu.</p></div>${badge(report.triggered_rule_count?'observed':'completed',`${report.triggered_rule_count||0} TETİK`)}</div>${rows?`<div class="clean-list section-gap">${rows}</div>`:''}${gaps.length?`<div class="compact-gap-list"><b>İşlem penceresi gerektiren kurallar</b>${gaps.map(signal=>`<span>${esc(signal.rule_id)} · ${esc(statusTR(signal.evidence_status))}</span>`).join('')}</div>`:''}</article>`;
  }
  function renderThreatFacts(payload){
    const report=obj(payload.threat_anticipation);if(!Object.keys(report).length)return'';
    const exit=obj(report.exit_capacity),paths=arr(report.pathways),missing=arr(report.missing_evidence);
    return`<article class="card compact-threat-card"><div class="card-head"><div><span class="eyebrow">TEHDİT YOLLARI · TEKNİK DURUM</span><h2>${paths.filter(path=>path.status==='open'||path.status==='observed').length} açık/gözlenen yol</h2><p class="muted">Yol durumu gerçek collector çıktısından okunur; niyet veya olasılık üretilmez.</p></div>${badge(report.status==='evidence_backed_pathway_analysis'?'verified':'observed')}</div><div class="metadata section-gap"><div><label>Owner payı</label><b>${num(exit.owner_percentage)}%</b></div><div><label>Pozisyon</label><b>${money(exit.owner_reference_usd_value)}</b></div><div><label>Likidite</label><b>${money(exit.liquidity_usd)}</b></div><div><label>Pozisyon / likidite</label><b>${exit.position_liquidity_multiple==null?'—':`${num(exit.position_liquidity_multiple)}x`}</b></div></div><div class="clean-list section-gap">${paths.map(path=>`<div class="summary-row"><span class="mono">${esc(path.id)}</span><b style="text-align:left">${esc(path.label)}</b>${badge(path.status)}</div>`).join('')}</div>${missing.length?`<div class="compact-gap-list"><b>Eksik teknik girdiler</b>${missing.map(item=>`<span>${esc(item)}</span>`).join('')}</div>`:''}</article>`;
  }
  function actorDossierEmpty(payload){
    const actor=obj(payload.actor_investigation),dossier=obj(actor.dossier),track=obj(dossier.track);
    if(String(actor.wallet||'').trim())return false;
    if(arr(dossier.tokens).length||arr(dossier.related_actors).length||arr(dossier.evidence).length)return false;
    return ['created_token_count','dominant_holder_token_count','traded_token_count','related_actor_count','verified_evidence_count','observed_evidence_count'].every(key=>Number(track[key]||0)===0);
  }
  function decorate(root,payload){
    root=rootFor(root);if(!root)return;
    root.querySelector('#full-scan-live-evidence')?.remove();
    const liveHTML=window.KoscheiLiveEvidenceCard?.render(payload,{lang:'tr'})||'';
    const verdict=root.querySelector('#verdict-card');
    if(liveHTML&&verdict)verdict.insertAdjacentHTML('afterend',liveHTML);
    const capability=cardByEyebrow(root,'ARVIS ARAŞTIRMA YETENEKLERİ');
    const capabilityHTML=renderCapabilities(payload);if(capability&&capabilityHTML)capability.outerHTML=capabilityHTML;
    const behavior=cardByEyebrow(root,'DAVRANIŞ KURALLARI');
    const behaviorHTML=renderBehaviorFacts(payload);if(behavior&&behaviorHTML)behavior.outerHTML=behaviorHTML;
    const threat=root.querySelector('#threat-anticipation');
    const threatHTML=renderThreatFacts(payload);if(threat&&threatHTML)threat.outerHTML=threatHTML;
    if(actorDossierEmpty(payload)){
      const actorCard=cardByEyebrow(root,'KOSCHEI DEFENSE NETWORK · ACTOR DOSSIER');
      actorCard?.remove();
    }
  }
  const baseRender=kit.renderUnified;
  const renderUnified=(root,payload)=>{const result=baseRender?.(root,payload);decorate(root,payload);return result};
  const baseScan=kit.scan;
  const scan=async(target,rootId)=>{const payload=await baseScan(target,rootId);decorate(rootId,payload);return payload};
  window.OwnerRadarKit={...kit,renderUnified,scan};
})();
