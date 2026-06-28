(()=>{
'use strict';

const NAV=[
  {id:'command',icon:'⌂',label:'Özet',title:'Owner Operasyon Merkezi',eyebrow:'Üretim operasyonları'},
  {id:'arvis',icon:'◉',label:'ARVIS',title:'ARVIS Radar',eyebrow:'Canlı risk altyapısı'},
  {id:'customers',icon:'◎',label:'Müşteriler',title:'Müşteriler',eyebrow:'Üyelik ve erişim'},
  {id:'revenue',icon:'₺',label:'Gelir',title:'Gelir ve Paketler',eyebrow:'Shopier ve entitlement'},
  {id:'feedback',icon:'✦',label:'Geri Bildirim',title:'Geri Bildirim',eyebrow:'Müşteri sinyalleri'},
  {id:'security',icon:'◇',label:'Güvenlik',title:'Güvenlik Olayları',eyebrow:'Denetim kayıtları'},
  {id:'system',icon:'⚙',label:'Sistem',title:'Sistem Sağlığı',eyebrow:'Servis durumu'},
  {id:'brain',icon:'◆',label:'Brain',title:'Owner Brain',eyebrow:'Operasyon copilotu'}
];

const state={
  active:'command',dashboard:null,users:[],payments:[],paymentHealth:null,
  feedback:[],security:[],radarSources:[],loading:false
};

const $=id=>document.getElementById(id);
const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const num=v=>new Intl.NumberFormat('tr-TR').format(Number(v||0));
const moneyCents=v=>new Intl.NumberFormat('tr-TR',{style:'currency',currency:'TRY',maximumFractionDigits:2}).format(Number(v||0)/100);
const moneyTRY=v=>new Intl.NumberFormat('tr-TR',{style:'currency',currency:'TRY',maximumFractionDigits:2}).format(Number(v||0));
const dt=v=>{if(!v)return'—';const d=new Date(v);return Number.isNaN(d.getTime())?'—':new Intl.DateTimeFormat('tr-TR',{dateStyle:'short',timeStyle:'short'}).format(d)};
const short=(v,n=32)=>{v=String(v||'');return v.length>n?v.slice(0,Math.max(8,n-10))+'…'+v.slice(-7):v||'—'};
const sum=(items,key)=>items.reduce((a,x)=>a+Number(x?.[key]||0),0);

const STATUS_LABELS={
  healthy:'Sağlıklı',connected:'Bağlı',configured:'Yapılandırıldı',active:'Aktif',success:'Başarılı',
  enabled:'Açık',ok:'Hazır',live:'Canlı',ready:'Hazır',processing:'İşleniyor',completed:'Tamamlandı',
  pending:'Bekliyor',manual:'Manuel',reviewing:'İnceleniyor',planned:'Planlandı',resolved:'Çözüldü',closed:'Kapalı',
  disabled:'Kapalı',waiting:'Bekliyor',unknown:'Bilinmiyor',missing:'Eksik',unavailable:'Kullanılamıyor',
  degraded:'Zayıfladı',stale:'Güncel değil',failed:'Başarısız',error:'Hata',critical:'Kritik',fatal:'Kritik',
  retryable:'Yeniden denenebilir',exhausted:'Tükendi',banned:'Yasaklı',removed:'Kaldırıldı',new:'Yeni',
  approved:'Onaylandı',rejected:'Reddedildi',none:'Yok'
};

async function api(path,opt={}){
  const headers=new Headers(opt.headers||{});
  if(opt.body&&!headers.has('Content-Type'))headers.set('Content-Type','application/json');
  const controller=new AbortController();
  const timeout=setTimeout(()=>controller.abort(),20000);
  try{
    const response=await fetch(path,{credentials:'same-origin',...opt,headers,signal:controller.signal});
    let data={};try{data=await response.json()}catch{}
    if(!response.ok||data.ok===false||data.success===false){
      const error=new Error(data.message||data.detail||data.error||`İstek başarısız (${response.status})`);
      error.status=response.status;error.data=data;throw error;
    }
    return data;
  }catch(error){
    if(error.name==='AbortError')throw new Error('İstek zaman aşımına uğradı.');
    throw error;
  }finally{clearTimeout(timeout)}
}

function toast(message,bad=false){
  const el=$('toast');if(!el)return;
  el.textContent=message;el.className=`toast show${bad?' bad':''}`;
  clearTimeout(toast.timer);toast.timer=setTimeout(()=>el.className='toast',3800);
}
function statusValue(raw){if(typeof raw==='string')return raw;if(raw&&typeof raw==='object')return raw.status||raw.state||(raw.configured===true?'configured':'unknown');return'unknown'}
function tone(value){const s=String(value||'').toLowerCase();if(['healthy','connected','configured','active','success','enabled','ok','live','ready','completed','approved','resolved'].some(x=>s.includes(x)))return'ok';if(['failed','error','critical','fatal','unavailable','banned','degraded','stale','missing','rejected','exhausted'].some(x=>s.includes(x)))return'bad';return'warn'}
function statusLabel(value){const raw=String(value||'unknown').toLowerCase().replaceAll('_',' ');return STATUS_LABELS[raw]||raw.replace(/\b\w/g,c=>c.toUpperCase())}
function badge(value){return`<span class="badge ${tone(value)}">${esc(statusLabel(value))}</span>`}
function setSync(text,kind='warn'){const dot=$('syncDot');if(dot)dot.className='dot'+(kind==='bad'?' bad':kind==='ok'?'':' warn');if($('syncText'))$('syncText').textContent=text}
function showLogin(message=''){$('loginView').classList.remove('hidden');$('appView').classList.add('hidden');const box=$('loginError');if(message){box.textContent=message;box.classList.remove('hidden')}else{box.textContent='';box.classList.add('hidden')}}
function showApp(){$('loginView').classList.add('hidden');$('appView').classList.remove('hidden')}
function loadingCard(message){return`<div class="card loading">${esc(message)}</div>`}
function pageError(message,retryPage){return`<div class="card error-state"><div><b>Bu bölüm yüklenemedi.</b><span>${esc(message)}</span></div><button class="btn small" data-retry-page="${esc(retryPage)}">Tekrar dene</button></div>`}
function bindRetry(){document.querySelectorAll('[data-retry-page]').forEach(b=>b.onclick=()=>loadPage(b.dataset.retryPage))}

function navCount(id){
  const s=state.dashboard?.summary||{};
  if(id==='revenue')return Number(s.pending_payments||0);
  if(id==='feedback')return Number(s.open_feedback||0);
  if(id==='security')return Number(s.critical_security_events_24h||0);
  if(id==='arvis')return Number(state.dashboard?.arvis?.processing_failed_recent||0);
  return 0;
}
function renderNavigation(){
  const desktop=NAV.map(n=>`<button class="nav-item ${state.active===n.id?'active':''}" data-nav="${n.id}" type="button"><span class="nav-icon">${n.icon}</span><span>${n.label}</span><span class="nav-badge ${navCount(n.id)?'':'hidden'}">${num(navCount(n.id))}</span></button>`).join('');
  const mobile=NAV.map(n=>`<button class="${state.active===n.id?'active':''}" data-nav="${n.id}" type="button"><span>${n.icon}</span>${n.label}</button>`).join('');
  $('desktopNav').innerHTML=desktop;$('mobileNav').innerHTML=mobile;
  document.querySelectorAll('[data-nav]').forEach(button=>button.onclick=()=>showPage(button.dataset.nav));
}
async function showPage(id){
  if(!NAV.some(n=>n.id===id))return;
  state.active=id;const item=NAV.find(n=>n.id===id);
  $('pageTitle').textContent=item.title;$('pageEyebrow').textContent=item.eyebrow;
  document.querySelectorAll('.page').forEach(p=>p.classList.toggle('active',p.id===`page-${id}`));
  renderNavigation();await loadPage(id);
}

function kpi(label,value,foot,toneClass='',icon=''){return`<article class="card kpi"><div class="kpi-top"><div><div class="kpi-label">${esc(label)}</div><div class="kpi-value ${toneClass}">${esc(value)}</div></div>${icon?`<span class="kpi-icon">${icon}</span>`:''}</div><div class="kpi-foot">${esc(foot)}</div></article>`}
function summaryRow(label,value,status='ok'){return`<div class="summary-row"><span>${esc(label)}</span><b>${esc(value)}</b>${badge(status)}</div>`}
function chart(items=[],key='count',currency=false){if(!items.length)return'<div class="empty compact">Trend verisi yok.</div>';const max=Math.max(1,...items.map(x=>Number(x[key]||0)));return`<div class="chart slim">${items.map(x=>{const value=Number(x[key]||0),pct=Math.max(4,Math.round(value/max*100));return`<div class="bar-col" title="${currency?moneyCents(value):num(value)}"><div class="bar" style="--value:${pct}"></div><small>${esc(String(x.date||'').slice(5))}</small></div>`}).join('')}</div>`}
function compactServiceGrid(services={}){
  const labels={database:'Veritabanı',neon_auth:'Neon Auth',solana_rpc:'Solana RPC',shopier:'Shopier',security_radar:'ARVIS Radar'};
  const order=['database','neon_auth','solana_rpc','security_radar','shopier'];
  return`<div class="service-strip">${order.filter(k=>services[k]!==undefined).map(key=>{const s=statusValue(services[key]);const target=key==='security_radar'?'arvis':key==='shopier'?'revenue':'system';return`<button class="service-mini" data-nav="${target}" type="button"><b>${esc(labels[key]||key)}</b>${badge(s)}<small>${serviceNote(key,s)}</small></button>`}).join('')}</div>`;
}
function serviceNote(key,status){if(key==='shopier')return tone(status)==='ok'?'Webhook doğrulaması hazır':'Manuel owner onayı';if(key==='security_radar')return'Pipeline ve worker';if(key==='database')return tone(status)==='ok'?'Neon bağlantısı aktif':'Bağlantı kontrol edilmeli';return tone(status)==='ok'?'Operasyonel':'Kontrol gerekli'}

function renderCommand(){
  const d=state.dashboard||{},s=d.summary||{},t=d.trends||{},a=d.arvis||{},services=d.services||{};
  $('commandContent').innerHTML=`
  <div class="owner-home">
    <section class="hero-card card">
      <div><span class="eyebrow">Gerçek üretim verisi</span><h2>Bugünün kritik durumu</h2><p class="muted">Müşteri, gelir, radar, geri bildirim ve güvenlik akışları tek yerde. Detay için ilgili karta geç.</p></div>
      <div class="hero-status">${badge(a.pipeline_status||'unknown')}<b>${esc(statusLabel(a.pipeline_status||'unknown'))}</b><span>ARVIS pipeline</span></div>
    </section>
    <section class="grid compact-grid">
      ${kpi('Toplam kullanıcı',num(s.total_users),`Son 7 gün +${num(s.new_users_7d)}`,'tone-cyan','◎')}
      ${kpi('Aktif paket',num(s.active_entitlements),`${num(s.expiring_entitlements_7d)} paket 7 günde bitiyor`,'tone-green','◈')}
      ${kpi('ARVIS 24 saat',num(s.radar_findings_24h),`${num(s.risk_cards_24h)} risk · ${num(s.monitor_cards_24h)} monitor`,'tone-cyan','◉')}
      ${kpi('30 günlük gelir',moneyCents(s.revenue_try_cents_30d),`${num(s.paid_orders_30d)} onaylı ödeme`,'tone-green','₺')}
      ${kpi('Bekleyen ödeme',num(s.pending_payments),'Shopier owner onayı',s.pending_payments?'tone-amber':'tone-green','⌛')}
      ${kpi('Açık geri bildirim',num(s.open_feedback),`${num(s.security_feedback)} güvenlik bildirimi`,s.security_feedback?'tone-red':s.open_feedback?'tone-amber':'tone-green','✦')}
      ${kpi('Güvenlik 24 saat',num(s.security_events_24h),`${num(s.critical_security_events_24h)} kritik`,s.critical_security_events_24h?'tone-red':'tone-green','◇')}
      ${kpi('Başarısız işler',num(s.failed_jobs_24h),'Son 24 saat',s.failed_jobs_24h?'tone-red':'tone-green','!')}
    </section>
    <section class="grid compact-grid">
      <article class="card span-8"><div class="card-head"><div><span class="eyebrow">Canlı servisler</span><h2>Operasyon durumu</h2></div></div>${compactServiceGrid(services)}</article>
      <article class="card span-4"><div class="card-head"><div><span class="eyebrow">Owner aksiyonu</span><h2>Bugün ne var?</h2></div></div><div class="clean-list">${(d.action_queue||[]).slice(0,6).map(item=>`<button class="clean-action ${esc(item.priority)}" data-action-tab="${esc(item.target_tab)}" type="button"><span><b>${esc(item.title)}</b><small>${esc(item.detail)}</small></span><i>${num(item.count)}</i></button>`).join('')||'<div class="success-box">Kritik owner aksiyonu yok.</div>'}</div></article>
      <article class="card span-7"><div class="card-head"><div><span class="eyebrow">ARVIS kanıtı</span><h2>Radar üretim özeti</h2></div><button class="btn small" data-nav="arvis" type="button">Radarı aç</button></div><div class="proof-grid">${summaryRow('Pipeline',statusLabel(a.pipeline_status||'unknown'),a.pipeline_status)}${summaryRow('Tamamlanan',num(a.processing_completed),'completed')}${summaryRow('Yakın hata',num(a.processing_failed_recent),a.processing_failed_recent?'bad':'ok')}${summaryRow('Çalışan kanıt kolu',num(a.runtime_engines),'live')}</div></article>
      <article class="card span-5"><div class="card-head"><div><span class="eyebrow">7 günlük trend</span><h2>Kullanıcı ve gelir</h2></div></div><div class="mini-trends"><div><b>${num(s.new_users_7d)}</b><span>Yeni kullanıcı</span>${chart(t.users_7d,'count')}</div><div><b>${moneyCents(sum(t.orders_7d||[],'revenue_try_cents'))}</b><span>Onaylı gelir</span>${chart(t.orders_7d,'revenue_try_cents',true)}</div></div></article>
    </section>
  </div>`;
  document.querySelectorAll('[data-action-tab]').forEach(b=>b.onclick=()=>showPage(b.dataset.actionTab));
  document.querySelectorAll('[data-nav]').forEach(b=>b.onclick=()=>showPage(b.dataset.nav));
}

async function loadArvis(){
  const content=$('arvisContent');content.innerHTML=loadingCard('ARVIS operasyonları yükleniyor…');
  try{const sources=await api('/api/owner/radar/sources');state.radarSources=sources.sources||[];renderArvis()}catch(e){content.innerHTML=pageError(e.message,'arvis');bindRetry()}
}
function renderArvis(){
  const a=state.dashboard?.arvis||{},sources=a.sources||{},fail=a.failures||{};
  const sourceBlock=(name,data={})=>`<div class="source-card"><div class="card-head"><h3>${esc(name)}</h3>${badge(Number(data.recent||0)>0?'live':Number(data.events||0)>0?'stale':'waiting')}</div><div class="source-stats"><div><label>Toplam</label><b>${num(data.events)}</b></div><div><label>15 dakika</label><b>${num(data.recent)}</b></div><div><label>Mint</label><b>${num(data.enriched)}</b></div></div><div class="small muted">Son olay: ${dt(data.last_event_at)}</div></div>`;
  $('arvisContent').innerHTML=`<div class="grid compact-grid">
    ${kpi('Pipeline',statusLabel(a.pipeline_status||'unknown'),'Canlı veri ve evidence worker',tone(a.pipeline_status)==='bad'?'tone-red':tone(a.pipeline_status)==='ok'?'tone-green':'tone-amber','◉')}
    ${kpi('Tamamlanan',num(a.processing_completed),`${num(a.processing_active)} aktif iş`,'tone-green','✓')}
    ${kpi('Çalışan kollar',num(a.runtime_engines),'Son 15 dakikada kanıt üreten kollar','tone-cyan','◇')}
    ${kpi('Başarısız',num(a.processing_failed),`${num(a.processing_failed_recent)} yakın zamanlı`,a.processing_failed_recent?'tone-red':'tone-green','!')}
    <article class="card span-8"><div class="card-head"><div><span class="eyebrow">Canlı kaynaklar</span><h2>Program gözlemcileri</h2></div></div><div class="source-cards">${sourceBlock('Pump.fun / PumpSwap',sources.pump)}${sourceBlock('Raydium',sources.raydium)}</div></article>
    <article class="card span-4"><div class="card-head"><div><span class="eyebrow">Kuyruk sağlığı</span><h2>İşlem kuyruğu</h2></div>${badge(a.pipeline_status)}</div><div class="proof-grid vertical">${summaryRow('Yeniden denenebilir',num(fail.retryable),fail.retryable?'warn':'ok')}${summaryRow('Tükenen',num(fail.exhausted),fail.exhausted?'bad':'ok')}${summaryRow('Son 15 dakika hata',num(fail.recent_15_minutes),fail.recent_15_minutes?'warn':'ok')}${summaryRow('Son hata',fail.latest_error_code||'Yok',fail.latest_error_code?'warn':'ok')}</div></article>
    <article class="card span-12"><details class="owner-details" open><summary><span><b>Radar kaynak kayıt defteri</b><small>Owner doğrulamalı kaynaklar</small></span><span>⌄</span></summary><form id="sourceForm" class="filters clean-form"><select class="select" id="sourceModule"><option value="pump_sybil_radar">Pump.fun Sybil Radar</option><option value="raydium_pool_guardian">Raydium Pool Guardian</option><option value="walletless_claim_shield">Walletless Claim Shield</option></select><input class="input" id="sourceLabel" placeholder="Kaynak etiketi"><input class="input mono" id="sourceAddress" placeholder="Solana program / source adresi"><input class="input" id="sourceNetwork" value="solana-mainnet"><button class="btn primary" type="submit">Kaydet</button></form><div id="sourceRegistry" class="section-gap">${renderSourceRegistry()}</div></details></article>
  </div>`;
  $('sourceForm').onsubmit=createRadarSource;
  document.querySelectorAll('[data-source-toggle]').forEach(b=>b.onclick=()=>toggleRadarSource(b.dataset.sourceToggle,b.dataset.enabled!=='true'));
  document.querySelectorAll('[data-source-delete]').forEach(b=>b.onclick=()=>deleteRadarSource(b.dataset.sourceDelete));
}
function renderSourceRegistry(){if(!state.radarSources.length)return'<div class="empty compact">Owner kayıtlı ek kaynak yok.</div>';return`<div class="table-wrap"><table class="table compact-table"><thead><tr><th>Modül</th><th>Etiket</th><th>Adres</th><th>Ağ</th><th>Durum</th><th>İşlem</th></tr></thead><tbody>${state.radarSources.map(s=>`<tr><td>${esc(s.module_id)}</td><td>${esc(s.label||s.name||'—')}</td><td class="mono">${esc(short(s.address||s.target,48))}</td><td>${esc(s.network||'solana-mainnet')}</td><td>${badge(s.enabled?'enabled':'disabled')}</td><td><div class="row-actions"><button class="btn small" data-source-toggle="${esc(s.id)}" data-enabled="${s.enabled?'true':'false'}" type="button">${s.enabled?'Durdur':'Aç'}</button><button class="btn small danger" data-source-delete="${esc(s.id)}" type="button">Sil</button></div></td></tr>`).join('')}</tbody></table></div>`}
async function createRadarSource(e){e.preventDefault();const body={module_id:$('sourceModule').value,label:$('sourceLabel').value.trim(),address:$('sourceAddress').value.trim(),target:$('sourceAddress').value.trim(),network:$('sourceNetwork').value.trim()||'solana-mainnet',enabled:true};if(!body.address)return toast('Kaynak adresi gerekli.',true);try{await api('/api/owner/radar/sources',{method:'POST',body:JSON.stringify(body)});toast('Kaynak kaydedildi.');await loadArvis()}catch(err){toast(err.message,true)}}
async function toggleRadarSource(id,enabled){try{const source=state.radarSources.find(x=>String(x.id)===String(id));if(!source)return;await api('/api/owner/radar/sources',{method:'PATCH',body:JSON.stringify({...source,id,enabled})});toast(enabled?'Kaynak açıldı.':'Kaynak durduruldu.');await loadArvis()}catch(e){toast(e.message,true)}}
async function deleteRadarSource(id){if(!confirm('Bu doğrulanmış kaynağı silmek istediğine emin misin?'))return;try{await api('/api/owner/radar/sources',{method:'DELETE',body:JSON.stringify({id})});toast('Kaynak silindi.');await loadArvis()}catch(e){toast(e.message,true)}}

async function loadCustomers(query=''){
  const content=$('customersContent');content.innerHTML=loadingCard('Müşteriler yükleniyor…');
  try{const d=await api('/api/owner/users'+(query?`?q=${encodeURIComponent(query)}`:''));state.users=d.users||[];renderCustomers()}catch(e){content.innerHTML=pageError(e.message,'customers');bindRetry()}
}
function renderCustomers(){
  const plans=[...new Set(state.users.map(u=>u.plan_id).filter(Boolean))];
  $('customersContent').innerHTML=`<article class="card"><div class="toolbar"><div><span class="eyebrow">Müşteri operasyonları</span><h2>Erişim ve paket yönetimi</h2></div><div class="filters"><input class="input" id="customerSearch" placeholder="E-posta, auth ID veya wallet"><select class="select" id="customerStatus"><option value="">Tüm durumlar</option><option value="active">Aktif</option><option value="banned">Yasaklı</option><option value="removed">Kaldırılmış</option></select><select class="select" id="customerPlan"><option value="">Tüm paketler</option>${plans.map(p=>`<option value="${esc(p)}">${esc(p)}</option>`).join('')}</select><button class="btn" id="customerSearchButton" type="button">Ara</button></div></div><div id="customerTable">${customerTable(state.users)}</div></article>`;
  $('customerSearchButton').onclick=()=>loadCustomers($('customerSearch').value.trim());$('customerSearch').onkeydown=e=>{if(e.key==='Enter')loadCustomers(e.target.value.trim())};$('customerStatus').onchange=filterCustomers;$('customerPlan').onchange=filterCustomers;bindUserButtons();
}
function filterCustomers(){const status=$('customerStatus').value,plan=$('customerPlan').value;customerTableReplace(state.users.filter(u=>(!status||(u.status||'active')===status)&&(!plan||u.plan_id===plan)))}
function customerTableReplace(users){$('customerTable').innerHTML=customerTable(users);bindUserButtons()}
function bindUserButtons(){document.querySelectorAll('[data-user-manage]').forEach(b=>b.onclick=()=>openUserModal(state.users.find(u=>String(u.id)===String(b.dataset.userManage))))}
function customerTable(users){if(!users.length)return'<div class="empty">Müşteri bulunamadı.</div>';return`<div class="table-wrap"><table class="table"><thead><tr><th>Müşteri</th><th>Paket</th><th>Kredi</th><th>Durum</th><th>Kayıt</th><th>İşlem</th></tr></thead><tbody>${users.map(u=>`<tr><td><b>${esc(u.email)}</b><div class="mono">${esc(short(u.auth_subject,32))}</div>${u.wallet_address?`<div class="mono">${esc(short(u.wallet_address,24))}</div>`:''}</td><td>${esc(u.plan_id||'Aktif paket yok')}<div class="small muted">${esc(statusLabel(u.active_entitlement_status||'none'))}</div></td><td>${num(u.credits)}</td><td>${badge(u.status||'active')}</td><td>${dt(u.created_at)}</td><td><button class="btn small" data-user-manage="${esc(u.id)}" type="button">Yönet</button></td></tr>`).join('')}</tbody></table></div>`}
function openUserModal(user){
  if(!user)return;
  openModal(`<div class="modal-head"><div><span class="eyebrow">Müşteri yönetimi</span><h2>${esc(user.email)}</h2></div><button class="modal-close" data-close type="button">×</button></div><div class="metadata"><div><label>Durum</label><b>${esc(statusLabel(user.status||'active'))}</b></div><div><label>Paket</label><b>${esc(user.plan_id||'Yok')}</b></div><div><label>Kredi</label><b>${num(user.credits)}</b></div><div><label>Paket bitiş</label><b>${dt(user.entitlement_expires_at)}</b></div></div><div class="field"><label for="creditAmount">Kredi ekle</label><input class="input" id="creditAmount" type="number" min="1" value="1"><input class="input" id="creditReason" placeholder="İşlem nedeni"></div><div class="modal-actions"><button class="btn" id="addCredit" type="button">Kredi ekle</button><button class="btn ${user.status==='banned'?'':'warn'}" id="toggleBan" type="button">${user.status==='banned'?'Yasağı kaldır':'Kullanıcıyı yasakla'}</button><button class="btn danger" id="removeUser" type="button">Erişimi kaldır</button></div>`);
  $('addCredit').onclick=async()=>{const credits=Number($('creditAmount').value);if(!Number.isFinite(credits)||credits<1)return toast('Geçerli kredi miktarı gir.',true);try{await api('/api/owner/credits/add',{method:'POST',body:JSON.stringify({email:user.email,credits,reason:$('creditReason').value.trim()||'owner_panel'})});closeModal();toast('Kredi eklendi.');loadCustomers()}catch(e){toast(e.message,true)}};
  $('toggleBan').onclick=async()=>{const ban=user.status!=='banned',reason=prompt(ban?'Yasaklama nedeni:':'Yasağı kaldırma nedeni:','owner_panel');if(reason===null)return;try{await api('/api/owner/users/ban',{method:'POST',body:JSON.stringify({email:user.email,ban,reason})});closeModal();toast(ban?'Kullanıcı yasaklandı.':'Yasak kaldırıldı.');loadCustomers()}catch(e){toast(e.message,true)}};
  $('removeUser').onclick=async()=>{if(!confirm(`${user.email} erişimini kaldırmak istediğine emin misin?`))return;try{await api('/api/owner/users/remove',{method:'POST',body:JSON.stringify({email:user.email,reason:'owner_panel_removed'})});closeModal();toast('Kullanıcı erişimi kaldırıldı.');loadCustomers()}catch(e){toast(e.message,true)}};
}

async function loadRevenue(){
  const content=$('revenueContent');content.innerHTML=loadingCard('Ödeme operasyonları yükleniyor…');
  try{const [health,requests]=await Promise.all([api('/api/owner/payment-health'),api('/api/owner/payment-requests')]);state.paymentHealth=health;state.payments=requests.payment_requests||[];renderRevenue()}catch(e){content.innerHTML=pageError(e.message,'revenue');bindRetry()}
}
function renderRevenue(){
  const h=state.paymentHealth||{},s=h.summary||{},pending=state.payments.filter(x=>x.status==='pending'),events=s.latest_payment_events||[];
  $('revenueContent').innerHTML=`<div class="grid compact-grid">
    ${kpi('Aktif paket',num(s.active_entitlements),'Entitlement tabanlı erişim','tone-green','◈')}
    ${kpi('Bekleyen ödeme',num(s.pending_payment_requests??pending.length),'Shopier owner onayı',pending.length?'tone-amber':'tone-green','⌛')}
    ${kpi('30 günlük gelir',moneyTRY(s.revenue_try_30d),`${num(s.approved_payments_30d)} onaylı ödeme`,'tone-green','₺')}
    ${kpi('Webhook hatası 24 saat',num(s.failed_webhook_events_24h),'Shopier doğrulama olayları',s.failed_webhook_events_24h?'tone-red':'tone-green','!')}
    <article class="card span-7"><div class="card-head"><div><span class="eyebrow">Owner onayı</span><h2>Shopier ödeme talepleri</h2></div>${badge(h.mode||'manual')}</div>${paymentTable(state.payments)}</article>
    <article class="card span-5"><div class="card-head"><div><span class="eyebrow">Son hareketler</span><h2>Ödeme kayıtları</h2></div>${badge(h.provider||'shopier')}</div>${paymentEventTable(events)}</article>
  </div>`;
  document.querySelectorAll('[data-pay-approve]').forEach(b=>b.onclick=()=>reviewPayment(b.dataset.payApprove,true));
  document.querySelectorAll('[data-pay-reject]').forEach(b=>b.onclick=()=>reviewPayment(b.dataset.payReject,false));
}
function paymentTable(items){if(!items.length)return'<div class="empty compact">Ödeme talebi yok.</div>';return`<div class="table-wrap"><table class="table"><thead><tr><th>Müşteri</th><th>Paket</th><th>Tutar</th><th>Durum</th><th>İşlem</th></tr></thead><tbody>${items.map(p=>`<tr><td><b>${esc(p.email)}</b><div class="small muted">${esc(p.full_name||'')}</div></td><td>${esc(p.product_id||p.product_slug||p.plan||'—')}</td><td>${moneyTRY(p.amount_try)} ${esc(p.currency||'TRY')}</td><td>${badge(p.status)}</td><td>${p.status==='pending'?`<div class="row-actions"><button class="btn small primary" data-pay-approve="${esc(p.id)}" type="button">Onayla</button><button class="btn small danger" data-pay-reject="${esc(p.id)}" type="button">Reddet</button></div>`:'—'}</td></tr>`).join('')}</tbody></table></div>`}
function paymentEventTable(items){if(!items.length)return'<div class="empty compact">Ödeme kaydı yok.</div>';return`<div class="table-wrap"><table class="table compact-table"><thead><tr><th>Müşteri</th><th>Paket</th><th>Tutar</th><th>Durum</th><th>Tarih</th></tr></thead><tbody>${items.map(item=>`<tr><td>${esc(item.email||'—')}</td><td>${esc(item.product_id||'—')}</td><td>${moneyTRY(item.amount_try)} ${esc(item.currency||'TRY')}</td><td>${badge(item.status)}</td><td>${dt(item.reviewed_at||item.created_at)}</td></tr>`).join('')}</tbody></table></div>`}
async function reviewPayment(id,approve){const reason=prompt(approve?'Onay notu:':'Red nedeni:',approve?'owner_verified_payment':'payment_not_verified');if(reason===null)return;try{await api(approve?'/api/owner/payments/approve':'/api/owner/payments/reject',{method:'POST',body:JSON.stringify({payment_request_id:id,reason})});toast(approve?'Ödeme onaylandı ve paket aktive edildi.':'Ödeme reddedildi.');await refreshDashboard();await loadRevenue()}catch(e){toast(e.message,true)}}

async function loadFeedback(status=''){
  const content=$('feedbackContent');content.innerHTML=loadingCard('Geri bildirimler yükleniyor…');
  try{const d=await api('/api/owner/feedback?limit=200'+(status?`&status=${encodeURIComponent(status)}`:''));state.feedback=d.items||[];renderFeedback()}catch(e){content.innerHTML=pageError(e.message,'feedback');bindRetry()}
}
function renderFeedback(){
  const counts=state.feedback.reduce((acc,item)=>{acc[item.status]=(acc[item.status]||0)+1;return acc},{});
  const categories=[...new Set(state.feedback.map(item=>item.category).filter(Boolean))];
  $('feedbackContent').innerHTML=`<div class="grid compact-grid">
    ${kpi('Yeni',num(counts.new),'Henüz incelenmedi',counts.new?'tone-amber':'tone-green','✦')}
    ${kpi('İnceleniyor',num(counts.reviewing),'Owner değerlendirmesi','tone-cyan','◎')}
    ${kpi('Planlandı',num(counts.planned),'Ürün planına alındı','tone-cyan','◇')}
    ${kpi('Çözüldü',num((counts.resolved||0)+(counts.closed||0)),'Tamamlanan kayıtlar','tone-green','✓')}
    <article class="card span-12"><div class="toolbar"><div><span class="eyebrow">Müşteri sinyalleri</span><h2>Geri bildirim kuyruğu</h2></div><div class="filters"><input class="input" id="feedbackSearch" placeholder="Başlık, mesaj veya e-posta"><select class="select" id="feedbackStatus"><option value="">Tüm durumlar</option><option value="new">Yeni</option><option value="reviewing">İnceleniyor</option><option value="planned">Planlandı</option><option value="resolved">Çözüldü</option><option value="closed">Kapalı</option></select><select class="select" id="feedbackCategory"><option value="">Tüm kategoriler</option>${categories.map(c=>`<option value="${esc(c)}">${esc(feedbackCategoryLabel(c))}</option>`).join('')}</select></div></div><div id="feedbackTable">${feedbackTable(state.feedback)}</div></article>
  </div>`;
  $('feedbackSearch').oninput=filterFeedback;$('feedbackStatus').onchange=filterFeedback;$('feedbackCategory').onchange=filterFeedback;bindFeedbackButtons();
}
function feedbackCategoryLabel(value){return({system_gap:'Sistem açığı',bug:'Hata',suggestion:'Öneri',usability:'Kullanılabilirlik',billing:'Ödeme',security:'Güvenlik',other:'Diğer'})[value]||value||'Diğer'}
function filterFeedback(){const q=$('feedbackSearch').value.toLowerCase(),status=$('feedbackStatus').value,category=$('feedbackCategory').value;const items=state.feedback.filter(item=>(!status||item.status===status)&&(!category||item.category===category)&&(!q||JSON.stringify(item).toLowerCase().includes(q)));$('feedbackTable').innerHTML=feedbackTable(items);bindFeedbackButtons()}
function feedbackTable(items){if(!items.length)return'<div class="empty">Geri bildirim bulunamadı.</div>';return`<div class="table-wrap"><table class="table"><thead><tr><th>Tarih</th><th>Kategori</th><th>Başlık ve mesaj</th><th>İletişim</th><th>Durum</th><th>İşlem</th></tr></thead><tbody>${items.map(item=>`<tr><td>${dt(item.created_at)}</td><td>${badge(item.category==='security'?'critical':item.category)}</td><td><b>${esc(item.subject)}</b><div class="small muted">${esc(short(item.message,110))}</div></td><td>${esc(item.contact_email||'—')}<div class="small muted">${esc(short(item.page_url,48))}</div></td><td>${badge(item.status)}</td><td><button class="btn small" data-feedback-manage="${esc(item.id)}" type="button">Yönet</button></td></tr>`).join('')}</tbody></table></div>`}
function bindFeedbackButtons(){document.querySelectorAll('[data-feedback-manage]').forEach(b=>b.onclick=()=>openFeedbackModal(state.feedback.find(item=>String(item.id)===String(b.dataset.feedbackManage))))}
function openFeedbackModal(item){
  if(!item)return;
  openModal(`<div class="modal-head"><div><span class="eyebrow">${esc(feedbackCategoryLabel(item.category))}</span><h2>${esc(item.subject)}</h2></div><button class="modal-close" data-close type="button">×</button></div><div class="feedback-message">${esc(item.message).replace(/\n/g,'<br>')}</div><div class="metadata"><div><label>İletişim</label><b>${esc(item.contact_email||'Yok')}</b></div><div><label>Kaynak sayfa</label><b>${esc(item.page_url||'Yok')}</b></div><div><label>Oluşturuldu</label><b>${dt(item.created_at)}</b></div><div><label>Güncellendi</label><b>${dt(item.updated_at)}</b></div></div><div class="field"><label for="feedbackModalStatus">Durum</label><select class="select" id="feedbackModalStatus"><option value="new">Yeni</option><option value="reviewing">İnceleniyor</option><option value="planned">Planlandı</option><option value="resolved">Çözüldü</option><option value="closed">Kapalı</option></select></div><div class="field"><label for="feedbackOwnerNote">Owner notu</label><textarea class="textarea" id="feedbackOwnerNote" maxlength="2000" placeholder="Karar, takip veya çözüm notu">${esc(item.owner_note||'')}</textarea></div><div class="modal-actions"><button class="btn primary" id="saveFeedback" type="button">Kaydet</button></div>`);
  $('feedbackModalStatus').value=item.status||'new';
  $('saveFeedback').onclick=async()=>{try{await api('/api/owner/feedback',{method:'POST',body:JSON.stringify({id:item.id,status:$('feedbackModalStatus').value,owner_note:$('feedbackOwnerNote').value.trim()})});closeModal();toast('Geri bildirim güncellendi.');await refreshDashboard();await loadFeedback()}catch(e){toast(e.message,true)}};
}

async function loadSecurity(){
  const content=$('securityContent');content.innerHTML=loadingCard('Güvenlik olayları yükleniyor…');
  try{const d=await api('/api/owner/security-events?limit=200');state.security=d.events||[];renderSecurity()}catch(e){content.innerHTML=pageError(e.message,'security');bindRetry()}
}
function renderSecurity(){
  const severities=[...new Set(state.security.map(e=>e.severity).filter(Boolean))];
  const successfulLogins=state.security.filter(e=>String(e.event_type||'').includes('login_success')).length;
  const critical=state.security.filter(e=>tone(e.severity)==='bad').length;
  const authFailures=state.security.filter(e=>/login|auth|token/i.test(String(e.event_type||''))&&tone(e.severity)==='bad').length;
  $('securityContent').innerHTML=`<div class="grid compact-grid">
    ${kpi('Toplam olay',num(state.security.length),'Son 200 kayıt','tone-cyan','◇')}
    ${kpi('Kritik / hata',num(critical),'Owner incelemesi',critical?'tone-red':'tone-green','!')}
    ${kpi('Başarılı giriş',num(successfulLogins),'Owner ve müşteri denetimi','tone-green','✓')}
    ${kpi('Auth hatası',num(authFailures),'Giriş ve token olayları',authFailures?'tone-amber':'tone-green','◎')}
    <article class="card span-12"><div class="toolbar"><div><span class="eyebrow">Güvenlik denetim akışı</span><h2>Kim, ne zaman, ne yaptı?</h2></div><div class="filters"><input class="input" id="securitySearch" placeholder="Olay, aktör, yol veya IP"><select class="select" id="securitySeverity"><option value="">Tüm önem seviyeleri</option>${severities.map(s=>`<option value="${esc(s)}">${esc(statusLabel(s))}</option>`).join('')}</select></div></div><div id="securityTable">${securityTable(state.security)}</div></article>
  </div>`;
  $('securitySearch').oninput=filterSecurity;$('securitySeverity').onchange=filterSecurity;
}
function filterSecurity(){const q=$('securitySearch').value.toLowerCase(),severity=$('securitySeverity').value;const items=state.security.filter(e=>(!severity||e.severity===severity)&&(!q||JSON.stringify(e).toLowerCase().includes(q)));$('securityTable').innerHTML=securityTable(items)}
function securityTable(items){if(!items.length)return'<div class="empty">Güvenlik olayı yok.</div>';return`<div class="table-wrap"><table class="table"><thead><tr><th>Zaman</th><th>Olay</th><th>Aktör</th><th>Yol / IP</th><th>Önem</th><th>Metadata</th></tr></thead><tbody>${items.map(e=>`<tr><td>${dt(e.created_at)}</td><td><b>${esc(e.event_type)}</b></td><td>${esc(e.actor_type||'—')}<div class="mono">${esc(short(e.actor_id,28))}</div></td><td class="mono">${esc(e.path||'—')}<br>${esc(e.ip||'')}</td><td>${badge(e.severity)}</td><td class="mono">${esc(short(JSON.stringify(e.metadata||{}),90))}</td></tr>`).join('')}</tbody></table></div>`}

async function loadSystem(){
  const content=$('systemContent');content.innerHTML=loadingCard('Sistem sağlığı yükleniyor…');
  try{const health=await api('/api/owner/health');renderSystem(health)}catch(e){renderSystem(null,e)}
}
function renderSystem(health,error){
  const services=state.dashboard?.services||{};
  const visible=Object.entries(health?.services||{}).filter(([key])=>!['github','openai'].includes(String(key).toLowerCase()));
  $('systemContent').innerHTML=`<div class="grid compact-grid">
    <article class="card span-8"><div class="card-head"><div><span class="eyebrow">Servis matrisi</span><h2>Bağımlılıkların gerçek durumu</h2></div></div>${compactServiceGrid(services)}${error?`<div class="error-box section-gap">${esc(error.message)}</div>`:''}</article>
    <article class="card span-4"><div class="card-head"><div><span class="eyebrow">İç servis kontrolü</span><h2>Owner health</h2></div></div><div class="proof-grid vertical">${visible.map(([k,v])=>summaryRow(k,statusLabel(statusValue(v)),statusValue(v))).join('')||summaryRow('Durum','Veri yok','warn')}</div></article>
    <article class="card span-12"><div class="card-head"><div><span class="eyebrow">Operasyon ilkeleri</span><h2>Owner güvenlik sınırları</h2></div></div><div class="policy-grid"><div class="success-box">Kanıt yoksa müşteri kartı ve temiz skor yok.</div><div class="success-box">Premium erişim yalnız aktif entitlement ile açılır.</div><div class="success-box">Secret, token ve hassas değerler panelde gösterilmez.</div></div></article>
  </div>`;
  document.querySelectorAll('[data-nav]').forEach(b=>b.onclick=()=>showPage(b.dataset.nav));
}

function renderBrain(){const root=$('brainContent');root.innerHTML=loadingCard('Owner AI Chat hazırlanıyor…');setTimeout(()=>{if(window.OwnerAIChat?.mount)window.OwnerAIChat.mount(root)},50)}
function openModal(html){$('modalRoot').innerHTML=`<div class="modal-backdrop"><section class="modal" role="dialog" aria-modal="true">${html}</section></div>`;document.querySelectorAll('[data-close]').forEach(b=>b.onclick=closeModal);document.querySelector('.modal-backdrop').onclick=e=>{if(e.target.classList.contains('modal-backdrop'))closeModal()}}
function closeModal(){$('modalRoot').innerHTML=''}
document.addEventListener('keydown',event=>{if(event.key==='Escape')closeModal()});

async function refreshDashboard(){
  if(state.loading)return;state.loading=true;setSync('Güncelleniyor','warn');
  try{
    state.dashboard=await api('/api/owner/command-center');showApp();setSync(`Canlı · ${dt(state.dashboard.generated_at)}`,'ok');renderNavigation();
    if(state.active==='command')renderCommand();
    if(state.active==='arvis'&&$('arvisContent').children.length)renderArvis();
    if(state.active==='system'&&$('systemContent').children.length)renderSystem(null);
  }catch(e){
    if(e.status===404||e.status===401||e.status===403)showLogin('Owner oturumu gerekli.');
    else{setSync('Bağlantı hatası','bad');toast(e.message,true)}
  }finally{state.loading=false}
}
async function loadPage(id){
  if(id==='command'){renderCommand();return}
  if(id==='arvis'){await loadArvis();return}
  if(id==='customers'){await loadCustomers();return}
  if(id==='revenue'){await loadRevenue();return}
  if(id==='feedback'){await loadFeedback();return}
  if(id==='security'){await loadSecurity();return}
  if(id==='system'){await loadSystem();return}
  if(id==='brain'){renderBrain()}
}

$('loginForm').onsubmit=async e=>{
  e.preventDefault();$('loginError').classList.add('hidden');$('loginButton').disabled=true;$('loginButton').textContent='Doğrulanıyor…';
  try{await api('/api/owner/login',{method:'POST',body:JSON.stringify({wallet:$('loginWallet').value.trim(),secret:$('loginSecret').value})});$('loginSecret').value='';await refreshDashboard();await showPage('command')}
  catch(err){showLogin(err.status===404?'Owner bilgileri doğrulanamadı.':err.message)}
  finally{$('loginButton').disabled=false;$('loginButton').textContent='Kontrol merkezine gir'}
};
async function logout(){try{await api('/api/owner/logout',{method:'POST'})}catch{}state.dashboard=null;showLogin('Oturum kapatıldı.')}
$('logoutButton').onclick=logout;$('mobileLogoutButton').onclick=logout;
$('refreshButton').onclick=async()=>{await refreshDashboard();await loadPage(state.active)};

renderNavigation();refreshDashboard();setInterval(()=>{if(!$('appView').classList.contains('hidden'))refreshDashboard()},30000);
})();
