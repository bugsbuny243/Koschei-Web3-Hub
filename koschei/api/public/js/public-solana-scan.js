(()=>{
'use strict';
const OFFICIAL_KOSCH_MINT='HHPpU9u56Bwxov12nf7DXUCuv6h1q5j1xgGS3yukpump';
const $=id=>document.getElementById(id),esc=value=>String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
const form=$('scanForm'),submit=$('submit'),target=$('target'),kind=$('kind'),note=$('note'),empty=$('empty'),result=$('result'),share=$('shareResult'),openExplorer=$('openExplorer');
let lastShareURL='';
const clamp=n=>Math.max(0,Math.min(100,Math.round(Number(n)||0)));
const grade=r=>r>=85?'F':r>=70?'E':r>=50?'D':r>=35?'C':r>=20?'B':'A';
const level=r=>r>=85?'critical':r>=65?'high':r>=35?'medium':'low';
const short=value=>{const text=String(value??'');return text.length>24?`${text.slice(0,10)}…${text.slice(-8)}`:text};
function fetchJSON(url,options){return fetch(url,options).then(async response=>{const data=await response.json().catch(()=>({}));if(!response.ok)throw new Error(data.error||'scan_failed');return data})}
function stateLabel(state){return({verified:'DOĞRULANDI',observed:'GÖZLENDİ',window_open:'İZLEME PENCERESİ',not_applicable:'UYGULANAMAZ',arm_pending:'KANIT KOLU EKSİK'}[state]||String(state||'').toUpperCase())}
function refChip(type,value){const raw=String(value??'').trim();if(!raw)return'';return`<button class="evidence-ref" type="button" data-copy-ref="${esc(raw)}" title="Kopyala: ${esc(raw)}"><span>${esc(type)}</span><b>${esc(short(raw))}</b></button>`}
function renderRefs(refs={}){
  const chips=[];
  (Array.isArray(refs.wallets)?refs.wallets:[]).forEach(value=>chips.push(refChip('wallet',value)));
  (Array.isArray(refs.accounts)?refs.accounts:[]).forEach(value=>chips.push(refChip('account',value)));
  (Array.isArray(refs.signatures)?refs.signatures:[]).forEach(value=>chips.push(refChip('signature',value)));
  (Array.isArray(refs.slots)?refs.slots:[]).forEach(value=>chips.push(refChip('slot',value)));
  (Array.isArray(refs.evidence_keys)?refs.evidence_keys:[]).forEach(value=>chips.push(refChip('evidence',value)));
  return chips.length?`<div class="evidence-refs" aria-label="Kanıt referansları">${chips.join('')}</div>`:'';
}
function renderTechnicalReport(report,mint){
  if(!report||!window.KoscheiVerdictCard)return false;
  const vm=window.KoscheiVerdictCard.mapVerdictCard(report,{lang:'tr'}),h=vm.header;
  const liveHTML=window.KoscheiLiveEvidenceCard?.render(report,{lang:'tr'})||'';
  empty.hidden=true;result.hidden=false;
  result.innerHTML=`<article class="public-investigation-card"><div class="resultHead"><div class="grade">${esc(h.grade||h.icon||'✓')}</div><div><div class="risk">${esc(h.title)}</div><div class="badge medium">İMZALI TEKNİK RAPOR</div></div></div><p class="sub" style="margin-top:16px">${esc(h.copy)}</p><div class="target">${esc(report.target||mint)}</div><div class="verdictMeta" data-signed="${h.signature_short?'true':'false'}">Ruleset ${esc(h.ruleset_version)} · imza ${esc(h.signature_short||'bekliyor')} · ${esc(h.generated_at||'')}</div><div class="official" ${mint===OFFICIAL_KOSCH_MINT?'':'hidden'}><strong>Resmî KOSCH mint eşleşti.</strong><br>Bu etiket yalnız varlık kimliğini doğrular.</div><div class="section"><h3>Kanıt kapsamı</h3><p class="historySummary">${esc(vm.coverage.text)}</p></div><div class="section"><h3>${esc(vm.checklist_title)}</h3><div class="public-signal-list">${vm.checklist.map(row=>`<div class="public-signal ${esc(row.state)}" id="evidence-${esc(row.id)}"><span><b>${esc(row.label)}</b><small>${esc(stateLabel(row.state))}</small>${row.detail?`<small>${esc(row.detail)}</small>`:''}</span><em>${esc(row.value)}</em>${renderRefs(row.refs)}</div>`).join('')}</div></div><div class="section"><h3>${esc(vm.leverage_title)}</h3>${vm.leverage.length?`<ul class="list">${vm.leverage.map(row=>`<li>${esc(row.text)}</li>`).join('')}</ul>`:'<p class="historySummary">Doğrulanmış aktif kontrol satırı gözlenmedi; bu ifade risksiz anlamına gelmez.</p>'}</div><p class="fine">${esc(vm.disclaimer)}</p></article>${liveHTML}`;
  lastShareURL=`${location.origin}/scan/${encodeURIComponent(report.target||mint)}`;share.hidden=false;openExplorer.hidden=false;openExplorer.href=`https://solscan.io/token/${encodeURIComponent(mint)}`;history.replaceState({},'',`/scan/${encodeURIComponent(mint)}`);return true;
}
function renderPreflight(data,value){
  const risk=clamp(data.score),g=grade(risk);empty.hidden=true;result.hidden=false;
  result.innerHTML=`<div class="resultHead"><div class="grade">${esc(g)}</div><div><div class="risk">HIZLI ÖN KONTROL</div><div class="badge ${esc(level(risk))}">${esc(String(data.decision||'review').toUpperCase())}</div></div></div><p class="sub" style="margin-top:16px">${esc(data.human_message||'Ön kontrol tamamlandı.')}</p><div class="target">${esc(value)}</div><div class="section"><h3>Gözlenen nedenler</h3><ul class="list">${(Array.isArray(data.reasons)?data.reasons:[]).slice(0,8).map(item=>`<li>${esc(item)}</li>`).join('')||'<li>Ön kontrol ek neden satırı üretmedi.</li>'}</ul></div><p class="fine">Safe Check hızlı preflight’tır; tam token araştırması değildir.</p>`;
  lastShareURL=location.href;share.hidden=false;openExplorer.hidden=true;
}
async function runScan(){
  const value=target.value.trim();if(!value)return;
  submit.disabled=true;submit.textContent='ARVIS kanıtları topluyor…';empty.hidden=false;empty.innerHTML='<h2>Teknik araştırma çalışıyor</h2><p>Collector sonuçları, holder kontrolü, canlı işlem penceresi ve piyasa bağlamı aynı raporda birleştiriliyor.</p>';result.hidden=true;share.hidden=true;openExplorer.hidden=true;
  try{
    if(kind.value==='token'){
      const data=await fetchJSON('/api/token/scan',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({mint:value,network:'solana-mainnet'})});
      if(!renderTechnicalReport(data.investigation_report,value))throw new Error('investigation_report_missing');
    }else{
      const data=await fetchJSON('/api/arvis/preflight',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({target:value,kind:kind.value,intent:note.value.trim(),note:note.value.trim()})});renderPreflight(data,value);
    }
  }catch(error){empty.hidden=false;empty.innerHTML='<h2>Tarama tamamlanamadı</h2><p>Bu hedef için teknik rapor üretilemedi. İşlem yapmadan önce tekrar dene.</p>'}
  finally{submit.disabled=false;submit.textContent='ARVIS taramasını başlat'}
}
form.addEventListener('submit',event=>{event.preventDefault();runScan()});
result.addEventListener('click',async event=>{const button=event.target.closest('[data-copy-ref]');if(!button)return;try{await navigator.clipboard.writeText(button.dataset.copyRef||'');const previous=button.innerHTML;button.textContent='Kopyalandı';setTimeout(()=>{button.innerHTML=previous},900)}catch{}});
share.addEventListener('click',async()=>{const payload={title:'Koschei ARVIS teknik araştırma raporu',text:'Koschei ARVIS teknik araştırma sonucu',url:lastShareURL};try{if(navigator.share)await navigator.share(payload);else{await navigator.clipboard.writeText(lastShareURL);share.textContent='Link kopyalandı'}}catch{}});
const params=new URLSearchParams(location.search),pathMint=location.pathname.startsWith('/scan/')?decodeURIComponent(location.pathname.slice(6).split('/')[0]||''):'';const initial=pathMint||params.get('mint')||params.get('target')||'';if(initial){target.value=initial;kind.value='token';runScan()}
})();
