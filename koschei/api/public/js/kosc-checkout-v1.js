(()=>{
'use strict';
if(window.__koscheiKOSCCheckoutV1)return;
window.__koscheiKOSCCheckoutV1=true;

const text=value=>String(value??'').trim();
const quoteButtons=()=>Array.from(document.querySelectorAll('[data-kosc-plan]'));
const panel=()=>document.getElementById('koscCheckoutPanel');
const amountNode=()=>document.getElementById('koscQuoteAmount');
const metaNode=()=>document.getElementById('koscQuoteMeta');
const detailNode=()=>document.getElementById('koscSettlementDetails');
const signatureInput=()=>document.getElementById('koscSettlementSignature');
const settlementButton=()=>document.getElementById('koscSettlementButton');
let activeQuote=null;

async function readJSON(response){
  const raw=await response.text().catch(()=> '');
  if(!raw)return {};
  try{return JSON.parse(raw);}catch{return {error:'invalid_json_response'};}
}

function login(){
  const target=window.KoscheiAuth?.loginURL?.('/login.html')||'/login.html?next=%2Fpricing';
  window.location.href=target;
}

function setDetail(value){
  const node=detailNode();
  if(node)node.textContent=value;
}

function formatExpiry(value){
  const date=new Date(value);
  return Number.isFinite(date.getTime())?date.toLocaleString():text(value);
}

async function requestQuote(button){
  if(!button||button.disabled)return;
  const original=button.textContent;
  button.disabled=true;
  button.setAttribute('aria-busy','true');
  button.textContent='Requesting live quote…';
  const box=panel();
  if(box)box.hidden=false;
  activeQuote=null;
  if(settlementButton())settlementButton().disabled=true;
  setDetail('Token holdings alone do not grant access.');
  try{
    if(!window.KoscheiAuth)throw new Error('Koschei account service is unavailable.');
    try{await KoscheiAuth.init();}catch{}
    if(!KoscheiAuth.isLoggedIn()){login();return;}
    const response=await KoscheiAuth.apiCall('/api/kosc/quote',{
      method:'POST',
      headers:{'Content-Type':'application/json'},
      body:JSON.stringify({plan:'professional'}),
      credentials:'same-origin',
    });
    if(!response)throw new Error('KOS quote service is unavailable.');
    const data=await readJSON(response);
    if(response.status===401){login();return;}
    if(!response.ok){
      if(data?.error==='verified_solana_wallet_required')throw new Error('Verify your Solana wallet in Account before requesting a KOS quote.');
      if(data?.error==='commercial_identity_not_provisioned')throw new Error('Your verified account must be provisioned before KOS settlement.');
      if(data?.error==='kosc_checkout_paused'||response.status===503)throw new Error('KOS settlement is not enabled yet.');
      throw new Error('A live KOS quote could not be created.');
    }
    activeQuote=data;
    if(amountNode())amountNode().textContent=text(data?.token_amount)+' KOS';
    if(metaNode())metaNode().textContent='$'+text(data?.usd_price)+' reference · '+text(data?.access_days)+' day access · expires '+formatExpiry(data?.expires_at);
    setDetail('Mint '+text(data?.mint)+' · Treasury '+text(data?.treasury)+' · Quote '+text(data?.quote_id));
    if(settlementButton())settlementButton().disabled=false;
  }catch(error){
    if(amountNode())amountNode().textContent='Quote unavailable';
    if(metaNode())metaNode().textContent=error?.message||'KOS settlement is unavailable.';
  }finally{
    button.disabled=false;
    button.removeAttribute('aria-busy');
    button.textContent=original;
  }
}

async function verifySettlement(){
  const button=settlementButton();
  if(!button||button.disabled||!activeQuote)return;
  const signature=text(signatureInput()?.value);
  if(!signature){setDetail('Enter the finalized Solana transaction signature after sending the quoted KOS amount.');return;}
  const original=button.textContent;
  button.disabled=true;
  button.setAttribute('aria-busy','true');
  button.textContent='Verifying finalized payment…';
  try{
    const response=await KoscheiAuth.apiCall('/api/kosc/settle',{
      method:'POST',
      headers:{'Content-Type':'application/json'},
      body:JSON.stringify({quote_id:text(activeQuote.quote_id),signature}),
      credentials:'same-origin',
    });
    if(!response)throw new Error('KOS settlement service is unavailable.');
    const data=await readJSON(response);
    if(response.status===401){login();return;}
    if(!response.ok){
      if(data?.error==='kosc_quote_expired'||data?.error==='kosc_quote_not_open')throw new Error('This quote is no longer open. Request a new live quote.');
      if(data?.error==='kosc_payment_not_verified')throw new Error('The finalized transaction does not prove the quoted KOS payment from your verified wallet to the configured treasury.');
      if(data?.error==='kosc_settlement_replay_or_conflict')throw new Error('This transaction or quote has already been used.');
      throw new Error('KOS payment could not be verified.');
    }
    activeQuote=null;
    setDetail('Professional access activated from verified KOS settlement. Open Account to confirm your entitlement.');
    button.textContent='Verified';
    if(signatureInput())signatureInput().disabled=true;
    return;
  }catch(error){
    setDetail(error?.message||'KOS payment could not be verified.');
  }finally{
    button.removeAttribute('aria-busy');
    if(activeQuote){
      button.disabled=false;
      button.textContent=original;
    }
  }
}

function bootstrap(){
  quoteButtons().forEach(button=>button.addEventListener('click',()=>requestQuote(button)));
  settlementButton()?.addEventListener('click',verifySettlement);
}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',bootstrap);else bootstrap();
})();
