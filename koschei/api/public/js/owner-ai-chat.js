(()=>{
'use strict';

const state={threadId:'',threads:[],messages:[],model:'router-default',aiReady:false,sending:false};
const $=id=>document.getElementById(id);
const esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
const formatTime=value=>{if(!value)return'';const date=new Date(value);return Number.isNaN(date.getTime())?'':new Intl.DateTimeFormat('tr-TR',{hour:'2-digit',minute:'2-digit'}).format(date)};

async function request(path,opt={}){
  const headers=new Headers(opt.headers||{});
  if(opt.body&&!headers.has('Content-Type'))headers.set('Content-Type','application/json');
  const response=await fetch(path,{credentials:'same-origin',...opt,headers});
  let data={};try{data=await response.json()}catch{}
  if(!response.ok||data.ok===false){const error=new Error(data.detail||data.message||data.error||`İstek başarısız (${response.status})`);error.status=response.status;throw error}
  return data;
}

function ensureAssets(){
  if(!document.querySelector('link[data-owner-ai-chat]')){
    const link=document.createElement('link');link.rel='stylesheet';link.href='/css/owner-ai-chat.css?v=2';link.dataset.ownerAiChat='1';document.head.appendChild(link);
  }
}

function mount(root){
  if(!root||root.querySelector('#ownerChatShell'))return;
  ensureAssets();
  root.innerHTML=`<div class="owner-chat-shell" id="ownerChatShell">
    <aside class="owner-chat-sidebar">
      <div class="owner-chat-sidebar-head"><div><span class="eyebrow">Özel çalışma alanı</span><h2>Owner AI Chat</h2></div><button class="btn small primary" id="ownerChatNew" type="button">Yeni</button></div>
      <div class="owner-chat-quick">
        <button data-owner-prompt="ARVIS radarının şu anki sağlığını yalnız üretim verilerine göre değerlendir. Kritik hata ve eksikleri sırala." type="button">Radar sağlığı</button>
        <button data-owner-prompt="Mevcut kullanıcı, aktif paket, geri bildirim ve gelir verilerine göre müşteri kazanımı için en kritik sonraki adımları sırala." type="button">Müşteri ve büyüme</button>
        <button data-owner-prompt="Shopier ödeme talepleri, onaylar ve entitlement akışında dikkat edilmesi gereken noktaları özetle." type="button">Ödeme ve paketler</button>
        <button data-owner-prompt="Son güvenlik olayları ve müşteri geri bildirimlerine göre owner müdahalesi gerektiren maddeleri önceliklendir." type="button">Güvenlik ve geri bildirim</button>
      </div>
      <div class="owner-chat-thread-title">Konuşmalar</div>
      <div class="owner-chat-threads" id="ownerChatThreads"><div class="empty">Yükleniyor…</div></div>
    </aside>
    <section class="owner-chat-main">
      <header class="owner-chat-head">
        <div><span class="eyebrow">Salt okunur operasyon copilotu</span><h2 id="ownerChatTitle">Yeni sohbet</h2></div>
        <div class="owner-chat-head-actions"><span class="badge warn" id="ownerChatModel">Kontrol ediliyor</span><button class="btn small danger hidden" id="ownerChatDelete" type="button">Sil</button></div>
      </header>
      <div class="owner-chat-messages" id="ownerChatMessages"></div>
      <form class="owner-chat-composer" id="ownerChatForm">
        <textarea id="ownerChatInput" maxlength="4000" placeholder="Radar, müşteriler, gelir, geri bildirim veya sistem sağlığı hakkında sor…"></textarea>
        <button class="btn primary" id="ownerChatSend" type="submit">Gönder</button>
      </form>
      <div class="owner-chat-note" id="ownerChatNote">Bu sohbet analiz yapar; üretim değişikliği veya para hareketi gerçekleştirmez.</div>
    </section>
  </div>`;
  bind();loadHistory();
}

function bind(){
  $('ownerChatNew').onclick=newChat;
  $('ownerChatForm').onsubmit=sendMessage;
  $('ownerChatInput').onkeydown=event=>{if(event.key==='Enter'&&!event.shiftKey){event.preventDefault();$('ownerChatForm').requestSubmit()}};
  $('ownerChatDelete').onclick=deleteThread;
  document.querySelectorAll('[data-owner-prompt]').forEach(button=>button.onclick=()=>{$('ownerChatInput').value=button.dataset.ownerPrompt;$('ownerChatInput').focus()});
}

async function loadHistory(threadId=''){
  const query=threadId?`?thread_id=${encodeURIComponent(threadId)}`:'';
  setNote('Konuşmalar yükleniyor…');
  try{
    const data=await request('/api/owner/chat'+query);
    state.threadId=data.thread_id||'';state.threads=data.threads||[];state.messages=data.messages||[];state.model=data.model||'router-default';state.aiReady=Boolean(data.ai_ready);
    renderAll();setNote(state.aiReady?'Mesajlar owner hesabına özel olarak Neon’da saklanıyor.':'AI sağlayıcısı yapılandırılmamış.');
  }catch(error){state.messages=[];renderMessages(error.message);setNote(error.message,true)}
}

function renderAll(){renderThreads();renderMessages();renderHeader()}
function renderThreads(){
  const root=$('ownerChatThreads');if(!root)return;
  if(!state.threads.length){root.innerHTML='<div class="empty">Henüz konuşma yok.</div>';return}
  root.innerHTML=state.threads.map(thread=>`<button class="owner-chat-thread ${thread.id===state.threadId?'active':''}" data-thread-id="${esc(thread.id)}" type="button"><b>${esc(thread.title)}</b><span>${esc(new Date(thread.updated_at).toLocaleDateString('tr-TR'))}</span></button>`).join('');
  root.querySelectorAll('[data-thread-id]').forEach(button=>button.onclick=()=>loadHistory(button.dataset.threadId));
}
function renderHeader(){
  const current=state.threads.find(thread=>thread.id===state.threadId);
  $('ownerChatTitle').textContent=current?.title||'Yeni sohbet';$('ownerChatModel').textContent=state.aiReady?state.model:'AI eksik';$('ownerChatModel').className=`badge ${state.aiReady?'ok':'bad'}`;$('ownerChatDelete').classList.toggle('hidden',!state.threadId);$('ownerChatSend').disabled=!state.aiReady||state.sending;
}
function renderMessages(error=''){
  const root=$('ownerChatMessages');if(!root)return;
  if(error){root.innerHTML=`<div class="error-box">${esc(error)}</div>`;return}
  if(!state.messages.length){root.innerHTML='<div class="owner-chat-welcome"><div class="owner-chat-orb">K</div><h3>Koschei Owner Copilot</h3><p>Radar, müşteriler, gelir, geri bildirim, güvenlik ve sistem sağlığını üretim verileri üzerinden değerlendirebilirsin.</p></div>';return}
  root.innerHTML=state.messages.map(message=>messageBubble(message)).join('')+(state.sending?'<div class="owner-chat-row assistant"><div class="owner-chat-avatar">K</div><div class="owner-chat-bubble typing"><i></i><i></i><i></i></div></div>':'');root.scrollTop=root.scrollHeight;
}
function messageBubble(message){const assistant=message.role==='assistant';const content=esc(message.content).replace(/\n/g,'<br>');return`<div class="owner-chat-row ${assistant?'assistant':'user'}"><div class="owner-chat-avatar">${assistant?'K':'O'}</div><div class="owner-chat-bubble"><div class="owner-chat-content">${content}</div><time>${formatTime(message.created_at)}</time></div></div>`}

async function sendMessage(event){
  event.preventDefault();if(state.sending||!state.aiReady)return;
  const input=$('ownerChatInput'),message=input.value.trim();if(!message)return;
  input.value='';state.sending=true;state.messages.push({id:`local-${Date.now()}`,role:'user',content:message,created_at:new Date().toISOString()});renderMessages();renderHeader();setNote('Koschei düşünüyor…');
  try{
    const data=await request('/api/owner/chat',{method:'POST',body:JSON.stringify({thread_id:state.threadId,message})});
    state.threadId=data.thread_id||state.threadId;state.messages=state.messages.filter(item=>!String(item.id).startsWith('local-'));if(data.user_message)state.messages.push(data.user_message);if(data.assistant_message)state.messages.push(data.assistant_message);state.model=data.model||state.model;state.sending=false;renderMessages();renderHeader();setNote('Yanıt hazır.');await refreshThreads();
  }catch(error){state.sending=false;renderMessages();setNote(error.message,true);input.value=message}
}
async function refreshThreads(){try{const data=await request('/api/owner/chat'+(state.threadId?`?thread_id=${encodeURIComponent(state.threadId)}`:''));state.threads=data.threads||state.threads;renderThreads();renderHeader()}catch{}}
function newChat(){state.threadId='';state.messages=[];renderAll();$('ownerChatInput').focus();setNote('Yeni konuşma hazır.')}
async function deleteThread(){if(!state.threadId||!confirm('Bu owner sohbetini silmek istediğine emin misin?'))return;try{await request(`/api/owner/chat?thread_id=${encodeURIComponent(state.threadId)}`,{method:'DELETE'});state.threadId='';state.messages=[];await loadHistory()}catch(error){setNote(error.message,true)}}
function setNote(message,bad=false){const note=$('ownerChatNote');if(!note)return;note.textContent=message;note.className=`owner-chat-note${bad?' bad':''}`}
function tryMount(){const root=$('brainContent');if(root&&root.closest('.page')?.classList.contains('active')&&!root.querySelector('#ownerChatShell'))mount(root)}
const observer=new MutationObserver(tryMount);observer.observe(document.documentElement,{childList:true,subtree:true});setInterval(tryMount,700);window.OwnerAIChat={mount};
})();
