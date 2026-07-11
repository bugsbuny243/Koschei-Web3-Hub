(()=>{
'use strict';

const kit=window.OwnerRadarKit;
if(!kit)return;
const originalScan=kit.scan.bind(kit);
const originalRender=kit.render.bind(kit);
const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const arr=v=>Array.isArray(v)?v:[];
const obj=v=>v&&typeof v==='object'&&!Array.isArray(v)?v:{};
const num=(v,d=4)=>new Intl.NumberFormat('tr-TR',{maximumFractionDigits:d}).format(Number(v||0));
const dt=v=>{if(!v)return'—';const d=new Date(v);return Number.isNaN(d.getTime())?'—':new Intl.DateTimeFormat('tr-TR',{dateStyle:'short',timeStyle:'short'}).format(d)};
const short=(v,n=36)=>{v=String(v||'');return v.length>n?v.slice(0,n-10)+'…'+v.slice(-7):v||'—'};
const tone=level=>['critical','high'].includes(String(level||'').toLowerCase())?'bad':String(level||'').toLowerCase()==='medium'?'warn':'ok';

async function request(path){
  const controller=new AbortController();
  const timeout=setTimeout(()=>controller.abort(),26000);
  try{
    const response=await fetch(path,{credentials:'same-origin',signal:controller.signal});
    let data={};try{data=await response.json()}catch{}
    if(!response.ok||data.ok===false)throw new Error(data.message||data.detail||data.error||`İstek başarısız (${response.status})`);
    return data;
  }catch(error){
    if(error.name==='AbortError')throw new Error('Creator davranış analizi zaman aşımına uğradı.');
    throw error;
  }finally{clearTimeout(timeout)}
}

function badge(level){return`<span class="badge ${tone(level)}">${esc(String(level||'unknown').toUpperCase())}</span>`}
function stat(title,value,detail,toneClass='tone-cyan'){return`<article class="card kpi"><div class="kpi-label">${esc(title)}</div><div class="kpi-value ${toneClass}">${esc(value)}</div><div class="kpi-foot">${esc(detail)}</div></article>`}

function creatorPanelHTML(ci){
  ci=obj(ci);
  if(ci.available===false)return`<section class="card section-gap"><div class="card-head"><div><span class="eyebrow">Creator intelligence</span><h2>Creator davranış katmanı çalışmadı</h2></div>${badge('unknown')}</div><p class="muted">${esc(ci.summary||'Creator/deployer cüzdanı doğrulanamadı.')}</p></section>`;
  const findings=arr(ci.findings),launches=arr(ci.observed_launches),recipients=arr(ci.recipient_wallets),funders=arr(ci.funding_wallets),links=arr(ci.holder_links),txs=arr(ci.transactions),holders=arr(ci.holder_accounts),limitations=arr(ci.limitations);
  const level=String(ci.risk_level||'unknown').toLowerCase();
  const riskTone=tone(level)==='bad'?'tone-red':tone(level)==='warn'?'tone-amber':'tone-green';
  return`<section class="card section-gap" data-creator-intelligence style="border-color:${tone(level)==='bad'?'#ff526f66':tone(level)==='warn'?'#ffc95c55':'#18ffb244'}">
    <div class="card-head"><div><span class="eyebrow">CREATOR / DEPLOYER BEHAVIOR FILE</span><h2>${esc(String(level).toUpperCase())} · ${num(ci.risk_index,0)}/100</h2><div class="mono muted">${esc(ci.creator_wallet||'—')}</div></div>${badge(level)}</div>
    <p class="muted">${esc(ci.summary||'Creator davranış özeti üretilemedi.')}</p>
    <div class="grid compact-grid">
      ${stat('Önceki launch',num(ci.previous_launch_count,0),'Koschei tarafından gözlemlenen diğer tokenlar',Number(ci.previous_launch_count)>=3?'tone-red':'tone-cyan')}
      ${stat('Erken sale-like',num(ci.early_sale_like_transactions,0),'Launch sonrası ilk 24 saat',Number(ci.early_sale_like_transactions)>0?'tone-red':'tone-green')}
      ${stat('Yakın sale-like',num(ci.sale_like_transactions,0),'İncelenen işlem penceresi',Number(ci.sale_like_transactions)>0?'tone-amber':'tone-green')}
      ${stat('Transfer çıkışı',num(ci.transfer_out_transactions,0),`${num(ci.current_token_outflow,8)} token çıkışı`,Number(ci.transfer_out_transactions)>=3?'tone-amber':'tone-cyan')}
      ${stat('Top-holder bağı',ci.creator_is_top_holder?`#${num(ci.creator_holder_rank,0)}`:'Yok',ci.creator_is_top_holder?`Creator payı yaklaşık ${num(ci.creator_holder_percentage)}%`:'Çözümlenen Top-20 içinde doğrudan eşleşme yok',ci.creator_is_top_holder?'tone-red':'tone-green')}
      ${stat('Dağıtım bağı',num(links.length,0),'Creator alıcısı ile Top-20 holder eşleşmesi',links.length?'tone-red':'tone-green')}
    </div>
    <div class="section-gap" style="display:flex;gap:8px;flex-wrap:wrap"><button class="btn primary" data-download-creator-poster type="button">Creator Görselini İndir</button></div>
    <canvas data-creator-poster width="1200" height="1500" style="width:min(100%,600px);display:block;margin:14px auto;border:1px solid #1de6c833;border-radius:18px;background:#02070d"></canvas>
    <details class="owner-details section-gap" open><summary><span><b>Koschei ne buldu?</b><small>Her madde ayrı kanıt sınıfıdır.</small></span><span>⌄</span></summary><div class="clean-list section-gap">${findings.map((x,i)=>`<div class="summary-row"><span>#${i+1}</span><b style="text-align:left">${esc(x)}</b>${badge(level)}</div>`).join('')||'<div class="empty">Creator davranış bulgusu yok.</div>'}</div></details>
    <details class="owner-details section-gap" open><summary><span><b>Launch geçmişi</b><small>${num(launches.length,0)} Koschei gözlemi</small></span><span>⌄</span></summary>${launches.length?`<div class="table-wrap section-gap"><table class="table"><thead><tr><th>Token</th><th>Kaynak</th><th>İlk gözlem</th><th>Olay</th><th>Durum</th></tr></thead><tbody>${launches.map(x=>`<tr><td class="mono">${esc(x.target)}</td><td>${esc(x.source||'—')}</td><td>${dt(x.observed_at)}</td><td>${num(x.event_count,0)}</td><td>${x.is_current_target?'<span class="badge ok">Mevcut token</span>':'<span class="badge warn">Önceki launch</span>'}</td></tr>`).join('')}</tbody></table></div>`:'<div class="empty section-gap">Koschei gözlemlerinde başka launch bulunmadı.</div>'}</details>
    <details class="owner-details section-gap" open><summary><span><b>Creator’dan çıkan tokenlar</b><small>${num(recipients.length,0)} alıcı cüzdan</small></span><span>⌄</span></summary>${recipients.length?`<div class="table-wrap section-gap"><table class="table"><thead><tr><th>Cüzdan</th><th>Token</th><th>İşlem</th><th>Top-holder eşleşmesi</th><th>Son gözlem</th></tr></thead><tbody>${recipients.map(x=>`<tr><td class="mono">${esc(x.wallet)}</td><td>${num(x.amount,8)}</td><td>${num(x.transactions,0)}</td><td>${x.matches_top_holder?`<span class="badge bad">#${num(x.holder_rank,0)} · ${num(x.holder_percentage)}%</span>`:'<span class="badge ok">Yok</span>'}</td><td>${dt(x.last_observed_at)}</td></tr>`).join('')}</tbody></table></div>`:'<div class="empty section-gap">İncelenen pencerede creator’dan token alan cüzdan gözlemlenmedi.</div>'}</details>
    <details class="owner-details section-gap"><summary><span><b>Funding cüzdanları</b><small>${num(funders.length,0)} olası fonlayıcı</small></span><span>⌄</span></summary>${funders.length?`<div class="table-wrap section-gap"><table class="table"><thead><tr><th>Cüzdan</th><th>SOL çıkışı</th><th>İşlem</th><th>Son gözlem</th></tr></thead><tbody>${funders.map(x=>`<tr><td class="mono">${esc(x.wallet)}</td><td>${num(x.amount,8)} SOL</td><td>${num(x.transactions,0)}</td><td>${dt(x.last_observed_at)}</td></tr>`).join('')}</tbody></table></div>`:'<div class="empty section-gap">Yakın işlem penceresinde belirgin funding bağı gözlemlenmedi.</div>'}</details>
    <details class="owner-details section-gap"><summary><span><b>İşlem kanıtları</b><small>${num(txs.length,0)} sınıflandırılmış transaction</small></span><span>⌄</span></summary>${txs.length?`<div class="table-wrap section-gap"><table class="table"><thead><tr><th>Zaman</th><th>İmza</th><th>Sınıf</th><th>Token delta</th><th>Swap</th></tr></thead><tbody>${txs.map(x=>`<tr><td>${dt(x.observed_at)}</td><td class="mono">${esc(short(x.signature,34))}</td><td>${esc(x.classification)}</td><td>${num(x.creator_token_delta,8)}</td><td>${x.swap_related?'<span class="badge warn">Evet</span>':'<span class="badge ok">Hayır</span>'}</td></tr>`).join('')}</tbody></table></div>`:'<div class="empty section-gap">Sınıflandırılabilir yakın transaction bulunmadı.</div>'}</details>
    <details class="owner-details section-gap"><summary><span><b>Top-20 owner çözümlemesi</b><small>${num(holders.length,0)} token account</small></span><span>⌄</span></summary>${holders.length?`<div class="table-wrap section-gap"><table class="table"><thead><tr><th>#</th><th>Owner wallet</th><th>Token account</th><th>Pay</th></tr></thead><tbody>${holders.map(x=>`<tr><td>${num(x.rank,0)}</td><td class="mono">${esc(x.owner_wallet||'Çözülemedi')}</td><td class="mono">${esc(short(x.token_account,32))}</td><td>${num(x.percentage)}%</td></tr>`).join('')}</tbody></table></div>`:'<div class="empty section-gap">Holder owner eşlemesi alınamadı.</div>'}</details>
    <div class="disclaimer section-gap">${esc(ci.evidence_scope||'Cüzdan ilişkileri kötü niyet veya gerçek kişi kimliği kanıtı değildir.')}</div>
    ${limitations.length?`<div class="muted small section-gap"><b>Sınırlar:</b> ${limitations.map(esc).join(' · ')}</div>`:''}
  </section>`;
}

function appendCreatorPanel(root,ci,data){
  root.querySelector('[data-creator-intelligence]')?.remove();
  root.insertAdjacentHTML('beforeend',creatorPanelHTML(ci));
  const canvas=root.querySelector('[data-creator-poster]');
  if(canvas)drawCreatorPoster(canvas,data,ci);
  root.querySelector('[data-download-creator-poster]')?.addEventListener('click',()=>downloadCreatorPoster(canvas,data,ci));
}

function wrapText(ctx,text,x,y,maxWidth,lineHeight,maxLines=5){
  const words=String(text||'').split(/\s+/);let line='',used=0;
  for(const word of words){const next=line?line+' '+word:word;if(ctx.measureText(next).width>maxWidth&&line){ctx.fillText(line,x,y);y+=lineHeight;used++;line=word;if(used>=maxLines)return y}else line=next}
  if(line&&used<maxLines){ctx.fillText(line,x,y);y+=lineHeight}return y;
}

function drawCreatorPoster(canvas,data,ci){
  if(!canvas)return;
  const c=canvas.getContext('2d'),level=String(ci.risk_level||'unknown').toUpperCase(),danger=['HIGH','CRITICAL'].includes(level),accent=danger?'#ff526f':level==='MEDIUM'?'#ffc95c':'#18ffb2';
  const g=c.createLinearGradient(0,0,1200,1500);g.addColorStop(0,'#02070d');g.addColorStop(.55,'#071824');g.addColorStop(1,'#020409');c.fillStyle=g;c.fillRect(0,0,1200,1500);c.strokeStyle='#1de6c855';c.lineWidth=3;c.strokeRect(28,28,1144,1444);
  c.fillStyle='#18ffb2';c.font='900 28px Arial';c.fillText('KOSCHEI WEB3',70,90);c.fillStyle='#fff';c.font='900 65px Arial';c.fillText('CREATOR INTELLIGENCE',70,170);c.fillStyle=accent;c.font='900 40px Arial';c.fillText(`${level} · ${Number(ci.risk_index||0)}/100`,70,235);
  c.fillStyle='#8fffe5';c.font='800 20px Arial';c.fillText('CREATOR / DEPLOYER WALLET',70,295);c.fillStyle='#fff';c.font='22px monospace';wrapText(c,ci.creator_wallet,70,335,1060,31,2);
  c.strokeStyle='#1de6c833';c.strokeRect(70,405,1060,240);const stats=[['PREVIOUS LAUNCH',ci.previous_launch_count],['EARLY SALE-LIKE',ci.early_sale_like_transactions],['SALE-LIKE',ci.sale_like_transactions],['HOLDER LINKS',arr(ci.holder_links).length]];stats.forEach((x,i)=>{const px=100+i*255;c.fillStyle='#8fa4b5';c.font='700 17px Arial';c.fillText(x[0],px,465);c.fillStyle=accent;c.font='900 48px Arial';c.fillText(String(Number(x[1]||0)),px,535)});
  c.fillStyle='#8fa4b5';c.font='700 18px Arial';c.fillText('CREATOR TOP-HOLDER MATCH',100,600);c.fillStyle=ci.creator_is_top_holder?'#ff526f':'#18ffb2';c.font='900 25px Arial';c.fillText(ci.creator_is_top_holder?`YES · RANK #${Number(ci.creator_holder_rank||0)} · ${Number(ci.creator_holder_percentage||0).toFixed(2)}%`:'NO DIRECT TOP-20 MATCH',430,600);
  c.strokeStyle='#1de6c833';c.strokeRect(70,690,1060,490);c.fillStyle='#8fffe5';c.font='800 22px Arial';c.fillText('BEHAVIOR FINDINGS',100,745);c.fillStyle='#fff';c.font='26px Arial';let y=800;for(const finding of arr(ci.findings).slice(0,6)){c.fillStyle=accent;c.fillText('•',100,y);c.fillStyle='#fff';y=wrapText(c,finding,135,y,930,36,3)+16;if(y>1110)break}
  c.fillStyle='#8fa4b5';c.font='20px Arial';wrapText(c,'Evidence-scoped wallet behavior analysis. Sale-like and wallet-link signals are not proof of fraud or real-world identity.',100,1245,990,31,3);
  c.fillStyle='#18ffb2';c.font='800 20px monospace';c.fillText('EVIDENCE FIRST · NO HYPE · NOT FINANCIAL ADVICE',70,1395);c.fillStyle='#6f8797';c.font='16px monospace';c.fillText(short(data.target,70),70,1435);
}

function downloadCreatorPoster(canvas,data,ci){
  if(!canvas)return;drawCreatorPoster(canvas,data,ci);const a=document.createElement('a');a.download=`koschei-creator-${String(data.target||'report').slice(0,10)}.png`;a.href=canvas.toDataURL('image/png');a.click();
}

async function enrich(data,root){
  const creator=obj(data.source_context).creator_wallet;
  if(!creator)return data;
  const loading=document.createElement('div');loading.className='card loading section-gap';loading.dataset.creatorLoading='1';loading.textContent='Creator/deployer geçmişi, satış davranışı ve holder bağlantıları inceleniyor…';root.appendChild(loading);
  try{
    const response=await request(`/api/owner/creator-intelligence?target=${encodeURIComponent(data.target)}&creator=${encodeURIComponent(creator)}&network=${encodeURIComponent(data.network||'solana-mainnet')}`);
    const ci=obj(response.intelligence);data.creator_intelligence=ci;
    if(ci.summary)data.narrative=[data.narrative,ci.summary].filter(Boolean).join(' ');
    originalRender(root,data);appendCreatorPanel(root,ci,data);return data;
  }catch(error){
    loading.remove();root.insertAdjacentHTML('beforeend',`<div class="error-box section-gap">Creator davranış katmanı tamamlanamadı: ${esc(error.message)}</div>`);return data;
  }
}

kit.scan=async(target,rootId)=>{
  const root=typeof rootId==='string'?document.getElementById(rootId):rootId;
  const data=await originalScan(target,rootId);
  if(!root)return data;
  return enrich(data,root);
};
kit.render=(root,data)=>{originalRender(root,data);if(data?.creator_intelligence)appendCreatorPanel(root,data.creator_intelligence,data)};
kit.drawCreatorPoster=drawCreatorPoster;
kit.downloadCreatorPoster=downloadCreatorPoster;
})();
