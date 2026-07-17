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
  const tone=value=>{const text=String(value||'').toLowerCase();if(['verified','ok','b','closed','not_observed','complete_investigation','completed'].includes(text))return'ok';if(['unverified','failed','d','open','critical','source_unavailable','insufficient_coverage'].includes(text))return'bad';return'warn'};
  const badge=(value,label=value)=>`<span class="badge ${tone(value)}">${esc(String(label||'').toUpperCase())}</span>`;
  const metricValue=value=>{
    if(value===null||value===undefined)return'—';
    if(typeof value==='boolean')return value?'EVET':'HAYIR';
    if(typeof value==='number')return num(value);
    return String(value);
  };
  const gradeLabel=grade=>grade&&grade!=='-'?`GRADE ${grade}`:'GRADE YOK';
  const threatLabel=value=>({open:'AÇIK',closed:'KAPALI',observed:'GÖZLENDİ',watch:'İZLE',unknown:'BİLİNMİYOR',limited:'SINIRLI',not_observed:'GÖZLENMEDİ'}[String(value||'').toLowerCase()]||String(value||'BİLİNMİYOR').toUpperCase());

  const criticalCoverage={
    token_authority_scanner:'Token authority',holder_concentration:'Owner-resolved holder',liquidity_movement:'Likidite derinliği ve kontrolü',
    creator_link_analysis:'Creator/deployer ilişkisi',funding_cluster_detector:'Holder funding ilişkileri',launch_distribution:'Launch / ilk alıcı geçmişi',repeat_actor_scan:'Kalıcı aktör hafızası'
  };

  function moduleExecution(module){
    module=obj(module);const signals=obj(module.signals);const explicit=String(signals.execution_status||'').toLowerCase();
    if(['completed','not_applicable','evidence_pending','source_unavailable','insufficient_evidence'].includes(explicit))return explicit;
    if(module.signed)return'completed';
    if(['walletless_claim_shield','mev_shield'].includes(String(module.module_id||'')))return'not_applicable';
    return'evidence_pending';
  }

  function buildInvestigationCoverage(legacy){
    legacy=obj(legacy);const supplied=obj(legacy.investigation_coverage);if(Object.keys(supplied).length)return supplied;
    const modules=arr(legacy.modules);const counts={completed:0,evidenceProducing:0,completedWithoutMatch:0,notApplicable:0,evidencePending:0,sourceUnavailable:0,insufficientEvidence:0};
    const completed=new Set();
    modules.forEach(module=>{
      const status=moduleExecution(module),signals=obj(module.signals);if(status==='completed'){counts.completed++;completed.add(String(module.module_id||''));if(module.signed&&arr(module.evidence).length)counts.evidenceProducing++;if(signals.finding_observed===false)counts.completedWithoutMatch++;}
      else if(status==='not_applicable')counts.notApplicable++;
      else if(status==='source_unavailable')counts.sourceUnavailable++;
      else if(status==='insufficient_evidence')counts.insufficientEvidence++;
      else counts.evidencePending++;
    });
    const criticalGaps=Object.entries(criticalCoverage).filter(([id])=>!completed.has(id)).map(([,label])=>label);
    let status='insufficient_coverage';if(counts.evidenceProducing>0)status='partial_investigation';if(!criticalGaps.length&&!counts.evidencePending&&!counts.sourceUnavailable&&!counts.insufficientEvidence)status='complete_investigation';
    return{status,capability_total:modules.length,attempted:modules.length,completed:counts.completed,evidence_producing:counts.evidenceProducing,completed_without_match:counts.completedWithoutMatch,not_applicable:counts.notApplicable,evidence_pending:counts.evidencePending,source_unavailable:counts.sourceUnavailable,insufficient_evidence:counts.insufficientEvidence,critical_gaps:criticalGaps};
  }

  function coverageTitle(status){return({complete_investigation:'TAM ARAŞTIRMA KAPSAMI',partial_investigation:'KISMİ ARAŞTIRMA KAPSAMI',insufficient_coverage:'YETERSİZ ARAŞTIRMA KAPSAMI'}[status]||'ARAŞTIRMA KAPSAMI')}
  function moduleStatusBadge(module){
    const status=moduleExecution(module),signals=obj(module.signals);
    if(status==='completed')return module.verified?badge('verified','VERIFIED'):badge('completed',signals.finding_observed===false?'ANALİZ EDİLDİ · EŞLEŞME YOK':'OBSERVED');
    if(status==='not_applicable')return badge('observed','UYGULANAMAZ');
    if(status==='source_unavailable')return badge('failed','KAYNAK HATASI');
    if(status==='insufficient_evidence')return badge('failed','YETERSİZ KANIT');
    return badge('observed','KANIT EKSİK');
  }

  function renderInvestigationCoverage(legacy){
    const coverage=buildInvestigationCoverage(legacy),gaps=arr(coverage.critical_gaps);
    return`<article class="card investigation-coverage-card ${esc(coverage.status)}"><div class="card-head"><div><span class="eyebrow">ARVIS INVESTIGATION COVERAGE</span><h2>${esc(coverageTitle(coverage.status))}</h2><p class="muted">Mimari kol sayısı başarı sayısı değildir. Bu bölüm yalnız gerçekten tamamlanan collector çalışmalarını gösterir.</p></div>${badge(coverage.status,coverage.status==='complete_investigation'?'TAM':coverage.status==='partial_investigation'?'KISMİ':'YETERSİZ')}</div><div class="coverage-metrics section-gap"><div><label>Yetenek</label><b>${num(coverage.capability_total)}</b></div><div><label>Tamamlandı</label><b>${num(coverage.completed)}</b></div><div><label>Kanıt üretti</label><b>${num(coverage.evidence_producing)}</b></div><div><label>Bulgu yok</label><b>${num(coverage.completed_without_match)}</b></div><div><label>Uygulanamaz</label><b>${num(coverage.not_applicable)}</b></div><div><label>Kanıt eksik</label><b>${num(coverage.evidence_pending)}</b></div><div><label>Kaynak hatası</label><b>${num(coverage.source_unavailable)}</b></div></div>${gaps.length?`<div class="warning-box section-gap"><b>Kritik kapsam boşlukları</b><br>${gaps.map(esc).join(' · ')}</div>`:'<div class="warning-box section-gap coverage-complete"><b>Kritik collector boşluğu yok.</b><br>Bu ifade güvenli veya risksiz anlamına gelmez; yalnız araştırma kapsamını tanımlar.</div>'}</article>`;
  }

  function renderVerdictCard(payload){
    if(!window.KoscheiVerdictCard)return'';
    const vm=window.KoscheiVerdictCard.mapVerdictCard(payload,{lang:'tr'}),h=vm.header;
    const headerMain=h.state==='gathering'?`<div class="vc-hourglass">${esc(h.icon)}</div>`:`<strong>${esc(h.grade||'—')}</strong>`;
    const leverage=vm.leverage.length?vm.leverage.map(row=>`<a class="vc-row red" href="${esc(row.evidence_anchor)}"><span></span><b>${esc(row.text)}</b></a>`).join(''):'<div class="empty compact">Doğrulanmış owner koz satırı yok.</div>';
    return`<article class="card verdict-card ${esc(h.tone)}" id="verdict-card"><div class="vc-header"><div class="vc-grade">${headerMain}</div><div><span class="eyebrow">YATIRIMCI OKUNABİLİR VERDICT CARD</span><h2>${esc(h.title)}</h2><p class="muted">${esc(h.copy)}</p><a class="vc-meta" href="#full-report-detail">Ruleset ${esc(h.ruleset_version)} · imza ${esc(h.signature_short||'—')} · ${esc(h.generated_at||'—')}</a></div></div><div class="vc-block"><h3>${esc(vm.leverage_title)}</h3><div class="vc-list">${leverage}</div></div><div class="vc-block"><h3>${esc(vm.checklist_title)}</h3><div class="vc-list">${vm.checklist.map(row=>`<a class="vc-row ${esc(row.status)}" id="evidence-${esc(row.id)}" href="${esc(row.evidence_anchor)}"><span></span><b>${esc(row.label)}</b><em>${esc(row.value)}</em></a>`).join('')}</div></div><p class="vc-disclaimer">${esc(vm.disclaimer)}</p></article>`;
  }

  function renderVerdict(verdict){
    verdict=obj(verdict);const rules=arr(verdict.triggered_rules),watch=arr(verdict.watch_flags);
    return`<article class="card" style="border-color:#18ffb255"><div class="card-head"><div><span class="eyebrow">TEK RADAR · DETERMINİSTİK FINAL</span><h2>${esc(gradeLabel(verdict.grade))}</h2><p class="muted">${esc(verdict.verdict||'no_grade_trigger')} · ${esc(verdict.ruleset_version||'ruleset yok')}</p></div>${badge(verdict.signed?'verified':'observed',verdict.signed?'İMZALI':'KARAR BEKLİYOR')}</div><div class="warning-box"><b>Sayısal final skor kapalıdır.</b><br>Grade yalnız aşağıdaki açık kurallardan çıkar. AI grade vermez; yalnız kuralları anlatabilir.</div>${rules.length?`<div class="clean-list section-gap">${rules.map(rule=>`<div class="summary-row"><span class="mono">${esc(rule.rule_id)}</span><b style="text-align:left">${esc(rule.title||rule.summary)}<small class="muted">${esc(rule.summary||'')}</small></b>${badge(rule.evidence_status||'observed')}</div>`).join('')}</div>`:'<div class="empty section-gap">Grade değiştiren kural tetiklenmedi. Bu durum güvenli veya A anlamına gelmez.</div>'}${watch.length?`<details class="owner-details section-gap"><summary><span><b>Watch flag</b><small>INFERRED bulgular grade düşürmez.</small></span><span>⌄</span></summary><div class="clean-list section-gap">${watch.map(rule=>`<div class="summary-row"><span class="mono">${esc(rule.rule_id)}</span><b>${esc(rule.summary||rule.title)}</b>${badge('inferred')}</div>`).join('')}</div></details>`:''}<div class="metadata section-gap"><div><label>Ruleset</label><b>${esc(verdict.ruleset_version||'—')}</b></div><div><label>Actor ruleset</label><b>${esc(verdict.actor_ruleset_version||'—')}</b></div><div><label>İmza</label><b class="mono">${esc(short(verdict.signature,48))}</b></div><div><label>Üretim zamanı</label><b>${dt(verdict.generated_at)}</b></div></div></article>`;
  }

  function renderBehavior(report){
    report=obj(report);const signals=arr(report.signals);if(!signals.length)return'';
    return`<article class="card"><div class="card-head"><div><span class="eyebrow">DAVRANIŞ KURALLARI · 4/4</span><h2>Hacim, likidite, creator satış ve holder çıkışı</h2><p class="muted">Ağırlıklı skor yok; her eşik açık ve versiyonludur.</p></div>${badge('observed',`${num(report.triggered_rule_count)} TETİK`)}</div><div class="grid compact-grid">${signals.map(signal=>`<div class="card"><div class="card-head"><div><span class="eyebrow mono">${esc(signal.rule_id)}</span><h3>${esc(signal.title)}</h3></div>${badge(signal.evidence_status,signal.triggered?'TETİKLENDİ':signal.evidence_status)}</div><p>${esc(signal.summary||'')}</p><div class="metadata">${Object.entries(obj(signal.metrics)).slice(0,8).map(([key,value])=>`<div><label>${esc(key.replaceAll('_',' '))}</label><b>${esc(metricValue(value))}</b></div>`).join('')}</div><div class="muted small section-gap">Kapsam: ${esc(signal.scope||'—')}</div>${arr(signal.signatures).length?`<div class="section-gap">${arr(signal.signatures).map(signature=>`<a class="mono" href="https://solscan.io/tx/${encodeURIComponent(signature)}" target="_blank" rel="noopener noreferrer">${esc(short(signature,34))}</a>`).join('<br>')}</div>`:''}${arr(signal.limitations).length?`<div class="warning-box section-gap">${arr(signal.limitations).map(esc).join(' · ')}</div>`:''}</div>`).join('')}</div></article>`;
  }

  function renderThreatAnticipation(report){
    report=obj(report);if(!Object.keys(report).length)return'';
    const exit=obj(report.exit_capacity),rug=obj(report.rug_pathway_assessment),paths=arr(report.pathways),scenarios=arr(report.scenarios),watch=arr(report.watch_signals),missing=arr(report.missing_evidence);
    const multiple=exit.position_liquidity_multiple==null?'—':`${num(exit.position_liquidity_multiple)}x`;
    return`<article class="card" id="threat-anticipation"><div class="card-head"><div><span class="eyebrow">KOSCHEI THREAT ANTICIPATION · ${esc(report.version||'v1')}</span><h2>Risk nasıl gerçekleşebilir?</h2><p class="muted">Niyet tahmini veya sayısal rug olasılığı yok. Koschei kapasiteyi, açık yolları ve bir sonraki doğrulanabilir sinyalleri gösterir.</p></div>${badge(report.status==='evidence_backed_pathway_analysis'?'verified':'observed',report.status==='evidence_backed_pathway_analysis'?'KANITLI YOL ANALİZİ':'KANIT EKSİK')}</div><div class="warning-box"><b>Ana maruziyet</b><br>${esc(report.primary_exposure||'Öncelikli yol belirlenemedi.')}</div><div class="metadata section-gap"><div><label>Baskın owner</label><b>${num(exit.owner_percentage)}%</b></div><div><label>Owner pozisyonu</label><b>${exit.owner_reference_usd_value==null?'—':money(exit.owner_reference_usd_value)}</b></div><div><label>Gözlenen likidite</label><b>${money(exit.liquidity_usd)}</b></div><div><label>Pozisyon / likidite</label><b>${esc(multiple)}</b></div><div><label>Etki kapasitesi</label><b>${esc(String(exit.capacity||'unknown').toUpperCase())}</b></div><div><label>Owner çözümü</label><b>${exit.owner_resolved?'DOĞRULANDI':'EKSİK'}</b></div></div><p class="muted section-gap">${esc(exit.interpretation||'Çıkış kapasitesi hesaplanamadı.')}</p><div class="clean-list section-gap">${paths.map(path=>`<div class="summary-row"><span class="mono">${esc(path.id)}</span><b style="text-align:left">${esc(path.label)}<small class="muted">${esc(path.summary||'')}</small></b>${badge(path.status,threatLabel(path.status))}</div>`).join('')}</div><details class="owner-details section-gap" open><summary><span><b>Rug yolu özeti</b><small>Olasılık puanı değil; teknik yol sınıflandırması.</small></span><span>⌄</span></summary><div class="section-gap"><p>${esc(rug.conclusion||'')}</p>${arr(rug.open_paths).length?`<div class="warning-box"><b>Açık yollar:</b> ${arr(rug.open_paths).map(esc).join(' · ')}</div>`:''}${arr(rug.closed_paths).length?`<div class="warning-box section-gap"><b>Mevcut kanıtta kapalı:</b> ${arr(rug.closed_paths).map(esc).join(' · ')}</div>`:''}</div></details>${scenarios.length?`<details class="owner-details section-gap"><summary><span><b>Muhtemel sonraki davranış yolları</b><small>Bunlar niyet iddiası değil; izlenecek senaryolardır.</small></span><span>⌄</span></summary><div class="clean-list section-gap">${scenarios.map(item=>`<div class="summary-row"><span class="mono">${esc(item.id)}</span><b style="text-align:left">${esc(item.title)}<small class="muted">${esc(item.basis||'')}</small><small>${arr(item.next_signals).map(esc).join(' · ')}</small></b>${badge(item.evidence_status||'inferred')}</div>`).join('')}</div></details>`:''}${watch.length?`<details class="owner-details section-gap"><summary><span><b>Erken uyarı emirleri</b><small>Snapshot ve işlem kanıtıyla takip edilecek tetikler.</small></span><span>⌄</span></summary><div class="clean-list section-gap">${watch.map(item=>`<div class="summary-row"><span class="mono">${esc(item.severity)}</span><b style="text-align:left">${esc(item.title)}<small class="muted">${esc(item.trigger)}</small></b>${badge(item.status||'observed')}</div>`).join('')}</div></details>`:''}${missing.length?`<div class="warning-box section-gap"><b>Karar için eksik deliller:</b><br>${missing.map(esc).join(' · ')}</div>`:''}</article>`;
  }

  function renderLegacy(legacy){
    legacy=obj(legacy);if(legacy.applicable===false)return`<article class="card"><div class="card-head"><div><span class="eyebrow">ARVIS ARAŞTIRMA YETENEKLERİ</span><h2>Bu hedefte uygulanamaz</h2></div>${badge('observed','N/A')}</div><div class="warning-box">${esc(legacy.reason||'Token mint gereklidir.')}</div></article>`;
    const modules=arr(legacy.modules),holders=obj(legacy.holder_intelligence),market=obj(legacy.market),source=obj(legacy.source_context),coverage=buildInvestigationCoverage(legacy);
    return`<article class="card"><div class="card-head"><div><span class="eyebrow">ARVIS ARAŞTIRMA YETENEKLERİ · TEK DOSYADA</span><h2>${num(coverage.evidence_producing)}/${num(coverage.capability_total)} kol kanıt üretti</h2><p class="muted">Tamamlanan, bulgu üretmeyen, uygulanamaz ve eksik collector sonuçları ayrı gösterilir. Mimari sayı çalışma sonucu değildir.</p></div>${badge(coverage.status,coverage.status==='complete_investigation'?'TAM KAPSAM':'KISMİ KAPSAM')}</div><div class="grid compact-grid">${['volume_24h_usd','liquidity_usd','market_cap_usd'].map(key=>`<div class="card kpi"><div class="kpi-label">${esc(key.replaceAll('_',' '))}</div><div class="kpi-value tone-cyan">${money(market[key])}</div></div>`).join('')}<div class="card kpi"><div class="kpi-label">Top owner</div><div class="kpi-value tone-amber">${num(holders.top_owner_percentage)}%</div></div><div class="card kpi"><div class="kpi-label">Creator</div><div class="kpi-value mono" style="font-size:13px">${esc(short(source.creator_wallet,28))}</div></div></div><div class="clean-list section-gap">${modules.map(module=>`<details class="owner-details module-drawer"><summary><span><b>${esc(module.module||module.module_id)}</b><small>${esc(module.verdict||'Collector sonucu')}</small></span>${moduleStatusBadge(module)}</summary><div class="section-gap">${arr(module.evidence).length?`<div class="clean-list">${arr(module.evidence).map((line,index)=>`<div class="summary-row"><span>E${index+1}</span><b style="text-align:left">${esc(line)}</b>${moduleStatusBadge(module)}</div>`).join('')}</div>`:'<div class="empty compact">Collector bu kapsamda kanıt satırı üretmedi.</div>'}</div></details>`).join('')}</div></article>`;
  }

  function renderActor(root,payload){
    const actor=obj(payload.actor_investigation),dossier=obj(actor.dossier);if(!Object.keys(dossier).length)return;
    const holder=document.createElement('div');holder.className='section-gap';root.appendChild(holder);
    if(typeof kit.renderDefense==='function'){kit.renderDefense(holder,{schema_version:'koschei-actor-defense-v3',wallet:actor.wallet,dossier,rule_verdict:actor.rule_verdict,funding_origin:actor.funding_origin});return;}
    holder.innerHTML=`<article class="card"><div class="card-head"><div><span class="eyebrow">ACTOR INVESTIGATION</span><h2>${esc(short(actor.wallet,40))}</h2></div>${badge(obj(dossier.track).state||'observed')}</div></article>`;
  }

  function renderUnified(root,payload){
    root=rootFor(root);if(!root)return;
    root.innerHTML=`${renderInvestigationCoverage(payload.legacy_14_arm_radar)}${renderVerdictCard(payload)}<div class="grid compact-grid section-gap" id="full-report-detail"><div class="span-12">${renderVerdict(payload.final_verdict)}</div><div class="span-12">${renderBehavior(payload.behavior_signals)}</div><div class="span-12">${renderThreatAnticipation(payload.threat_anticipation)}</div><div class="span-12">${renderLegacy(payload.legacy_14_arm_radar)}</div></div>`;
    renderActor(root,payload);
  }

  async function scan(target,rootId){
    const root=rootFor(rootId);if(!root)throw new Error('Radar sonuç alanı bulunamadı.');
    root.innerHTML='<div class="card loading">ARVIS, 14 araştırma yeteneğinin uygulanabilir collectorlarını çalıştırıyor; tamamlanan, uygulanamaz ve eksik sonuçlar ayrı raporlanacak…</div>';
    const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),210000);
    try{
      const response=await fetch('/api/owner/radar/unified',{method:'POST',credentials:'same-origin',signal:controller.signal,headers:{'Content-Type':'application/json'},body:JSON.stringify({target,network:'solana-mainnet',live_evidence:true})});
      let data={};try{data=await response.json()}catch{}
      if(!response.ok||data.ok===false)throw new Error(data.message||data.detail||data.error||`İstek başarısız (${response.status})`);
      renderUnified(root,data);return data;
    }catch(error){
      const message=error?.name==='AbortError'?'Tek Radar 210 saniyede tamamlanamadı.':(error?.message||'Tek Radar başarısız oldu.');
      root.innerHTML=`<div class="card error-state"><div><b>Tek Radar tamamlanamadı.</b><span>${esc(message)}</span></div></div>`;throw error;
    }finally{clearTimeout(timer)}
  }

  window.OwnerRadarKit={...kit,scan,renderUnified};
})();
