(()=>{
  'use strict';

  const OVERVIEW_API='/api/owner/arvis';
  let busy=false;
  const $=id=>document.getElementById(id);
  const arr=v=>Array.isArray(v)?v:[];
  const obj=v=>v&&typeof v==='object'&&!Array.isArray(v)?v:{};
  const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const money=v=>{const n=Number(v||0);if(n>=1e9)return'$'+(n/1e9).toFixed(2)+'B';if(n>=1e6)return'$'+(n/1e6).toFixed(2)+'M';if(n>=1e3)return'$'+(n/1e3).toFixed(1)+'K';return'$'+new Intl.NumberFormat('en-US',{maximumFractionDigits:2}).format(n)};
  const short=v=>{const s=String(v||'');return s.length>20?s.slice(0,8)+'…'+s.slice(-6):s||'—'};
  const dt=v=>{const d=new Date(v);return Number.isNaN(d.getTime())?'—':new Intl.DateTimeFormat('tr-TR',{day:'2-digit',month:'2-digit',hour:'2-digit',minute:'2-digit'}).format(d)};
  const BLOCKED_NAME_PARTS=['nigg','niga','n1gg','faggot','fagg','retard','kike','k1ke','chink','spic','tranny','rape','coon'];
  const symbol=v=>{const raw=String(v||'TOKEN');const low=raw.toLowerCase().replace(/[^a-z0-9]/g,'');if(BLOCKED_NAME_PARTS.some(p=>low.includes(p)))return'TOKEN';return raw.replace(/[^a-zA-Z0-9_$-]/g,'').slice(0,18)||'TOKEN';};

  async function request(path,opt={}){
    const response=await fetch(path,{credentials:'same-origin',...opt,headers:{'Content-Type':'application/json',...(opt.headers||{})}});
    let data={};try{data=await response.json()}catch{}
    if(!response.ok||data.ok===false)throw new Error(data.message||data.detail||data.error||`İstek başarısız (${response.status})`);
    return data;
  }

  function note(text){let n=document.querySelector('.publish-toast');if(n)n.remove();n=document.createElement('div');n.className='publish-toast';n.textContent=text;document.body.appendChild(n);setTimeout(()=>n.remove(),2600)}
  async function copyText(text){try{await navigator.clipboard.writeText(text);note('Kanıt özeti kopyalandı.')}catch{note('Metin kopyalanamadı.')}}

  function actorStatusLabel(status){
    return ({
      verified_linked_actor_network:'BAĞLANTILI AKTÖR AĞI DOĞRULANDI',
      verified_actor_observation_no_cross_link:'SINIRLI PENCEREDE GÜÇLÜ ÇAPRAZ BAĞ YOK',
      partial_actor_observation:'AKTÖR KANITI KISMİ',
      actor_evidence_unavailable:'AKTÖR KANITI YOK'
    })[String(status||'')]||'KANIT BEKLİYOR';
  }

  function publishingEligibility(actor){
    const a=obj(actor),coverage=obj(a.coverage);
    const status=String(a.status||'');
    const creatorVerified=coverage.creator_relation_verified===true;
    const creatorTx=Number(coverage.creator_transactions_checked||0);
    const holderAnalyzed=Number(coverage.holder_wallets_analyzed||0);
    const eligible=(status==='verified_linked_actor_network'||status==='verified_actor_observation_no_cross_link')&&creatorVerified&&creatorTx>0&&holderAnalyzed>=3;
    const reasons=[];
    if(!creatorVerified)reasons.push('creator/deployer kök cüzdanı doğrulanmadı');
    if(creatorTx<1)reasons.push('creator işlem geçmişi parse edilmedi');
    if(holderAnalyzed<3)reasons.push(`holder davranış kapsamı yetersiz (${holderAnalyzed}/3 minimum)`);
    if(status==='partial_actor_observation'||status==='actor_evidence_unavailable')reasons.push('aktör ağı yalnız kısmi veya kullanılamaz durumda');
    return {eligible,reasons,status,coverage};
  }

  function queueCard(item,index){
    const s=symbol(item.symbol||item.name);
    const completed=item.report_status==='completed';
    return `<article class="publish-token" data-risk="unknown">
      <div class="publish-token-top">
        <div class="publish-identity">
          <div class="publish-symbol"><span class="publish-symbol-mark">${esc(s.replace('$','').slice(0,2).toUpperCase())}</span><span>$${esc(s)}</span></div>
          <div class="publish-name">Pump 500K+ keşfi · ${esc(dt(item.observed_at))}</div>
          <div class="publish-mint">${esc(short(item.target))}</div>
        </div>
        <div class="publish-risk unknown"><div><b>${completed?'ÖN':'—'}</b><span>${completed?'TARAMA':'BEKLİYOR'}</span></div></div>
      </div>
      <div class="publish-metrics">
        <div class="publish-metric"><span>24s hacim</span><b>${esc(money(item.volume_24h_usd))}</b></div>
        <div class="publish-metric"><span>Likidite</span><b>${esc(money(item.liquidity_usd))}</b></div>
        <div class="publish-metric"><span>Yayın durumu</span><b>İNCELENMEDİ</b></div>
      </div>
      <div class="publish-insight">Bu yalnız hacim eşiği kaydıdır. Creator, funder ve Top-holder bağlantıları doğrulanmadan X metni veya görsel üretilemez.</div>
      <div class="publish-actions"><button class="publish-action primary" data-pack="${index}" type="button">Aktör ağını incele</button></div>
    </article>`;
  }

  function history(items){return arr(items).slice(0,10).map(x=>`<div class="publish-history-row"><b>${esc(short(x.target))}</b><span>${esc(x.module_id||'verdict')}</span><span>${esc(dt(x.created_at))}</span></div>`).join('')}

  function render(data){
    const root=$('arvisContent');if(!root)return;
    const auto=arr(data.high_volume_pump),items=arr(data.items),pipeline=obj(data.pipeline),threshold=Number(pipeline.pump_volume_threshold_usd||500000);
    root.innerHTML=`<div class="publishing-shell" data-publishing-shell>
      <section class="publish-hero"><div class="publish-hero-copy"><span class="publish-kicker">OWNER-ONLY SOLANA SECURITY</span><h2>500K+ hacim yalnız keşiftir; yayın kararı değildir.</h2><p>Koschei önce creator/deployer, funder, Top-holder ve token çıkış bağlantılarını doğrular. Kanıt ağı tamamlanmadan X metni ve görsel düğmeleri açılmaz.</p></div>
      <div class="publish-stats"><div class="publish-stat"><span>Keşfedilen</span><b>${auto.length}</b><small>${money(threshold)}+ hacim</small></div><div class="publish-stat"><span>Otomatik yayın</span><b>KAPALI</b><small>Owner onayı zorunlu</small></div><div class="publish-stat"><span>Kanıt kapısı</span><b>AKTİF</b><small>Actor evidence required</small></div></div></section>
      <section class="publish-scan"><div class="publish-scan-copy"><b>Belirli token için aktör incelemesi</b><span>Mint gir; tam ARVIS ve aktör ağı birlikte çalışsın.</span></div><form class="publish-scan-form" id="publishScanForm"><input class="input mono" id="publishScanMint" placeholder="Solana token mint adresi"><button class="publish-action primary" type="submit">Tam incele</button></form></section>
      <div class="publish-section-head"><div><span class="publish-kicker">HACİM KEŞİF KUYRUĞU</span><h3>Henüz paylaşım paketi değil</h3><p>Her token önce aktör kanıt kapısından geçer.</p></div><span class="publish-count">${auto.length} TOKEN</span></div>
      ${auto.length?`<section class="publish-grid">${auto.map(queueCard).join('')}</section>`:'<div class="publish-empty">Henüz 500K+ Pump tokeni yok.</div>'}
      <details class="publish-archive"><summary><span>Geçmiş karar kayıtları</span><span>${items.length} kayıt · gerektiğinde aç</span></summary><div class="publish-archive-list">${history(items)}</div></details>
    </div>`;
    bind(root,auto);
  }

  function bind(root,auto){
    root.querySelector('#publishScanForm')?.addEventListener('submit',event=>{event.preventDefault();const mint=root.querySelector('#publishScanMint')?.value.trim();if(mint)openPackage({target:mint,symbol:'TOKEN'})});
    root.querySelectorAll('[data-pack]').forEach(button=>button.onclick=()=>openPackage(auto[Number(button.dataset.pack)]||{}));
  }

  function closeModal(){document.querySelector('.publish-modal-backdrop')?.remove()}

  async function openPackage(item){
    closeModal();
    const back=document.createElement('div');back.className='publish-modal-backdrop';
    back.innerHTML=`<section class="publish-modal" role="dialog" aria-modal="true"><header class="publish-modal-head"><div><b>${esc(symbol(item.symbol||item.name))} · güvenlik incelemesi</b><span>${esc(item.target)}</span></div><button class="publish-close" type="button">×</button></header><div class="publish-modal-tools" data-tools><button class="publish-action" type="button" disabled>Aktör kanıtı hazırlanıyor…</button></div><div class="publish-modal-body" data-body><div class="publish-loading"><div><b>Tam ARVIS çalışıyor</b>Creator, funder, Top-holder ve token çıkış imzaları birleştiriliyor.</div></div></div></section>`;
    document.body.appendChild(back);back.querySelector('.publish-close').onclick=closeModal;back.onclick=e=>{if(e.target===back)closeModal()};
    const body=back.querySelector('[data-body]'),tools=back.querySelector('[data-tools]');
    try{
      if(!window.OwnerRadarKit?.scan)throw new Error('ARVIS tarama motoru hazır değil.');
      body.innerHTML='<div id="publishingFullReport"></div>';
      const detail=await window.OwnerRadarKit.scan(item.target,'publishingFullReport');
      const actorPayload=await request(`/api/owner/actor-intelligence?target=${encodeURIComponent(item.target)}&network=solana-mainnet`);
      const actor=obj(actorPayload.actor_intelligence),gate=publishingEligibility(actor);
      if(!gate.eligible){
        tools.innerHTML=`<div class="publish-empty"><b>PAYLAŞIMA HAZIR DEĞİL</b><br>${esc(gate.reasons.join(' · ')||actorStatusLabel(actor.status))}</div>`;
        return;
      }
      tools.innerHTML='<button class="publish-action primary" data-full-copy type="button">Kanıtlı X metni</button><button class="publish-action" data-full-image type="button">Kanıt görseli</button>';
      tools.querySelector('[data-full-copy]').onclick=()=>copyText(actorPost(detail,actor,item));
      tools.querySelector('[data-full-image]').onclick=()=>downloadActorCard(detail,actor,item);
    }catch(error){body.innerHTML=`<div class="publish-empty"><b>${esc(error.message||'Tam inceleme tamamlanamadı.')}</b><br><br>Kanıt oluşmadığı için paylaşım üretilemedi.</div>`;tools.innerHTML='<button class="publish-action" type="button" disabled>PAYLAŞIMA HAZIR DEĞİL</button>'}
  }

  function actorPost(detail,actor,item){
    const coverage=obj(actor.coverage),s=symbol(obj(detail.market).symbol||item.symbol||item.name);
    const findings=arr(actor.findings).slice(0,2).join(' ');
    const base=`Koschei on-chain aktör incelemesi: $${s}. ${actorStatusLabel(actor.status)}. Creator ${short(actor.creator_wallet)}; önceki launch ${Number(actor.previous_launch_count||0)}; doğrulanmış çapraz bağ ${Number(coverage.cross_actor_links||0)}; holder kapsamı ${Number(coverage.holder_wallets_analyzed||0)}/${Number(coverage.holder_wallets_requested||0)}. ${findings}`;
    return (base+' #Solana #Koschei').slice(0,278);
  }

  function save(canvas,name){const link=document.createElement('a');link.download=name;link.href=canvas.toDataURL('image/png');link.click();note('Kanıt görseli hazır.')}
  function wrap(ctx,text,x,y,maxWidth,lineHeight,maxLines){const words=String(text||'').split(/\s+/);let row='',lines=0;for(const word of words){const next=row?row+' '+word:word;if(ctx.measureText(next).width>maxWidth&&row){ctx.fillText(row,x,y);y+=lineHeight;row=word;if(++lines>=maxLines)return y}else row=next}if(row&&lines<maxLines){ctx.fillText(row,x,y);y+=lineHeight}return y}

  function downloadActorCard(detail,actor,item){
    const gate=publishingEligibility(actor);if(!gate.eligible){note('Aktör kanıtı yeterli değil; görsel üretilmedi.');return}
    const canvas=document.createElement('canvas');canvas.width=1600;canvas.height=900;const ctx=canvas.getContext('2d');
    const gradient=ctx.createLinearGradient(0,0,1600,900);gradient.addColorStop(0,'#02090e');gradient.addColorStop(.58,'#07303a');gradient.addColorStop(1,'#02070c');ctx.fillStyle=gradient;ctx.fillRect(0,0,1600,900);ctx.strokeStyle='#26eac777';ctx.lineWidth=3;ctx.strokeRect(34,34,1532,832);
    const market=obj(detail.market),coverage=obj(actor.coverage),s=symbol(market.symbol||item.symbol||item.name);
    ctx.fillStyle='#20efbd';ctx.font='900 28px Arial';ctx.fillText('KOSCHEI · SOLANA SECURITY INTELLIGENCE',80,100);
    ctx.fillStyle='#fff';ctx.font='900 66px Arial';ctx.fillText('$'+s,80,190);ctx.fillStyle='#7e99a6';ctx.font='24px monospace';ctx.fillText(short(item.target),82,235);
    ctx.fillStyle=actor.status==='verified_linked_actor_network'?'#ff5d78':'#ffd067';ctx.font='900 48px Arial';ctx.fillText(actorStatusLabel(actor.status),80,325);
    const metrics=[['CREATOR / DEPLOYER',short(actor.creator_wallet)],['ÖNCEKİ LAUNCH',String(Number(actor.previous_launch_count||0))],['ÇAPRAZ AKTÖR BAĞI',String(Number(coverage.cross_actor_links||0))],['HOLDER DAVRANIŞ KAPSAMI',`${Number(coverage.holder_wallets_analyzed||0)}/${Number(coverage.holder_wallets_requested||0)}`]];
    metrics.forEach((metric,index)=>{const x=80+(index%2)*740,y=430+Math.floor(index/2)*125;ctx.fillStyle='#718b99';ctx.font='800 17px Arial';ctx.fillText(metric[0],x,y);ctx.fillStyle='#fff';ctx.font='900 34px Arial';ctx.fillText(metric[1],x,y+48)});
    ctx.fillStyle='#a9bec8';ctx.font='25px Arial';let y=700;for(const finding of arr(actor.findings).slice(0,2)){y=wrap(ctx,'• '+finding,80,y,1390,34,2)+12}
    ctx.fillStyle='#20efbd';ctx.font='800 18px monospace';ctx.fillText('ON-CHAIN ACTOR EVIDENCE · OWNER REVIEW · SIGNATURES AVAILABLE',80,835);
    save(canvas,`koschei-actor-evidence-${s.toLowerCase()}.png`);
  }

  async function mount(force=false){
    const page=$('page-arvis'),root=$('arvisContent');if(!page?.classList.contains('active')||!root||busy)return;if(root.querySelector('[data-publishing-shell]')&&!force)return;
    busy=true;root.innerHTML='<div class="publish-loading"><div><b>Güvenlik kuyruğu hazırlanıyor</b>500K+ Pump keşifleri yükleniyor…</div></div>';
    try{render(await request(OVERVIEW_API))}catch(error){root.innerHTML=`<div class="publish-empty"><b>Güvenlik kuyruğu açılamadı.</b><br><br>${esc(error.message)}</div>`}finally{busy=false}
  }

  let timer=0;function schedule(force=false){clearTimeout(timer);timer=setTimeout(()=>mount(force),160)}
  document.addEventListener('click',event=>{if(event.target.closest('[data-nav="arvis"]'))schedule(true)});
  document.addEventListener('keydown',event=>{if(event.key==='Escape')closeModal()});
  const observer=new MutationObserver(()=>schedule(false));observer.observe(document.documentElement,{subtree:true,attributes:true,attributeFilter:['class']});
  schedule(false);
})();