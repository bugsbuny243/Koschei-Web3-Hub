(()=>{
'use strict';
if(window.__koscheiCustomerWorkspacePlansV3)return;
window.__koscheiCustomerWorkspacePlansV3=true;
const ready=fn=>document.readyState==='loading'?document.addEventListener('DOMContentLoaded',fn,{once:true}):fn();
const text=value=>String(value??'').trim();

function capability(title,copy,state='available'){
  const article=document.createElement('article');article.className='workspace-capability';article.dataset.state=state;
  article.innerHTML=`<b>${title}</b><span>${copy}</span>`;return article;
}

// Historical package labels may still exist in old records, but the customer
// product has one entitlement only. The server remains the authority.
function normalizedPlan(value){return text(value).toLowerCase()==='professional'?'professional':'none'}
function available(plan){return plan==='professional'}
function planLabel(plan){return available(plan)?'Professional':'No active plan'}

async function loadAccess(){
  try{
    await window.KoscheiAuth?.init?.();
    if(!window.KoscheiAuth?.isLoggedIn?.())return {plan:'none',active:false,signedOut:true};
    const response=await window.KoscheiAuth.apiCall('/api/auth/premium-access',{method:'GET'});
    const data=await response.json().catch(()=>({}));
    const access=data?.access||{};
    return {plan:normalizedPlan(access.plan),active:response.ok&&access.active===true,signedOut:false,remaining:access.outputs_remaining,total:access.outputs_total,paymentProvider:text(access.payment_provider).toLowerCase()};
  }catch{return {plan:'none',active:false,signedOut:false,unavailable:true}}
}

function gateLink(link,plan,label){
  if(!link)return;
  link.dataset.requiredPlan='professional';
  if(available(plan))return;
  link.dataset.planState='locked';
  link.href='/pricing';
  link.title=`${label} requires Professional access`;
}

function annotateWorkspace(plan){
  document.querySelectorAll('.workspace-quick-action').forEach(link=>{
    const href=new URL(link.href,location.origin).pathname;
    if(href==='/watchlist')gateLink(link,plan,'Watchlist and alerts');
    if(href==='/scan')gateLink(link,plan,'Account investigation');
  });
  document.querySelectorAll('.workspace-grid .workspace-card').forEach(card=>{
    card.dataset.requiredPlan='professional';
    card.dataset.planState=available(plan)?'available':'locked';
    if(!available(plan))card.querySelectorAll('a[href="/reports"],a[href="/watchlist"]').forEach(link=>{link.href='/pricing';link.textContent='View Professional';});
  });
}

function render(state){
  const host=document.getElementById('workspaceMissionControl');if(!host||host.dataset.workspacePlansMounted==='1')return;
  host.dataset.workspacePlansMounted='1';
  const plan=state.active?state.plan:'none';
  const section=document.getElementById('workspacePlanStrip')||document.createElement('section');section.className='workspace-plan-strip';section.id='workspacePlanStrip';section.replaceChildren();
  const copy=document.createElement('div');
  const eyebrow=document.createElement('span');eyebrow.className='eyebrow';eyebrow.textContent='Your workspace';
  const h3=document.createElement('h3');h3.textContent=state.signedOut?'Sign in to load your plan':state.unavailable?'Plan service unavailable':`${planLabel(plan)} workspace`;
  const p=document.createElement('p');
  if(state.active&&Number.isFinite(Number(state.remaining))&&Number.isFinite(Number(state.total)))p.textContent=`${state.remaining} of ${state.total} Professional outputs remain. Polar and KOSC are payment methods for the same Professional access; payment method never changes evidence or capability.`;
  else p.textContent='Koschei has one operational customer package: Professional. Polar and KOSC are alternative payment methods for the same server-side entitlement.';
  copy.append(eyebrow,h3,p);
  const badge=document.createElement('span');badge.className='workspace-plan-badge';badge.textContent=state.signedOut?'SIGNED OUT':state.unavailable?'UNAVAILABLE':plan==='none'?'NO ACTIVE PLAN':'PROFESSIONAL';
  section.append(copy,badge);
  const grid=document.createElement('div');grid.className='workspace-capability-grid';
  grid.append(
    capability('Investigations','Professional: canonical ARVIS investigation and account-scoped evidence history.',available(plan)?'available':'locked'),
    capability('Advanced intelligence','Professional: radar, watchlist and supported security-analysis surfaces.',available(plan)?'available':'locked'),
    capability('Developer operations','Professional: registered API credentials and integrations, subject to production readiness and metering.',available(plan)?'available':'locked')
  );
  const wrap=document.createElement('div');wrap.append(section,grid);host.replaceChildren(wrap);
  annotateWorkspace(plan);
}

ready(async()=>render(await loadAccess()));
})();
