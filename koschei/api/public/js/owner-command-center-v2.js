(()=>{
  'use strict';
  if(window.__ownerCommandCenterV2)return;
  window.__ownerCommandCenterV2=true;
  const pages=[
    {id:'command',icon:'⌂',label:'Operasyon özeti',copy:'KPI, canlı servisler ve hızlı geçişler'},
    {id:'arvis',icon:'◉',label:'ARVIS tam tarama',copy:'Kanıt, threat anticipation ve imzalı sonuç'},
    {id:'customers',icon:'◎',label:'Müşteriler',copy:'Hesap, cüzdan ve erişim durumu'},
    {id:'access',icon:'◈',label:'KOSCH erişim',copy:'Holder, tier ve kota görünümü'},
    {id:'feedback',icon:'✦',label:'Geri bildirim',copy:'Müşteri sinyalleri ve ürün talepleri'},
    {id:'security',icon:'◇',label:'Güvenlik olayları',copy:'Denetim ve güvenlik kayıtları'},
    {id:'system',icon:'⚙',label:'Sistem sağlığı',copy:'Üretim servisleri ve bağımlılıklar'},
    {id:'brain',icon:'◆',label:'Koschei Brain',copy:'Owner konuşma ve açıklama paneli'}
  ];
  const ready=fn=>document.readyState==='loading'?document.addEventListener('DOMContentLoaded',fn,{once:true}):fn();
  const $=selector=>document.querySelector(selector);
  const esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));

  function jump(id){
    const page=pages.find(item=>item.id===id);
    if(!page)return;
    const target=[...document.querySelectorAll('[data-nav]')].find(node=>node.dataset.nav===page.id);
    if(target)target.click();
    closePalette();
  }

  function paletteMarkup(){
    return `<div class="owner-command-backdrop" id="ownerCommandBackdrop" aria-hidden="true"><section class="owner-command-palette" role="dialog" aria-modal="true" aria-labelledby="ownerCommandTitle"><span class="eyebrow">Owner command palette</span><h2 id="ownerCommandTitle" style="margin:6px 0 12px">Nereye gitmek istiyorsun?</h2><input class="owner-command-search" id="ownerCommandSearch" autocomplete="off" placeholder="Sayfa veya görev ara…"><div class="owner-command-list" id="ownerCommandList"></div></section></div>`;
  }

  function renderPalette(filter=''){
    const list=$('#ownerCommandList');
    if(!list)return;
    const query=filter.trim().toLocaleLowerCase('tr-TR');
    const visible=pages.filter(page=>`${page.label} ${page.copy}`.toLocaleLowerCase('tr-TR').includes(query));
    list.innerHTML=visible.map((page,index)=>`<button class="owner-command-item${index===0?' active':''}" type="button" data-owner-command="${esc(page.id)}"><i>${esc(page.icon)}</i><span><b>${esc(page.label)}</b><span>${esc(page.copy)}</span></span><kbd>Alt+${pages.findIndex(item=>item.id===page.id)+1}</kbd></button>`).join('')||'<div class="empty compact">Eşleşen komut yok.</div>';
    list.querySelectorAll('[data-owner-command]').forEach(button=>button.addEventListener('click',()=>jump(button.dataset.ownerCommand)));
  }

  function openPalette(){
    const backdrop=$('#ownerCommandBackdrop');
    if(!backdrop)return;
    backdrop.classList.add('open');
    backdrop.setAttribute('aria-hidden','false');
    renderPalette('');
    const input=$('#ownerCommandSearch');
    if(input){input.value='';window.setTimeout(()=>input.focus(),30)}
  }

  function closePalette(){
    const backdrop=$('#ownerCommandBackdrop');
    if(!backdrop)return;
    backdrop.classList.remove('open');
    backdrop.setAttribute('aria-hidden','true');
  }

  function installPalette(){
    document.body.insertAdjacentHTML('beforeend',paletteMarkup());
    const actions=$('.top-actions');
    if(actions){
      const button=document.createElement('button');
      button.className='btn owner-command-button';
      button.id='ownerCommandButton';
      button.type='button';
      button.innerHTML='<span>Komut ara</span><kbd>⌘K</kbd>';
      button.addEventListener('click',openPalette);
      actions.insertBefore(button,actions.firstChild);
    }
    const backdrop=$('#ownerCommandBackdrop');
    const search=$('#ownerCommandSearch');
    backdrop?.addEventListener('click',event=>{if(event.target===backdrop)closePalette()});
    search?.addEventListener('input',event=>renderPalette(event.target.value));
    search?.addEventListener('keydown',event=>{
      if(event.key==='Enter'){
        const active=$('#ownerCommandList .owner-command-item.active')||$('#ownerCommandList .owner-command-item');
        if(active)jump(active.dataset.ownerCommand);
      }
    });
    document.addEventListener('keydown',event=>{
      if((event.metaKey||event.ctrlKey)&&event.key.toLowerCase()==='k'){event.preventDefault();openPalette();return}
      if(event.key==='Escape'){closePalette();return}
      if(event.altKey&&/^[1-8]$/.test(event.key)){event.preventDefault();jump(pages[Number(event.key)-1].id)}
    });
  }

  function missionBrief(){
    return `<section class="owner-mission-brief" data-owner-enhanced="mission"><article class="owner-mission-copy"><span class="eyebrow">Koschei Threat Operations</span><h2>Kanıtı topla, açık yolu gör, teknik sonucu hızlandır.</h2><p>Owner merkezi artık yalnız servis sayılarını göstermiyor. ARVIS taraması; holder kontrolü, likidite yolu, aktör ilişkisi, threat anticipation ve imzalı sonuç zincirini tek operasyon akışında topluyor.</p><div class="owner-mission-tags"><span>Deterministik sonuç</span><span>Threat anticipation</span><span>Evidence-first</span><span>Aynı kanıt sözleşmesi</span></div></article><aside class="owner-mission-actions"><button class="owner-jump" type="button" data-owner-jump="arvis"><i>◉</i><span><b>Yeni araştırma başlat</b><small>Mint, cüzdan, program veya işlem tara.</small></span><em>→</em></button><button class="owner-jump" type="button" data-owner-jump="security"><i>◇</i><span><b>Güvenlik olayları</b><small>Son denetim ve alarm kayıtlarını incele.</small></span><em>→</em></button><button class="owner-jump" type="button" data-owner-jump="system"><i>⚙</i><span><b>Sistem sağlığı</b><small>RPC, veritabanı ve servis durumunu kontrol et.</small></span><em>→</em></button></aside></section>`;
  }

  function caseFlow(){
    const steps=[
      ['01','Hedef','Mint, wallet, program veya işlem'],
      ['02','Kanıt collectorları','Authority, holder, funding, creator, flow'],
      ['03','Threat anticipation','Açık, kapalı, observed ve unknown yollar'],
      ['04','İmzalı sonuç','Tek deterministik sonuç ve evidence policy'],
      ['05','Bağımsız kontrol','Aynı kanıt paketinin çoklu-model tutarlılık incelemesi']
    ];
    return `<section class="owner-case-flow" data-owner-enhanced="case-flow">${steps.map(step=>`<div class="owner-case-step"><small>${step[0]}</small><b>${step[1]}</b><span>${step[2]}</span></div>`).join('')}</section>`;
  }

  function bindInjectedActions(root=document){
    root.querySelectorAll('[data-owner-jump]').forEach(button=>{
      if(button.dataset.bound==='1')return;
      button.dataset.bound='1';
      button.addEventListener('click',()=>jump(button.dataset.ownerJump));
    });
  }

  function enhanceCommand(){
    const root=$('#commandContent');
    if(!root||!root.children.length||root.querySelector('[data-owner-enhanced="mission"]'))return;
    root.insertAdjacentHTML('afterbegin',missionBrief());
    bindInjectedActions(root);
  }

  function enhanceArvis(){
    const root=$('#arvisContent');
    if(!root||!root.children.length||root.querySelector('[data-owner-enhanced="case-flow"]'))return;
    root.insertAdjacentHTML('afterbegin',caseFlow());
  }

  function installObservers(){
    ['#commandContent','#arvisContent'].forEach(selector=>{
      const root=$(selector);
      if(!root)return;
      const observer=new MutationObserver(()=>{enhanceCommand();enhanceArvis()});
      observer.observe(root,{childList:true,subtree:false});
    });
    enhanceCommand();
    enhanceArvis();
  }

  function syncTitle(){
    const title=$('#pageTitle');
    if(!title)return;
    const observer=new MutationObserver(()=>{document.title=`${title.textContent.trim()} · Koschei Owner`});
    observer.observe(title,{childList:true,characterData:true,subtree:true});
  }

  ready(()=>{installPalette();installObservers();syncTitle();bindInjectedActions()});
})();
