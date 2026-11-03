(()=>{
'use strict';
if(window.__ownerTronFlowRadarV1)return;
window.__ownerTronFlowRadarV1=true;
const REFRESH_MS=15000;
const esc=value=>String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
const arr=value=>Array.isArray(value)?value:[];
const num=value=>{const n=Number(value);return Number.isFinite(n)?n.toLocaleString('en-US'):'—'};
const short=(value,length=22)=>{const text=String(value||'');return text.length>length?`${text.slice(0,8)}…${text.slice(-6)}`:text||'—'};
const dt=value=>{if(!value)return'—';const date=new Date(value);return Number.isNaN(date.getTime())?'—':new Intl.DateTimeFormat('en-GB',{dateStyle:'short',timeStyle:'medium'}).format(date)};
const ms=value=>value?dt(Number(value)):'—';
const tronAddress=/^T[1-9A-HJ-NP-Za-km-z]{33}$/;
const tronTx=/^[0-9a-f]{64}$/;
const addressLink=value=>tronAddress.test(String(value||''))?`<a class="mono" href="https://tronscan.org/#/address/${esc(value)}" target="_blank" rel="noopener noreferrer" title="${esc(value)}">${esc(short(value))}</a>`:`<span class="mono">${esc(short(value))}</span>`;
const txLink=value=>tronTx.test(String(value||''))?`<a class="mono" href="https://tronscan.org/#/transaction/${esc(value)}" target="_blank" rel="noopener noreferrer" title="${esc(value)}">${esc(short(value,16))}</a>`:'—';
const severityTone=value=>({critical:'bad',high:'bad',medium:'warn',low:'warn',info:'ok'}[String(value||'').toLowerCase()]||'warn');
const badge=(tone,label)=>`<span class="badge ${tone}">${esc(String(label||'UNKNOWN').toUpperCase())}</span>`;
const freezeBadge=value=>({frozen:badge('ok','Frozen by issuer'),not_frozen:badge('bad','Not frozen'),unknown:badge('warn','Unknown'),not_checked:badge('warn','Not checked')}[String(value||'not_checked')]||badge('warn',value));
const kpi=(label,value,foot,tone='tone-cyan')=>`<article class="card kpi"><div class="kpi-top"><div><div class="kpi-label">${esc(label)}</div><div class="kpi-value ${tone}">${esc(value)}</div></div></div><div class="kpi-foot">${esc(foot)}</div></article>`;
const kindLabel={tron_flow_seed_outflow:'Seed outflow',tron_flow_fan_out:'Scripted fan-out',tron_flow_bridge_exit:'Bridge exit',tron_flow_freeze_candidate:'Unfrozen holder',tron_large_bridge_inflow:'Large bridge deposit'};
const actionLabel=value=>{const text=String(value||'');if(text==='issuer_freeze_request:tether_usdt')return'Ask Tether to freeze';if(text.startsWith('bridge_operator_notice:'))return`Notify ${text.slice(23)}`;if(text==='receiving_party_notice')return'Notify receiving party';return text};

function panelMarkup(){
  return `<article class="card span-12" id="ownerTronFlowRadarPanel"><div class="card-head"><div><span class="eyebrow">TRON FLOW RADAR · IN MEMORY · OBSERVE ONLY</span><h2>Network-wide USDT follow from seed wallets</h2><p class="muted">Reads every solidified Tron block and follows USDT leaving seed wallets hop by hop. Nothing per block, transfer or address is stored; only capped findings become security alerts. Koschei cannot reverse a transfer, so each finding names who can hold the funds.</p></div><span class="badge warn" id="ownerTronFlowRadarState">LOADING</span></div><div id="ownerTronFlowRadarBody" class="section-gap"><div class="card loading">Loading Tron Flow Radar state…</div></div></article>`;
}

async function fetchState(){
  const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),15000);
  try{
    const response=await fetch('/api/owner/radar/tron-flow',{credentials:'same-origin',cache:'no-store',signal:controller.signal});
    const data=await response.json().catch(()=>({}));
    if(!response.ok)throw new Error(data.error||data.message||`HTTP ${response.status}`);
    return data;
  }finally{clearTimeout(timer)}
}

function findingsTable(findings){
  if(!findings.length)return'<div class="empty">No findings yet. Findings appear when a seed wallet moves USDT, a followed wallet fans out, followed funds enter a pinned bridge, or an unfrozen address accumulates followed USDT.</div>';
  const rows=findings.map(item=>{const evidence=item.evidence||{};const actions=arr(evidence.response_options).map(actionLabel).map(esc).join('<br>')||'—';const hop=evidence.hop===undefined?'':` · hop ${esc(evidence.hop)}`;
    return `<tr><td class="small">${esc(dt(item.observed_at))}</td><td>${badge(severityTone(item.severity),item.severity)}</td><td><b>${esc(kindLabel[item.kind]||item.kind)}</b><div class="muted small">${esc(evidence.amount||'')}${hop}</div></td><td>${addressLink(item.address)}</td><td class="small">${esc(item.message)}</td><td class="small">${actions}</td><td>${txLink(evidence.transaction_id)}</td></tr>`;}).join('');
  return `<div class="table-wrap"><table class="table"><thead><tr><th>Observed</th><th>Severity</th><th>Finding</th><th>Address</th><th>What happened</th><th>Who can hold funds</th><th>Transaction</th></tr></thead><tbody>${rows}</tbody></table></div>`;
}

function followedTable(followed){
  if(!followed.length)return'<div class="empty">No seed wallets yet. Add Tron wallets to the watchlist or set KOSCHEI_TRON_FLOW_RADAR_SEEDS.</div>';
  const rows=followed.map(item=>`<tr><td>${addressLink(item.address)} ${item.seed?badge('warn','Seed'):''}</td><td>${esc(item.hop)}</td><td>${item.seed?'—':addressLink(item.origin_seed)}</td><td class="mono">${esc(item.seed?'—':item.followed_received)}</td><td>${item.seed?'—':freezeBadge(item.usdt_freeze_state)}</td><td class="small">${esc(ms(item.last_activity_block_timestamp_ms))}</td></tr>`).join('');
  return `<div class="table-wrap"><table class="table"><thead><tr><th>Address</th><th>Hop</th><th>Origin seed</th><th>Followed USDT received</th><th>USDT issuer freeze</th><th>Last activity</th></tr></thead><tbody>${rows}</tbody></table></div><p class="muted small">"Followed" means the address received USDT descending from a seed within the hop limit. It is not an attribution of control or intent.</p>`;
}

function render(data){
  const body=document.getElementById('ownerTronFlowRadarBody'),state=document.getElementById('ownerTronFlowRadarState');if(!body||!state)return;
  if(data.enabled!==true){state.className='badge warn';state.textContent='OFF';body.innerHTML=`<div class="empty">${esc(data.reason||'Tron Flow Radar is not running in this process.')}</div>`;return;}
  const radar=data.radar||{},settings=radar.settings||{};
  const head=Number(radar.head||0),cursor=Number(radar.cursor||0),pending=head>=cursor?head-cursor:0;
  const healthy=radar.running===true&&!radar.last_error;
  state.className=`badge ${healthy?'ok':'bad'}`;state.textContent=radar.running!==true?'STOPPED':radar.last_error?'DEGRADED':'RUNNING';
  const error=radar.last_error?`<div class="error-box section-gap"><b>Latest cycle failed:</b> ${esc(radar.last_error)}<br><span class="muted">Provider failures are not security results; the cursor resumes from the last processed block.</span></div>`:'';
  body.innerHTML=`<div class="grid compact-grid">${kpi('Solid head',num(head),`Last cycle ${dt(radar.last_cycle_at)}`)}${kpi('Pending blocks',num(pending),`Cursor ${num(cursor)}`,pending>100?'tone-amber':'tone-green')}${kpi('Seed wallets',num(radar.seeds),'Env seeds + active Tron watchlist')}${kpi('Followed addresses',num(radar.followed_addresses),`Max ${num(settings.max_tracked)} in memory`)}${kpi('USDT transfers read',num(radar.usdt_transfers_processed),'Since process start')}${kpi('Findings suppressed',num(radar.findings_suppressed_this_hour),`Cap ${num(settings.max_findings_per_hour)} per hour`,Number(radar.findings_suppressed_this_hour)>0?'tone-amber':'tone-green')}</div>${error}<p class="muted small section-gap">Follow ≥ ${esc(settings.min_follow||'—')} up to ${esc(settings.max_hops??'—')} hops · fan-out ${esc(settings.burst_count??'—')} transfers / ${esc(settings.burst_window_seconds??'—')}s · freeze check ≥ ${esc(settings.freeze_check||'—')} · bridge deposit alert ≥ ${esc(settings.bridge_alert||'—')} · ${esc(settings.pinned_bridges??0)} pinned bridge address(es) · storage ${esc(radar.storage||'in_memory_only')}</p><h3 class="section-gap">Recent findings</h3>${findingsTable(arr(radar.recent_findings))}<h3 class="section-gap">Followed addresses</h3>${followedTable(arr(radar.top_followed))}`;
}

let timer=null;let busy=false;
async function refresh(){
  if(busy||!document.getElementById('ownerTronFlowRadarPanel'))return;busy=true;
  try{render(await fetchState());}
  catch(error){const body=document.getElementById('ownerTronFlowRadarBody'),state=document.getElementById('ownerTronFlowRadarState');if(state){state.className='badge bad';state.textContent='UNAVAILABLE';}if(body)body.innerHTML=`<div class="error-box"><b>Tron Flow Radar state could not be read.</b><br>${esc(error?.message||'unknown error')}</div>`;}
  finally{busy=false;}
}

function schedule(){
  if(timer)clearInterval(timer);
  timer=setInterval(()=>{if(document.visibilityState==='visible')refresh();},REFRESH_MS);
}

function ensurePanel(){
  const root=document.getElementById('securityContent');if(!root||!root.children.length||document.getElementById('ownerTronFlowRadarPanel'))return;
  const grid=root.querySelector(':scope > .grid.compact-grid')||root;grid.insertAdjacentHTML('beforeend',panelMarkup());refresh();schedule();
}

const root=document.getElementById('securityContent');if(root){new MutationObserver(ensurePanel).observe(root,{childList:true});ensurePanel();}
})();
