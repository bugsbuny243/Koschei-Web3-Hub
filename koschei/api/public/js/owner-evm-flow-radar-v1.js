(()=>{
'use strict';
if(window.__ownerEvmFlowRadarV1)return;
window.__ownerEvmFlowRadarV1=true;
const REFRESH_MS=15000;
const esc=value=>String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
const arr=value=>Array.isArray(value)?value:[];
const num=value=>{const n=Number(value);return Number.isFinite(n)?n.toLocaleString('en-US'):'—'};
const short=(value,length=22)=>{const text=String(value||'');return text.length>length?`${text.slice(0,8)}…${text.slice(-6)}`:text||'—'};
const dt=value=>{if(!value)return'—';const date=new Date(value);return Number.isNaN(date.getTime())?'—':new Intl.DateTimeFormat('en-GB',{dateStyle:'short',timeStyle:'medium'}).format(date)};
const evmAddress=/^0x[0-9a-f]{40}$/;
const evmTx=/^0x[0-9a-f]{64}$/;
const explorerOk=/^https:\/\/[a-z0-9.-]+$/;
const addressLink=(explorer,value)=>evmAddress.test(String(value||''))&&explorerOk.test(String(explorer||''))?`<a class="mono" href="${esc(explorer)}/address/${esc(value)}" target="_blank" rel="noopener noreferrer" title="${esc(value)}">${esc(short(value))}</a>`:`<span class="mono">${esc(short(value))}</span>`;
const txLink=(explorer,value)=>evmTx.test(String(value||''))&&explorerOk.test(String(explorer||''))?`<a class="mono" href="${esc(explorer)}/tx/${esc(value)}" target="_blank" rel="noopener noreferrer" title="${esc(value)}">${esc(short(value,16))}</a>`:'—';
const severityTone=value=>({critical:'bad',high:'bad',medium:'warn',low:'warn',info:'ok'}[String(value||'').toLowerCase()]||'warn');
const badge=(tone,label)=>`<span class="badge ${tone}">${esc(String(label||'UNKNOWN').toUpperCase())}</span>`;
const freezeBadge=value=>({frozen:badge('ok','Frozen by issuer'),not_frozen:badge('bad','Not frozen'),unknown:badge('warn','Unknown'),not_checked:badge('warn','Not checked'),issuer_cannot_freeze:badge('warn','No issuer freeze')}[String(value||'not_checked')]||badge('warn',value));
const kpi=(label,value,foot,tone='tone-cyan')=>`<article class="card kpi"><div class="kpi-top"><div><div class="kpi-label">${esc(label)}</div><div class="kpi-value ${tone}">${esc(value)}</div></div></div><div class="kpi-foot">${esc(foot)}</div></article>`;
const kindLabel={evm_flow_seed_outflow:'Seed outflow',evm_flow_fan_out:'Scripted fan-out',evm_flow_bridge_exit:'Bridge exit',evm_flow_freeze_candidate:'Unfrozen holder',evm_large_bridge_inflow:'Large bridge deposit'};
const actionLabel=value=>{const text=String(value||'');if(text==='issuer_freeze_request:tether_usdt')return'Ask Tether to freeze';if(text==='issuer_freeze_request:circle_usdc')return'Ask Circle to freeze';if(text.startsWith('bridge_operator_notice:'))return`Notify ${text.slice(23)}`;if(text==='receiving_party_notice')return'Notify receiving party';return text};

function panelMarkup(){
  return `<article class="card span-12" id="ownerEvmFlowRadarPanel"><div class="card-head"><div><span class="eyebrow">EVM FLOW RADAR · IN MEMORY · OBSERVE ONLY</span><h2>Network-wide USDT/USDC follow from seed wallets</h2><p class="muted">Reads issuer stablecoin Transfer logs for every block on each enabled EVM network, a few confirmations behind the head, and follows funds leaving seed wallets hop by hop. No ClickHouse and nothing per block, transfer or address is stored; only capped findings become security alerts. Koschei cannot reverse a transfer, so each finding names who can hold the funds and can be downloaded as a signed evidence package for them.</p></div><span class="badge warn" id="ownerEvmFlowRadarState">LOADING</span></div><div id="ownerEvmFlowRadarBody" class="section-gap"><div class="card loading">Loading EVM Flow Radar state…</div></div></article>`;
}

async function fetchState(){
  const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),15000);
  try{
    const response=await fetch('/api/owner/radar/evm-flow',{credentials:'same-origin',cache:'no-store',signal:controller.signal});
    const data=await response.json().catch(()=>({}));
    if(!response.ok)throw new Error(data.error||data.message||`HTTP ${response.status}`);
    return data;
  }finally{clearTimeout(timer)}
}

function findingsTable(explorer,findings){
  if(!findings.length)return'<div class="empty">No findings yet on this network. Findings appear when a seed wallet moves USDT/USDC, a followed wallet fans out, followed funds enter a pinned bridge, or an unfrozen address accumulates followed funds.</div>';
  const rows=findings.map(item=>{const evidence=item.evidence||{};const actions=arr(evidence.response_options).map(actionLabel).map(esc).join('<br>')||'—';const hop=evidence.hop===undefined?'':` · hop ${esc(evidence.hop)}`;
    return `<tr><td class="small">${esc(dt(item.observed_at))}</td><td>${badge(severityTone(item.severity),item.severity)}</td><td><b>${esc(kindLabel[item.kind]||item.kind)}</b><div class="muted small">${esc(evidence.amount||'')}${hop}</div></td><td>${addressLink(explorer,item.address)}</td><td class="small">${esc(item.message)}</td><td class="small">${actions}</td><td>${txLink(explorer,evidence.transaction_id)}</td><td><button class="btn" type="button" data-evm-evidence="${esc(item.dedupe_key)}">Download package</button></td></tr>`;}).join('');
  return `<div class="table-wrap"><table class="table"><thead><tr><th>Observed</th><th>Severity</th><th>Finding</th><th>Address</th><th>What happened</th><th>Who can hold funds</th><th>Transaction</th><th>Signed evidence</th></tr></thead><tbody>${rows}</tbody></table></div>`;
}

function holdingsCell(item){
  const holdings=arr(item.holdings);if(item.seed||!holdings.length)return'—';
  return holdings.map(h=>`<div class="small"><b>${esc(h.symbol)}</b> received ${esc(h.followed_received)} · now ${esc(h.current_balance||'not read')} ${freezeBadge(h.issuer_freeze_state)}</div>`).join('');
}

function followedTable(explorer,followed){
  if(!followed.length)return'<div class="empty">No seed wallets yet. Add EVM wallets to the watchlist on this network or set KOSCHEI_EVM_FLOW_RADAR_SEEDS.</div>';
  const rows=followed.map(item=>`<tr><td>${addressLink(explorer,item.address)} ${item.seed?badge('warn','Seed'):''}</td><td>${esc(item.hop)}</td><td>${item.seed?'—':addressLink(explorer,item.origin_seed)}</td><td class="mono">${esc(item.seed?'—':item.followed_received)}</td><td>${holdingsCell(item)}</td><td class="small mono">${esc(item.last_activity_block&&item.last_activity_block!=='0'?num(item.last_activity_block):'—')}</td></tr>`).join('');
  return `<div class="table-wrap"><table class="table"><thead><tr><th>Address</th><th>Hop</th><th>Origin seed</th><th>Followed received</th><th>Per token: received · current balance · issuer freeze</th><th>Last activity block</th></tr></thead><tbody>${rows}</tbody></table></div>`;
}

function networkSection(radar){
  const settings=radar.settings||{};const explorer=radar.explorer||'';
  const head=Number(radar.head||0),cursor=Number(radar.cursor||0),pending=head>=cursor?head-cursor:0;
  const healthy=radar.running===true&&!radar.last_error;
  const status=radar.running!==true?badge('bad','Stopped'):radar.last_error?badge('bad','Degraded'):badge('ok','Running');
  const tokens=arr(radar.tokens).map(t=>`${esc(t.symbol)}${t.issuer?` (${esc(t.issuer)} can freeze)`:' (no issuer freeze)'}`).join(', ');
  const error=radar.last_error?`<div class="error-box section-gap"><b>Latest cycle failed:</b> ${esc(radar.last_error)}<br><span class="muted">Provider failures are not security results; the cursor resumes from the last processed block.</span></div>`:'';
  return {healthy,html:`<section class="section-gap" data-evm-network="${esc(radar.network)}"><h3>${esc(radar.network)} ${status}</h3><p class="muted small">Tokens: ${tokens||'—'} · RPC from ${esc(settings.rpc_variable||'—')}</p><div class="grid compact-grid">${kpi('Confirmed head',num(head),`Latest ${num(radar.latest_block)} · ${esc(settings.confirmations??'—')} confirmations`)}${kpi('Pending blocks',num(pending),`Cursor ${num(cursor)} · range ${num(radar.block_range)}`,pending>200?'tone-amber':'tone-green')}${kpi('Seed wallets',num(radar.seeds),'Env seeds + active watchlist')}${kpi('Followed addresses',num(radar.followed_addresses),`Max ${num(settings.max_tracked)} in memory`)}${kpi('Transfers read',num(radar.transfers_processed),'Since process start')}${kpi('Pending holder checks',num(radar.pending_holder_checks),'Retried with backoff',Number(radar.pending_holder_checks)>0?'tone-amber':'tone-green')}${kpi('Findings suppressed',num(radar.findings_suppressed_this_hour),`Cap ${num(settings.max_findings_per_hour)} per hour`,Number(radar.findings_suppressed_this_hour)>0?'tone-amber':'tone-green')}</div>${error}<p class="muted small section-gap">Follow ≥ ${esc(settings.min_follow||'—')} up to ${esc(settings.max_hops??'—')} hops · fan-out ${esc(settings.burst_count??'—')} transfers / ${esc(settings.burst_window_seconds??'—')}s · freeze check ≥ ${esc(settings.freeze_check||'—')} · bridge deposit alert ≥ ${esc(settings.bridge_alert||'—')} · ${esc(settings.pinned_bridges??0)} pinned bridge address(es) · poll ${esc(settings.poll_seconds??'—')}s · last cycle ${esc(dt(radar.last_cycle_at))} · storage ${esc(radar.storage||'in_memory_only')}</p><h4 class="section-gap">Recent findings</h4>${findingsTable(explorer,arr(radar.recent_findings))}<h4 class="section-gap">Followed addresses</h4>${followedTable(explorer,arr(radar.top_followed))}<p class="muted small">"Followed" means the address received stablecoins descending from a seed within the hop limit. It is not an attribution of control or intent. Received is historical; the current balance and issuer freeze state are read before any holder finding.</p></section>`};
}

function render(data){
  const body=document.getElementById('ownerEvmFlowRadarBody'),state=document.getElementById('ownerEvmFlowRadarState');if(!body||!state)return;
  if(data.enabled!==true){state.className='badge warn';state.textContent='OFF';body.innerHTML=`<div class="empty">${esc(data.reason||'EVM Flow Radar is not running in this process.')}</div>`;return;}
  const sections=arr(data.networks).map(networkSection);
  const allHealthy=sections.length>0&&sections.every(s=>s.healthy);
  state.className=`badge ${allHealthy?'ok':'bad'}`;state.textContent=allHealthy?`RUNNING · ${sections.length} NETWORK${sections.length===1?'':'S'}`:'DEGRADED';
  body.innerHTML=sections.map(s=>s.html).join('')||'<div class="empty">No network is running.</div>';
}

let timer=null;let busy=false;
async function refresh(){
  if(busy||!document.getElementById('ownerEvmFlowRadarPanel'))return;busy=true;
  try{render(await fetchState());}
  catch(error){const body=document.getElementById('ownerEvmFlowRadarBody'),state=document.getElementById('ownerEvmFlowRadarState');if(state){state.className='badge bad';state.textContent='UNAVAILABLE';}if(body)body.innerHTML=`<div class="error-box"><b>EVM Flow Radar state could not be read.</b><br>${esc(error?.message||'unknown error')}</div>`;}
  finally{busy=false;}
}

function schedule(){
  if(timer)clearInterval(timer);
  timer=setInterval(()=>{if(document.visibilityState==='visible')refresh();},REFRESH_MS);
}

async function downloadEvidence(button){
  const key=button.getAttribute('data-evm-evidence');if(!key)return;const label=button.textContent;button.disabled=true;button.textContent='Preparing…';
  try{
    const response=await fetch('/api/owner/radar/evm-flow/evidence?finding='+encodeURIComponent(key),{credentials:'same-origin',cache:'no-store'});
    if(!response.ok){const data=await response.json().catch(()=>({}));throw new Error(data.error||`HTTP ${response.status}`);}
    const blob=await response.blob();const url=URL.createObjectURL(blob);const link=document.createElement('a');
    link.href=url;link.download='koschei-'+key.replace(/[^a-z0-9-]+/gi,'-').slice(0,90)+'.json';document.body.appendChild(link);link.click();link.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);
    button.textContent='Downloaded';
  }catch(error){button.textContent='Unavailable';button.title=String(error?.message||'unknown error');}
  finally{setTimeout(()=>{button.disabled=false;button.textContent=label;},2500);}
}

function ensurePanel(){
  const root=document.getElementById('securityContent');if(!root||!root.children.length||document.getElementById('ownerEvmFlowRadarPanel'))return;
  const grid=root.querySelector(':scope > .grid.compact-grid')||root;grid.insertAdjacentHTML('beforeend',panelMarkup());document.getElementById('ownerEvmFlowRadarPanel')?.addEventListener('click',event=>{const button=event.target.closest?.('[data-evm-evidence]');if(button)downloadEvidence(button);});refresh();schedule();
}

const root=document.getElementById('securityContent');if(root){new MutationObserver(ensurePanel).observe(root,{childList:true});ensurePanel();}
})();
