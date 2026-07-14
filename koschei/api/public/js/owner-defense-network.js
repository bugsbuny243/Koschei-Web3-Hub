(()=>{
  'use strict';

  const kit=window.OwnerRadarKit;
  if(!kit||window.__ownerDefenseNetworkInstalled)return;
  window.__ownerDefenseNetworkInstalled=true;
  const originalScan=kit.scan.bind(kit);
  const originalRender=kit.render.bind(kit);
  const esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  const arr=value=>Array.isArray(value)?value:[];
  const obj=value=>value&&typeof value==='object'&&!Array.isArray(value)?value:{};
  const num=value=>new Intl.NumberFormat('tr-TR',{maximumFractionDigits:6}).format(Number(value||0));
  const dt=value=>{if(!value)return'—';const date=new Date(value);return Number.isNaN(date.getTime())?'—':new Intl.DateTimeFormat('tr-TR',{dateStyle:'short',timeStyle:'short'}).format(date)};
  const short=(value,length=36)=>{const text=String(value||'');return text.length>length?`${text.slice(0,length-10)}…${text.slice(-7)}`:text||'—'};
  const rootFor=value=>typeof value==='string'?document.getElementById(value):value;
  const stateLabel=value=>({detected:'TESPİT EDİLDİ',tracked:'TAKİPTE',correlated:'İLİŞKİLENDİRİLDİ',verified:'DOĞRULANDI',alerted:'UYARI ÜRETİLDİ'}[String(value||'').toLowerCase()]||String(value||'BİLİNMİYOR').toUpperCase());
  const actionLabel=value=>({monitor_sensor_memory:'Sensör hafızasında izle',review_verified_evidence:'Doğrulanmış kanıtı incele',verify_creator_funding_and_transfer_chain:'Creator funding ve transfer zincirini doğrula',collect_live_transaction_evidence:'Canlı transaction kanıtı topla',expand_cross_token_holder_network:'Cross-token holder ağını genişlet',expand_creator_token_history:'Creator token geçmişini genişlet'}[String(value||'')]||String(value||'İncele').replaceAll('_',' '));
  const statusClass=value=>String(value||'').toLowerCase()==='verified'?'ok':String(value||'').toLowerCase()==='alerted'?'bad':'warn';
  const badge=(value,label=value)=>`<span class="badge ${statusClass(value)}">${esc(label)}</span>`;
  const kpi=(label,value,foot)=>`<article class="card kpi"><div class="kpi-top"><div><div class="kpi-label">${esc(label)}</div><div class="kpi-value tone-cyan">${esc(value)}</div></div></div><div class="kpi-foot">${esc(foot)}</div></article>`;

  function renderTokens(tokens){
    if(!tokens.length)return'<div class="empty">Koschei sensörlerinde bu cüzdana bağlı token gözlemi yok.</div>';
    return`<div class="table-wrap"><table class="table"><thead><tr><th>Mint</th><th>Roller</th><th>Holder</th><th>Pump işlemleri</th><th>İlk / son gözlem</th></tr></thead><tbody>${tokens.map(token=>`<tr><td><b>${esc(token.symbol||token.name||'Token')}</b><div class="mono">${esc(short(token.mint,34))}</div></td><td>${arr(token.roles).map(role=>badge('observed',String(role).replaceAll('_',' ').toUpperCase())).join(' ')||'—'}</td><td>${token.holder_rank?`#${num(token.holder_rank)} · ${num(token.holder_percentage)}%`:'—'}</td><td>${num(token.buy_count)} alım · ${num(token.sell_count)} satış<div class="muted small">${num(token.sol_bought)} SOL alım · ${num(token.sol_sold)} SOL satış</div></td><td>${dt(token.first_observed_at)}<br><span class="muted small">${dt(token.last_observed_at)}</span></td></tr>`).join('')}</tbody></table></div>`;
  }

  function renderActors(actors){
    if(!actors.length)return'<div class="empty">Bu token kümesinde tekrar eden owner-resolved holder henüz gözlemlenmedi.</div>';
    return`<div class="table-wrap"><table class="table"><thead><tr><th>Cüzdan</th><th>Ortak token</th><th>En yüksek holder payı</th><th>Gözlem penceresi</th></tr></thead><tbody>${actors.map(actor=>`<tr><td class="mono">${esc(short(actor.wallet,38))}</td><td><b>${num(actor.shared_token_count)}</b></td><td><b>${num(actor.max_holder_percentage)}%</b></td><td>${dt(actor.first_observed_at)} → ${dt(actor.last_observed_at)}</td></tr>`).join('')}</tbody></table></div>`;
  }

  function renderEvidence(evidence){
    if(!evidence.length)return'<div class="warning-box"><b>Doğrudan transaction bağlantısı henüz kaydedilmedi.</b><br>Bu, bağlantı olmadığı anlamına gelmez; yalnız sorgulanan imza penceresinde VERIFIED veya OBSERVED kanıt oluşmadığını gösterir.</div>';
    return`<div class="table-wrap"><table class="table"><thead><tr><th>Durum</th><th>İlişki</th><th>Karşı taraf</th><th>Miktar</th><th>İmza / zaman</th></tr></thead><tbody>${evidence.map(item=>`<tr><td>${badge(item.verification_status,String(item.verification_status||'observed').toUpperCase())}</td><td><b>${esc(String(item.relation||'').replaceAll('_',' ').toUpperCase())}</b><div class="muted small">${esc(item.source||'koschei')}</div></td><td><span class="mono">${esc(short(item.counterpart_id,34))}</span><div class="muted small">${esc(item.counterpart_kind||'')}</div></td><td>${item.amount_native?`${num(item.amount_native)} SOL`:item.token_amount?`${num(item.token_amount)} token`:'—'}${item.token_mint?`<div class="mono muted small">${esc(short(item.token_mint,25))}</div>`:''}</td><td>${item.signature?`<a class="mono" href="https://solscan.io/tx/${encodeURIComponent(item.signature)}" target="_blank" rel="noopener noreferrer">${esc(short(item.signature,30))}</a>`:'—'}<div class="muted small">${dt(item.observed_at)} · slot ${num(item.slot)}</div></td></tr>`).join('')}</tbody></table></div>`;
  }

  function renderDefense(root,payload){
    root=rootFor(root);
    if(!root)return;
    const dossier=obj(payload.dossier),track=obj(dossier.track),coverage=obj(dossier.coverage),live=obj(coverage.live_evidence);
    const tokens=arr(dossier.tokens),actors=arr(dossier.related_actors),evidence=arr(dossier.evidence);
    root.innerHTML=`<div class="card" style="border-color:#18ffb255"><div class="card-head"><div><span class="eyebrow">KOSCHEI DEFENSE NETWORK · ACTOR DOSSIER</span><h2>${esc(stateLabel(track.state))}</h2><div class="mono muted">${esc(dossier.wallet||payload.wallet||payload.target)}</div></div>${badge(track.state==='verified'?'verified':'observed',stateLabel(track.state))}</div><div class="grid compact-grid">${kpi('Creator token',num(track.created_token_count),'Pump discovery / deployer gözlemi')}${kpi('Baskın holder',num(track.dominant_holder_token_count),'Owner-resolved Top-5 snapshot')}${kpi('İşlem gördüğü token',num(track.traded_token_count),'Pump trade ledger')}${kpi('İlişkili aktör',num(track.related_actor_count),'Aynı token kümesinde tekrar')}${kpi('Verified kanıt',num(track.verified_evidence_count),'Parsed transfer instruction')}${kpi('Observed kanıt',num(track.observed_evidence_count),'Sınırı açık davranış gözlemi')}</div><div class="metadata section-gap"><div><label>Track ID</label><b class="mono">${esc(short(track.id,34))}</b></div><div><label>Durum</label><b>${esc(stateLabel(track.state))}</b></div><div><label>Son soruşturma</label><b>${dt(track.last_investigated_at)}</b></div><div><label>Canlı RPC</label><b>${esc(String(live.status||'stored_evidence_only').toUpperCase())}</b></div><div><label>Parsed transaction</label><b>${num(live.transactions_parsed)}</b></div><div><label>Yeni kanıt</label><b>${num(live.evidence_persisted)}</b></div></div><details class="owner-details section-gap" open><summary><span><b>Token operasyon yüzeyi</b><small>Creator/deployer, baskın holder ve Pump trader rolleri ayrı tutulur.</small></span><span>⌄</span></summary><div class="section-gap">${renderTokens(tokens)}</div></details><details class="owner-details section-gap" open><summary><span><b>Cross-token aktör korelasyonu</b><small>Aynı token kümesinde yeniden görünen owner-resolved cüzdanlar.</small></span><span>⌄</span></summary><div class="section-gap">${renderActors(actors)}</div></details><details class="owner-details section-gap" open><summary><span><b>Transaction kanıt günlüğü</b><small>İmza, slot, yön ve token-account owner çözümlemesi.</small></span><span>⌄</span></summary><div class="section-gap">${renderEvidence(evidence)}</div></details><div class="warning-box section-gap"><b>Kanıt politikası</b><br>Risk skoru üretilmez. VERIFIED yalnız parsed transfer instruction veya owner-resolved zincir kanıtıyla kullanılır. Cüzdan ilişkisi gerçek kişi kimliği ya da kötü niyet iddiası değildir.${arr(live.limitations).length?`<br><br>${arr(live.limitations).map(esc).join(' · ')}`:''}</div></div>`;
  }

  async function investigate(target,rootId){
    const root=rootFor(rootId);
    if(!root)throw new Error('Tarama alanı bulunamadı.');
    root.innerHTML='<div class="card loading">Koschei Defense Network creator, holder, trade ve transaction sensörlerini ilişkilendiriyor…</div>';
    const controller=new AbortController();
    const timer=setTimeout(()=>controller.abort(),180000);
    try{
      const response=await fetch('/api/owner/defense/investigate',{method:'POST',credentials:'same-origin',signal:controller.signal,headers:{'Content-Type':'application/json'},body:JSON.stringify({target,network:'solana-mainnet',live_evidence:true})});
      let data={};
      try{data=await response.json()}catch{}
      if(!response.ok||data.ok===false)throw new Error(data.message||data.detail||data.error||`İstek başarısız (${response.status})`);
      renderDefense(root,data);
      loadDefenseQueue();
      return data;
    }catch(error){
      const message=error?.name==='AbortError'?'Actor investigation 180 saniyede tamamlanamadı.':(error?.message||'Actor investigation başarısız oldu.');
      root.innerHTML=`<div class="card error-state"><div><b>Actor investigation tamamlanamadı.</b><span>${esc(message)}</span></div></div>`;
      throw error;
    }finally{clearTimeout(timer)}
  }

  async function scan(target,rootId){
    try{return await originalScan(target,rootId)}catch(error){
      if(Number(error?.status)!==422)throw error;
      return investigate(target,rootId);
    }
  }

  function queueTable(items){
    if(!items.length)return'<div class="empty">Henüz kalıcı actor track oluşmadı. Sensör korelasyonu veri geldikçe kuyruğu otomatik doldurur.</div>';
    return`<div class="table-wrap"><table class="table"><thead><tr><th>Öncelik</th><th>Wallet / durum</th><th>Tekrar yüzeyi</th><th>Kanıt</th><th>Sonraki doğrulama</th><th>İşlem</th></tr></thead><tbody>${items.map(item=>{const track=obj(item.track),reasons=arr(item.priority_reasons);return`<tr><td><b style="font-size:22px">${num(item.verification_priority)}</b><div class="muted small">operasyon önceliği</div></td><td><span class="mono">${esc(short(track.target_id,35))}</span><div class="section-gap">${badge(track.state,stateLabel(track.state))}${item.needs_live_evidence?' '+badge('observed','CANLI KANIT GEREKLİ'):''}</div></td><td>${num(track.created_token_count)} creator · ${num(track.dominant_holder_token_count)} baskın holder<div class="muted small">${num(track.related_actor_count)} ilişkili aktör · ${num(track.traded_token_count)} işlem tokenı</div></td><td>${num(track.verified_evidence_count)} verified · ${num(track.observed_evidence_count)} observed<div class="muted small">${reasons.slice(0,3).map(esc).join(' · ')||'—'}</div></td><td><b>${esc(actionLabel(item.next_action))}</b><div class="muted small">Son gözlem ${dt(track.last_seen_at)}</div></td><td><button class="btn small" type="button" data-defense-investigate="${esc(track.target_id)}">Dossier aç</button></td></tr>`}).join('')}</tbody></table></div>`;
  }

  function renderDefenseQueue(payload){
    const panel=document.getElementById('ownerDefenseQueuePanel');
    if(!panel)return;
    const queue=obj(payload.queue),counts=obj(queue.counts),items=arr(queue.items);
    panel.innerHTML=`<div class="card-head"><div><span class="eyebrow">KOSCHEI DEFENSE NETWORK · CANLI THREAT QUEUE</span><h2>Actor doğrulama sırası</h2><p class="muted">Bu değer risk skoru değildir. Kalıcı tekrar, kanıt kapsamı ve gözlem tazeliğine göre hangi wallet’ın önce doğrulanacağını belirler.</p></div><div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap"><span class="badge ok">${num(queue.total)} track</span><button class="btn small" type="button" data-defense-queue-refresh>Yenile</button></div></div><div class="grid compact-grid">${kpi('İlişkilendirildi',num(counts.correlated),'Cross-token tekrar')}${kpi('Doğrulandı',num(counts.verified),'Transaction kanıtı')}${kpi('Takipte',num(counts.tracked),'Tekrar gözlemi')}${kpi('Uyarı',num(counts.alerted),'Owner alert state')}</div><div class="section-gap">${queueTable(items)}</div><div class="warning-box section-gap"><b>Operasyon politikası</b><br>Kuyruk otomatik korelasyon hafızasından gelir. Priority bir suçlama veya güvenlik hükmü değildir; VERIFIED state için canlı zincir kanıtı gerekir. Son güncelleme: ${esc(dt(queue.generated_at))}</div>`;
    bindDefenseQueueActions();
  }

  function bindDefenseQueueActions(){
    document.querySelectorAll('[data-defense-queue-refresh]').forEach(button=>button.onclick=()=>loadDefenseQueue(true));
    document.querySelectorAll('[data-defense-investigate]').forEach(button=>button.onclick=async()=>{
      const wallet=button.dataset.defenseInvestigate;
      const input=document.getElementById('ownerRadarTarget');
      if(input)input.value=wallet;
      const result=document.getElementById('ownerRadarResult');
      result?.scrollIntoView({behavior:'smooth',block:'start'});
      button.disabled=true;
      try{await investigate(wallet,'ownerRadarResult')}finally{button.disabled=false}
    });
  }

  let queueRequest=0;
  async function loadDefenseQueue(force=false){
    const panel=document.getElementById('ownerDefenseQueuePanel');
    if(!panel)return;
    if(panel.dataset.loading==='true'&&!force)return;
    panel.dataset.loading='true';
    const request=++queueRequest;
    if(!panel.dataset.loaded)panel.innerHTML='<div class="card loading">Kalıcı actor track’leri ve doğrulama önceliği yükleniyor…</div>';
    const controller=new AbortController();
    const timer=setTimeout(()=>controller.abort(),30000);
    try{
      const response=await fetch('/api/owner/defense/tracks?network=solana-mainnet&limit=50',{credentials:'same-origin',signal:controller.signal});
      let data={};
      try{data=await response.json()}catch{}
      if(!response.ok||data.ok===false)throw new Error(data.message||data.detail||data.error||`İstek başarısız (${response.status})`);
      if(request!==queueRequest)return;
      panel.dataset.loaded='true';
      renderDefenseQueue(data);
    }catch(error){
      if(request!==queueRequest)return;
      const message=error?.name==='AbortError'?'Threat queue 30 saniyede yüklenemedi.':(error?.message||'Threat queue yüklenemedi.');
      panel.innerHTML=`<div class="card error-state"><div><b>Actor doğrulama kuyruğu yüklenemedi.</b><span>${esc(message)}</span></div><button class="btn small" type="button" data-defense-queue-refresh>Tekrar dene</button></div>`;
      bindDefenseQueueActions();
    }finally{
      clearTimeout(timer);
      if(request===queueRequest)panel.dataset.loading='false';
    }
  }

  function ensureDefenseQueuePanel(){
    const root=document.getElementById('arvisContent');
    const grid=root?.querySelector(':scope > .grid.compact-grid');
    if(!grid||document.getElementById('ownerDefenseQueuePanel'))return;
    const panel=document.createElement('article');
    panel.id='ownerDefenseQueuePanel';
    panel.className='card span-12';
    const anchor=grid.children[1]||null;
    grid.insertBefore(panel,anchor);
    loadDefenseQueue();
  }

  const arvisRoot=document.getElementById('arvisContent');
  if(arvisRoot){
    new MutationObserver(()=>ensureDefenseQueuePanel()).observe(arvisRoot,{childList:true});
    ensureDefenseQueuePanel();
  }

  window.OwnerRadarKit={...kit,scan,investigate,loadDefenseQueue,render:(root,data)=>data?.schema_version==='koschei-actor-defense-v1'?renderDefense(root,data):originalRender(root,data),renderDefense};
})();
