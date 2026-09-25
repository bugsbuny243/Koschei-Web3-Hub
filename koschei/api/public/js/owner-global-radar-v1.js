(()=>{
'use strict';
if(window.__ownerGlobalRadarV1)return;
window.__ownerGlobalRadarV1=true;
const esc=value=>String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
const arr=value=>Array.isArray(value)?value:[];
const short=(value,length=38)=>{const text=String(value||'');return text.length>length?`${text.slice(0,length-10)}…${text.slice(-7)}`:text||'—'};
const dt=value=>{if(!value)return'—';const date=new Date(value);return Number.isNaN(date.getTime())?'—':new Intl.DateTimeFormat('tr-TR',{dateStyle:'short',timeStyle:'short'}).format(date)};
const stateTone=value=>['verified','observed'].includes(String(value||'').toLowerCase())?'ok':'warn';
const badge=(value,label=value)=>`<span class="badge ${stateTone(value)}">${esc(String(label||'UNKNOWN').toUpperCase())}</span>`;

function panelMarkup(){
  return `<article class="card span-12" id="ownerGlobalRadarPanel"><div class="card-head"><div><span class="eyebrow">GLOBAL CRYPTO RADAR · PERSISTED EVIDENCE</span><h2>Multi-network graph & event timeline</h2><p class="muted">ClickHouse'ta saklanan tarihsel kanıtı okur. Bu görünüm mevcut zincir durumunu veya canlı collector sağlığını iddia etmez.</p></div><span class="badge warn">HISTORICAL EVIDENCE</span></div><form id="ownerGlobalRadarForm" class="filters" style="display:grid;grid-template-columns:220px minmax(240px,1fr) 150px auto;gap:9px"><select class="input" id="ownerGlobalRadarNetwork" required><option value="">Ağ yükleniyor…</option></select><input class="input mono" id="ownerGlobalRadarSubject" placeholder="subject_id (opsiyonel)"><select class="input" id="ownerGlobalRadarWindow"><option value="1">Son 1 saat</option><option value="6">Son 6 saat</option><option value="24" selected>Son 24 saat</option><option value="72">Son 72 saat</option><option value="168">Son 7 gün</option></select><button class="btn primary" type="submit" id="ownerGlobalRadarRun">Kanıtı yükle</button></form><div id="ownerGlobalRadarStatus" class="muted small section-gap">Ağ seç ve tarihsel graph/event ledger kanıtını yükle.</div><div id="ownerGlobalRadarResult" class="section-gap"><div class="empty">Henüz sorgu çalıştırılmadı.</div></div></article>`;
}

async function fetchJSON(url){
  const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),25000);
  try{
    const response=await fetch(url,{credentials:'same-origin',cache:'no-store',signal:controller.signal});
    const data=await response.json().catch(()=>({}));
    if(!response.ok)throw new Error(data.error||data.message||`HTTP ${response.status}`);
    return data;
  }finally{clearTimeout(timer)}
}

async function populateNetworks(){
  const select=document.getElementById('ownerGlobalRadarNetwork');if(!select)return;
  try{
    const registry=await fetchJSON('/fabric/security-center/capabilities');
    const networks=arr(registry.networks);
    select.innerHTML='<option value="">Network seç</option>'+networks.map(network=>`<option value="${esc(network.id)}">${esc(network.name)} · ${esc(network.family)}</option>`).join('');
  }catch(error){
    select.innerHTML='<option value="solana-mainnet">Solana</option><option value="ethereum-mainnet">Ethereum</option><option value="bitcoin-mainnet">Bitcoin</option>';
  }
}

function typeCounts(items,key){
  const counts=new Map();for(const item of items){const value=String(item?.[key]||'unknown');counts.set(value,(counts.get(value)||0)+1);}return [...counts.entries()].sort((a,b)=>b[1]-a[1]);
}

function renderRecords(records){
  if(!records.length)return'<div class="empty compact">Bu pencerede persisted graph record yok.</div>';
  return `<div class="table-wrap"><table class="table"><thead><tr><th>Zaman</th><th>Tip / kind</th><th>Subject</th><th>Evidence</th><th>Payload hash</th></tr></thead><tbody>${records.slice(0,100).map(row=>`<tr><td>${esc(dt(row.observed_at||row.recorded_at))}</td><td><b>${esc(row.record_type||'—')}</b><div class="muted small">${esc(row.record_kind||'')}</div></td><td class="mono">${esc(short(row.subject_id,34))}${row.target_subject_id?`<br><span class="muted">→ ${esc(short(row.target_subject_id,30))}</span>`:''}</td><td>${badge(row.evidence_status)}</td><td class="mono">${esc(short(row.payload_sha256,30))}</td></tr>`).join('')}</tbody></table></div>`;
}

function renderEvents(events){
  if(!events.length)return'<div class="empty compact">Bu pencerede canonical event yok.</div>';
  return `<div class="table-wrap"><table class="table"><thead><tr><th>Observed</th><th>Kind</th><th>Subject</th><th>Evidence</th><th>Producer / digest</th></tr></thead><tbody>${events.slice(0,100).map(row=>{const event=row.event||{};return`<tr><td>${esc(dt(row.observed_at))}</td><td><b>${esc(event.kind||'—')}</b><div class="muted small">${esc(event.subject_kind||'')}</div></td><td class="mono">${esc(short(event.subject_id,34))}</td><td>${badge(event.evidence_state)}</td><td>${esc(event.producer||'—')}<div class="mono muted small">${esc(short(event.event_sha256,30))}</div></td></tr>`}).join('')}</tbody></table></div>`;
}

function render(recordsPayload,eventsPayload){
  const root=document.getElementById('ownerGlobalRadarResult');if(!root)return;
  const records=arr(recordsPayload.records),events=arr(eventsPayload.events),recordCounts=typeCounts(records,'record_type'),eventCounts=typeCounts(events.map(row=>row.event||{}),'kind');
  root.innerHTML=`<div class="grid compact-grid"><article class="card kpi"><div class="kpi-label">Graph records</div><div class="kpi-value tone-cyan">${records.length}</div><div class="kpi-foot">Payload SHA-256 reverified</div></article><article class="card kpi"><div class="kpi-label">Canonical events</div><div class="kpi-value tone-cyan">${events.length}</div><div class="kpi-foot">Event digest reverified</div></article><article class="card kpi"><div class="kpi-label">Record classes</div><div class="kpi-value tone-green">${recordCounts.length}</div><div class="kpi-foot">${esc(recordCounts.map(([k,v])=>`${k}:${v}`).join(' · ')||'none')}</div></article><article class="card kpi"><div class="kpi-label">Event classes</div><div class="kpi-value tone-green">${eventCounts.length}</div><div class="kpi-foot">${esc(eventCounts.map(([k,v])=>`${k}:${v}`).join(' · ')||'none')}</div></article><article class="card span-12"><div class="card-head"><div><span class="eyebrow">GRAPH RECORDS</span><h2>Persisted relationship/evidence records</h2></div></div>${renderRecords(records)}</article><article class="card span-12"><div class="card-head"><div><span class="eyebrow">CANONICAL EVENT LEDGER</span><h2>Native-digest event timeline</h2></div></div>${renderEvents(events)}</article><article class="card span-12"><div class="warning-box"><b>Truth boundary</b><br>Persisted historical evidence = YES · Current chain state = NO · Missing evidence policy = UNKNOWN. Yeni/current durum gerekiyorsa ilgili native probe tekrar çalıştırılmalıdır.</div></article></div>`;
}

async function run(){
  const network=document.getElementById('ownerGlobalRadarNetwork')?.value||'';if(!network)return;
  const subject=(document.getElementById('ownerGlobalRadarSubject')?.value||'').trim();const hours=Math.max(1,Math.min(744,Number(document.getElementById('ownerGlobalRadarWindow')?.value||24)));
  const until=new Date(),since=new Date(until.getTime()-hours*3600_000),query=new URLSearchParams({network,since:since.toISOString(),until:until.toISOString(),limit:'200'});if(subject)query.set('subject_id',subject);
  const status=document.getElementById('ownerGlobalRadarStatus'),button=document.getElementById('ownerGlobalRadarRun'),root=document.getElementById('ownerGlobalRadarResult');button.disabled=true;status.textContent='Persisted graph ve canonical event ledger okunuyor…';root.innerHTML='<div class="card loading">Global Radar historical evidence yükleniyor…</div>';
  try{
    const [records,events]=await Promise.all([fetchJSON('/api/owner/radar/global/records?'+query),fetchJSON('/api/owner/radar/global/events?'+query)]);render(records,events);status.textContent=`${network} · ${hours}s window · persisted historical evidence`;
  }catch(error){root.innerHTML=`<div class="error-box"><b>Global Radar persisted evidence okunamadı.</b><br>${esc(error?.message||'unknown error')}<br><span class="muted">ClickHouse reader kapalıysa bu bir güvenlik sonucu değil, deployment bağımlılığı durumudur.</span></div>`;status.textContent='Reader unavailable / query failed.';
  }finally{button.disabled=false;}
}

function ensurePanel(){
  const root=document.getElementById('securityContent');if(!root||!root.children.length||document.getElementById('ownerGlobalRadarPanel'))return;
  const grid=root.querySelector(':scope > .grid.compact-grid')||root;grid.insertAdjacentHTML('beforeend',panelMarkup());document.getElementById('ownerGlobalRadarForm')?.addEventListener('submit',event=>{event.preventDefault();run();});populateNetworks();
}

const root=document.getElementById('securityContent');if(root){new MutationObserver(ensurePanel).observe(root,{childList:true});ensurePanel();}
})();
