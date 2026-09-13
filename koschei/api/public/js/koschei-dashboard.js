(()=>{
'use strict';
if(window.__koscheiDashboard)return;
window.__koscheiDashboard=true;

const $=id=>document.getElementById(id);
const text=value=>String(value??'').trim();

function setNav(open){
  document.body.classList.toggle('nav-open',open);
  const trigger=$('mobileMenu');
  if(trigger)trigger.setAttribute('aria-expanded',open?'true':'false');
}

function installNavigation(){
  const trigger=$('mobileMenu');
  trigger?.addEventListener('click',()=>setNav(!document.body.classList.contains('nav-open')));
  document.querySelectorAll('.side-nav a').forEach(link=>link.addEventListener('click',()=>setNav(false)));
  document.addEventListener('click',event=>{
    if(!document.body.classList.contains('nav-open'))return;
    const sidebar=$('sidebar');
    if(sidebar?.contains(event.target)||trigger?.contains(event.target))return;
    setNav(false);
  });
  document.addEventListener('keydown',event=>{if(event.key==='Escape')setNav(false);});
}

function installUniversalScanEntry(){
  const overview=$('overview');
  if(!overview||$('dashboardUniversalScan'))return;
  const style=document.createElement('style');
  style.textContent=`
    .dashboard-scan-entry{margin:18px 0 0;padding:18px;border:1px solid rgba(255,255,255,.08);border-radius:14px;background:linear-gradient(180deg,rgba(13,17,23,.96),rgba(8,11,15,.98))}
    .dashboard-scan-entry>div:first-child{display:flex;justify-content:space-between;gap:24px;align-items:end}.dashboard-scan-entry h2{margin:5px 0 0;font-size:22px}.dashboard-scan-entry p{margin:0;max-width:560px;color:#7f8b96;font-size:12px;line-height:1.55}
    .dashboard-scan-form{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:8px;margin-top:14px}.dashboard-scan-form input{min-height:48px;padding:0 14px;border:1px solid rgba(255,255,255,.1);border-radius:9px;background:#070a0e;color:#f2f5f7;font:700 12px "SFMono-Regular",Consolas,monospace;outline:none}.dashboard-scan-form input:focus{border-color:rgba(101,223,255,.42);box-shadow:0 0 0 3px rgba(101,223,255,.05)}.dashboard-scan-form button{min-width:140px;border:1px solid rgba(101,223,255,.55);border-radius:9px;background:linear-gradient(135deg,#82e6ff,#5ad1f5);color:#061014;font-weight:900}.dashboard-scan-note{margin-top:8px!important;color:#64717c!important;font:700 9px "SFMono-Regular",Consolas,monospace!important;text-transform:uppercase;letter-spacing:.05em}
    @media(max-width:760px){.dashboard-scan-entry>div:first-child{display:block}.dashboard-scan-entry p{margin-top:8px}.dashboard-scan-form{grid-template-columns:1fr}.dashboard-scan-form button{min-height:46px}}
  `;
  document.head.appendChild(style);
  const section=document.createElement('section');
  section.id='dashboardUniversalScan';
  section.className='dashboard-scan-entry';
  section.innerHTML=`<div><div><span class="side-label">Start here</span><h2>Scan any supported address</h2></div><p>Paste a wallet or contract address. Koschei detects the address family and routes it to the supported evidence probes.</p></div><form class="dashboard-scan-form" id="dashboardUniversalScanForm"><input id="dashboardUniversalTarget" autocomplete="off" spellcheck="false" placeholder="0x… / Solana address / Bitcoin address" aria-label="Wallet or contract address"><button type="submit">Scan address</button></form><p class="dashboard-scan-note">One input · read-only · no private keys · unknown stays unknown</p>`;
  const head=overview.querySelector('.page-head');
  if(head)head.insertAdjacentElement('afterend',section);else overview.prepend(section);
  const form=$('dashboardUniversalScanForm');
  const input=$('dashboardUniversalTarget');
  form?.addEventListener('submit',event=>{
    event.preventDefault();
    const target=text(input?.value);
    if(!target){input?.focus();return;}
    location.href=`/scan?target=${encodeURIComponent(target)}`;
  });
}

async function hydrateHealth(){
  const pipeline=$('commandPipelineState');
  const top=$('topStatus');
  const showPipeline=ready=>{
    if(pipeline){pipeline.textContent=ready?'ARVIS PIPELINE OPERATIONAL':'DEGRADED / UNVERIFIED';pipeline.closest('.status-row')?.setAttribute('data-tone',ready?'ready':'unknown');}
    if(top){top.dataset.state=ready?'live':'degraded';top.querySelector('span').textContent=ready?'Evidence pipeline operational':'Pipeline degraded / unverified';}
  };
  const controller=new AbortController();
  const timer=window.setTimeout(()=>controller.abort('health_timeout'),10000);
  try{
    const response=await fetch('/health?evidence=refresh',{cache:'no-store',credentials:'same-origin',signal:controller.signal});
    const data=await response.json().catch(()=>({}));
    if(!response.ok)throw new Error(data.error||data.details||`HTTP ${response.status}`);
    const arvis=data.arvis||{};
    const expiresAt=typeof arvis.cache_expires_at==='string'?Date.parse(arvis.cache_expires_at):NaN;
    const ready=['healthy','operational'].includes(arvis.pipeline_status)&&arvis.cached===true&&expiresAt>Date.now();
    showPipeline(ready);
    if(ready)window.setTimeout(()=>showPipeline(false),Math.min(Math.max(0,expiresAt-Date.now()),2147483647));
  }catch(error){
    if(pipeline){pipeline.textContent='UNAVAILABLE';pipeline.closest('.status-row')?.setAttribute('data-tone','unknown');}
    if(top){top.dataset.state='degraded';top.querySelector('span').textContent='Evidence service unavailable';top.title=text(error?.message||error);}
  }finally{window.clearTimeout(timer);window.setTimeout(hydrateHealth,15000);}
}

function syncAccountState(){
  const source=$('workspaceLiveState');
  const target=$('commandAccountState');
  if(target){
    target.textContent=text(source?.textContent)||'UNAVAILABLE';
    const tone=source?.dataset?.state==='live'?'ready':'unknown';
    target.closest('.status-row')?.setAttribute('data-tone',tone);
  }
  const jobs=$('workspaceReportsKpi')?.querySelector('strong');
  const jobsTarget=$('commandInvestigationState');
  if(jobsTarget)jobsTarget.textContent=text(jobs?.textContent)||'—';
}

function watchAccountState(){
  syncAccountState();
  const observer=new MutationObserver(syncAccountState);
  for(const id of ['workspaceLiveState','workspaceReportsKpi']){
    const node=$(id);
    if(node)observer.observe(node,{subtree:true,childList:true,characterData:true,attributes:true});
  }
}

function installSectionTracking(){
  if(!('IntersectionObserver'in window))return;
  const links=[...document.querySelectorAll('.side-nav a[href^="#"]')];
  const byId=new Map(links.map(link=>[link.getAttribute('href').slice(1),link]));
  const observer=new IntersectionObserver(entries=>{
    const visible=entries.filter(entry=>entry.isIntersecting).sort((a,b)=>b.intersectionRatio-a.intersectionRatio)[0];
    if(!visible)return;
    links.forEach(link=>link.removeAttribute('aria-current'));
    byId.get(visible.target.id)?.setAttribute('aria-current','page');
  },{rootMargin:'-20% 0px -65% 0px',threshold:[.05,.2,.5]});
  byId.forEach((_,id)=>{const section=$(id);if(section)observer.observe(section);});
}

function mount(){
  installNavigation();
  installUniversalScanEntry();
  installSectionTracking();
  watchAccountState();
  hydrateHealth();
}

if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',mount,{once:true});else mount();
})();
