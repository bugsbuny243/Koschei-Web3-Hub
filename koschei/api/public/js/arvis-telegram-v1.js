'use strict';
(()=>{
  if(window.__koscheiArvisTelegramV1)return;
  window.__koscheiArvisTelegramV1=true;
  const $=id=>document.getElementById(id);
  const root=$('arvis-telegram');
  if(!root||!window.KoscheiAuth)return;
  const state=$('arvisTelegramState');
  const detail=$('arvisTelegramDetail');
  const pair=$('arvisTelegramPair');
  const pause=$('arvisTelegramPause');
  const resume=$('arvisTelegramResume');
  const disconnect=$('arvisTelegramDisconnect');
  const pairing=$('arvisTelegramPairing');
  const pairLink=$('arvisTelegramPairLink');
  const pairCommand=$('arvisTelegramPairCommand');
  const message=$('arvisTelegramMessage');

  async function api(path,options={}){
    const headers={...(options.headers||{})};
    if(options.body!==undefined&&!headers['Content-Type'])headers['Content-Type']='application/json';
    const response=await KoscheiAuth.apiCall(path,{...options,headers});
    const raw=await response.text();let data={};
    if(raw){try{data=JSON.parse(raw);}catch{throw new Error('Telegram service returned invalid JSON.');}}
    if(!response.ok)throw new Error(String(data?.error||`Telegram request failed with HTTP ${response.status}`));
    return data;
  }

  function show(value,tone=''){
    if(!message)return;
    message.textContent=value;
    message.className=`api-key-message show${tone?` ${tone}`:''}`;
  }
  function resetPairing(){
    if(pairing)pairing.hidden=true;
    if(pairLink)pairLink.removeAttribute('href');
    if(pairCommand)pairCommand.textContent='';
  }
  function render(data){
    const configured=data?.configured===true;
    const current=String(data?.state||'disconnected').toLowerCase();
    if(state){state.textContent=!configured?'UNAVAILABLE':current.toUpperCase();state.className=`access-badge${current==='active'?' good':''}`;}
    if(detail){
      if(!configured)detail.textContent='Telegram provider is not configured on the server.';
      else if(current==='active')detail.textContent=`Connected${data?.recipient_mask?` · ${data.recipient_mask}`:''}. Every completed customer ARVIS scan is mirrored to Telegram.`;
      else if(current==='paused')detail.textContent='Connected but paused. ARVIS scan results remain available in the web workspace.';
      else detail.textContent='Not connected. Pair your Telegram once to receive completed ARVIS scan results there too.';
    }
    if(pair)pair.hidden=!configured||current!=='disconnected';
    if(pause)pause.hidden=current!=='active';
    if(resume)resume.hidden=current!=='paused';
    if(disconnect)disconnect.hidden=current==='disconnected';
    if(current!=='disconnected')resetPairing();
  }

  async function load(){
    try{render(await api('/api/customer/arvis/telegram'));}
    catch(error){if(state)state.textContent='UNAVAILABLE';if(detail)detail.textContent='Telegram connection state could not be loaded.';show(error.message,'bad');}
  }

  pair?.addEventListener('click',async()=>{
    pair.disabled=true;resetPairing();
    try{
      const data=await api('/api/customer/arvis/telegram/pair',{method:'POST',body:JSON.stringify({consent:true})});
      if(!data?.url||!data?.command)throw new Error('Pairing response is incomplete.');
      if(pairLink){pairLink.href=data.url;pairLink.textContent='Open Telegram bot and connect';}
      if(pairCommand)pairCommand.textContent=data.command;
      if(pairing)pairing.hidden=false;
      show('Pairing created. Complete it in Telegram; the code expires automatically.','good');
    }catch(error){show(error.message,'bad');}
    finally{pair.disabled=false;}
  });

  pause?.addEventListener('click',async()=>{pause.disabled=true;try{await api('/api/customer/arvis/telegram',{method:'PATCH',body:JSON.stringify({state:'paused'})});show('ARVIS Telegram delivery paused.','good');await load();}catch(error){show(error.message,'bad');}finally{pause.disabled=false;}});
  resume?.addEventListener('click',async()=>{resume.disabled=true;try{await api('/api/customer/arvis/telegram',{method:'PATCH',body:JSON.stringify({state:'active'})});show('ARVIS Telegram delivery resumed.','good');await load();}catch(error){show(error.message,'bad');}finally{resume.disabled=false;}});
  disconnect?.addEventListener('click',async()=>{if(!window.confirm('Disconnect Telegram from this Koschei account?'))return;disconnect.disabled=true;try{await api('/api/customer/arvis/telegram',{method:'DELETE',body:'{}'});show('Telegram disconnected.','good');await load();}catch(error){show(error.message,'bad');}finally{disconnect.disabled=false;}});

  (async()=>{try{await KoscheiAuth.init();}catch{}if(!KoscheiAuth.requireAuth('/login.html'))return;await load();})();
})();
