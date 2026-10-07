(() => {
 'use strict';
 const $ = (id) => document.getElementById(id);
 const auth = window.KoscheiAuth;
 let subscriptions = [], items = [], sources = [], loggedIn = false;
 let pairTimer;
 const labels = {bitcoin:'Bitcoin',ethereum:'Ethereum',solana:'Solana',global:'Genel',security:'Güvenlik',network:'Ağ',defi:'DeFi',regulation:'Düzenleme',market:'Piyasa',general:'Diğer'};
 const message = (text, error = false) => { $('briefMessage').textContent=text; $('briefMessage').dataset.error=String(error); };
 const selected = (name) => Array.from(document.querySelectorAll(`input[name="${name}"]:checked`), x => x.value);
 function preferences() { return {networks:selected('network'),topics:selected('topic'),cadence:$('briefCadence').value,timezone:$('briefTimezone').value,quiet_start:Number($('briefQuietStart').value),quiet_end:Number($('briefQuietEnd').value)}; }
 function setPreferences(p) { for (const [name, values] of [['network',p.networks],['topic',p.topics]]) for (const input of document.querySelectorAll(`input[name="${name}"]`)) input.checked=(values||[]).includes(input.value); $('briefCadence').value=p.cadence; $('briefTimezone').value=p.timezone; $('briefQuietStart').value=p.quiet_start; $('briefQuietEnd').value=p.quiet_end; }
 async function request(path, options = {}) {
  const response = await auth.apiCall(path, options);
  if (!response) throw new Error('Bağlantı kurulamadı. Tekrar deneyin.');
  if (response.status===401) throw new Error('Oturumunuz sona erdi. Tekrar giriş yapın.');
  let value; try {value=await response.json();} catch {throw new Error('Sunucudan yanıt alınamadı.');}
  if (!response.ok) { const errors={channel_not_configured:'Telegram henüz sunucuya bağlanmadı.',pairing_rate_limited:'Yeni bağlantı için bir dakika bekleyin.',channel_pairing_failed:'Telegram’ı yeniden bağlamak için önce bağlantıyı kaldırın.',explicit_consent_required:'Bildirim iznini seçin.'};throw new Error(errors[value.error]||'İşlem tamamlanamadı. Tekrar deneyin.'); }
  return value;
 }
 function renderFeed() {
  const target=$('briefFeed');target.replaceChildren();const networks=selected('network'),topics=selected('topic');
  const visible=items.filter(x=>(!networks.length||x.networks.some(n=>networks.includes(n)))&&(!topics.length||x.topics.some(t=>topics.includes(t))));
  if (!visible.length) {target.textContent='Bu konular için güncel haber bulunamadı.';return;}
  for(const item of visible){ let url;try{url=new URL(item.url);}catch{continue;}const source=sources.find(x=>x.id===item.source_id);if(!source||url.protocol!=='https:'||url.hostname!==new URL(source.feed_url).hostname)continue;
   const article=document.createElement('article');article.className='brief-item';const link=document.createElement('a');link.href=url.href;link.target='_blank';link.rel='noopener noreferrer';link.textContent=item.title;
   const meta=document.createElement('small');meta.textContent=`${source.name} · ${new Date(item.published_at).toLocaleString('tr-TR',{timeZone:$('briefTimezone').value||'Europe/Istanbul'})}`;
   const tags=document.createElement('div');tags.className='brief-item-tags';tags.textContent=[...item.networks,...item.topics].map(x=>labels[x]||x).join(' · ');article.append(link,meta,tags);target.append(article);
  }
 }
 async function loadFeed() {try{const r=await fetch('/api/crypto-brief/feed',{cache:'no-store'});if(!r.ok)throw new Error();const v=await r.json();items=v.items||[];sources=v.sources||[];const health=v.source_status||[];$('briefSourceState').textContent=sources.map(s=>{const h=health.find(x=>x.id===s.id);return `${s.name}: ${h?.fresh?'güncel':h?.error_code?'kaynağa ulaşılamıyor':'güncelleme bekleniyor'}`;}).join(' · ');renderFeed();}catch{$('briefFeed').textContent='Haber akışına şu an ulaşılamıyor. Yenile düğmesiyle tekrar deneyin.';}}
 function button(text,action,disabled=false){const b=document.createElement('button');b.type='button';b.className='ops-btn';b.textContent=text;b.disabled=disabled;b.addEventListener('click',action);return b;}
 function renderChannels(){const target=$('briefChannels');target.replaceChildren();for(const sub of subscriptions.filter(x=>x.channel==='telegram')){const box=document.createElement('article');box.className='brief-channel';const title=document.createElement('strong');title.textContent='Telegram';const state=document.createElement('p');const states={active:'Bağlı · bildirimler açık',paused:'Bağlı · bildirimler durduruldu',disconnected:'Bağlı değil'};state.textContent=sub.configured?`${states[sub.state]||sub.state}${sub.recipient_mask?' · '+sub.recipient_mask:''}`:'Sunucu bağlantısı hazırlanıyor';box.append(title,state);
  const actions=document.createElement('div');actions.className='brief-channel-actions';if(sub.state==='disconnected')actions.append(button('Bağlan',()=>pair('telegram'),!sub.configured));else{actions.append(button(sub.state==='active'?'Durdur':'Devam et',()=>change('telegram',sub.state==='active'?'paused':'active')));actions.append(button('Bağlantıyı kaldır',()=>change('telegram','disconnected')));}box.append(actions);target.append(box);}}
 async function loadSubscriptions(initial=false){const v=await request('/api/customer/crypto-brief');subscriptions=(v.subscriptions||[]).filter(x=>x.channel==='telegram');if(initial&&subscriptions[0])setPreferences(subscriptions.find(s=>s.state==='active')?.preferences||subscriptions[0].preferences);renderChannels();renderFeed();}
 async function save(showMessage=true){const p=preferences();await request('/api/customer/crypto-brief',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({channel:'telegram',preferences:p})});if(showMessage)message('Tercihleriniz kaydedildi.');}
 async function pair(){try{if(!$('briefConsent').checked){message('Bağlanmadan önce bildirim iznini seçin.',true);return;}await save(false);const p=await request('/api/customer/crypto-brief/pair',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({channel:'telegram',consent:true})});$('briefPairLink').href=p.url;$('briefPairCommand').textContent=p.command;$('briefPairing').hidden=false;clearTimeout(pairTimer);pairTimer=setTimeout(()=>{$('briefPairing').hidden=true;$('briefPairCommand').textContent='';$('briefPairLink').removeAttribute('href');},Math.max(0,Math.min(600000,new Date(p.expires_at)-Date.now())));message('Telegram’da mesajı gönderince bağlantı tamamlanır.');}catch(e){message(e.message,true);}}
 async function change(channel,state){try{await request('/api/customer/crypto-brief',{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({channel,state})});await loadSubscriptions();message(state==='disconnected'?'Telegram bağlantısı kaldırıldı.':state==='paused'?'Bildirimler durduruldu.':'Bildirimler açıldı.');}catch(e){message(e.message,true);}}
 $('briefPreferences').addEventListener('submit',async e=>{e.preventDefault();$('briefSave').disabled=true;try{await save();await loadSubscriptions();}catch(err){message(err.message,true);}finally{$('briefSave').disabled=false;}});
 $('briefRefresh').addEventListener('click',async()=>{await loadFeed();if(loggedIn)try{await loadSubscriptions();}catch(e){message(e.message,true);}});
 document.querySelectorAll('input[name=network],input[name=topic]').forEach(x=>x.addEventListener('change',renderFeed));$('briefTimezone').addEventListener('change',renderFeed);
 window.addEventListener('focus',()=>{if(loggedIn)loadSubscriptions().catch(()=>{});});
 (async()=>{await loadFeed();try{await auth.init();loggedIn=auth.isLoggedIn();}catch{loggedIn=false;}if(!loggedIn){$('briefAuthState').textContent='Haber akışını inceleyebilirsiniz. Bildirimleri bağlamak için giriş yapın.';$('briefLogin').hidden=false;return;}$('briefAuthState').textContent='Haber tercihlerinizi ve Telegram bağlantınızı buradan yönetebilirsiniz.';$('briefPreferences').hidden=false;try{await loadSubscriptions(true);}catch(e){message(e.message,true);}})();
})();
