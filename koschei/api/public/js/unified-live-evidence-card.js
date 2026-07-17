(function(root,factory){
  const api=factory();
  if(typeof module==='object'&&module.exports)module.exports=api;
  root.KoscheiLiveEvidenceCard=api;
})(typeof globalThis!=='undefined'?globalThis:this,function(){
  'use strict';
  const esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  const arr=value=>Array.isArray(value)?value:[];
  const obj=value=>value&&typeof value==='object'&&!Array.isArray(value)?value:{};
  const short=(value,length=32)=>{const text=String(value||'');return text.length>length?`${text.slice(0,length-10)}…${text.slice(-7)}`:text||'—'};
  const num=value=>new Intl.NumberFormat('tr-TR',{maximumFractionDigits:8}).format(Number(value||0));
  const role=value=>({creator_source_observed:'Creator kaynağı',launch_signer_observed:'Launch imzacısı',risk_bearing_holder:'Owner-resolved holder'}[String(value||'')]||String(value||'cüzdan').replaceAll('_',' '));
  const direction=value=>({buy:'ALIM / SWAP GİRİŞİ',sell:'SATIŞ / SWAP ÇIKIŞI',transfer_in:'TOKEN GİRİŞİ',transfer_out:'TOKEN ÇIKIŞI'}[String(value||'')]||String(value||'').replaceAll('_',' ').toUpperCase());
  const tone=value=>({buy:'ok',transfer_in:'ok',sell:'bad',transfer_out:'warn'}[String(value||'')]||'warn');
  const txLink=signature=>signature?`<a class="mono live-evidence-link" href="https://solscan.io/tx/${encodeURIComponent(signature)}" target="_blank" rel="noopener noreferrer">${esc(short(signature,34))}</a>`:'—';
  function render(payload,options={}){
    const report=obj(payload?.full_scan_live_evidence||payload),rows=arr(report.transactions),wallets=arr(report.wallet_coverage),launch=obj(report.launch_signer);
    if(!Object.keys(report).length||report.status==='not_requested')return'';
    const status=String(report.status||'unknown');
    const statusLabel={complete:'CANLI PENCERE TAMAMLANDI',partial:'KISMİ CANLI PENCERE',partial_timeout:'SÜRE SINIRINA ULAŞTI',collection_failed:'TOPLAMA BAŞARISIZ',source_unavailable:'RPC YOK',no_resolved_wallet_targets:'CÜZDAN HEDEFİ YOK'}[status]||status.replaceAll('_',' ').toUpperCase();
    const launchLine=launch.available?`<div class="live-launch-relation"><b>Launch işlem imzacısı gözlendi</b><span class="mono">${esc(short(launch.wallet,40))}</span>${txLink(launch.signature)}<small>Bu yalnız zincir üstü imza ilişkisidir; creator veya gerçek kişi kimliği iddiası değildir.</small></div>`:'';
    const table=rows.length?`<div class="live-evidence-table-wrap"><table class="live-evidence-table"><thead><tr><th>Rol / cüzdan</th><th>Hareket</th><th>Token değişimi</th><th>Karşı cüzdanlar</th><th>Kanıt</th></tr></thead><tbody>${rows.map(row=>`<tr><td><b>${esc(role(row.role))}</b><div class="mono">${esc(short(row.wallet,34))}</div></td><td><span class="live-direction ${esc(tone(row.direction))}">${esc(direction(row.direction))}</span><small>${row.swap_related?'Swap izi doğrulandı':'Doğrudan token bakiyesi değişimi'}</small></td><td class="mono">${esc(num(row.token_delta))}</td><td>${arr(row.counterparties).map(wallet=>`<span class="mono live-counterparty">${esc(short(wallet,28))}</span>`).join('')||'—'}</td><td>${txLink(row.signature)}<small>slot ${esc(row.slot||'—')} · ${esc(row.block_time||'—')}</small></td></tr>`).join('')}</tbody></table></div>`:`<div class="live-evidence-empty"><b>İlgili token bakiye hareketi bulunmadı.</b><span>${esc(report.transactions_parsed||0)} işlem ayrıştırıldı; ${esc(report.wallets_completed||0)}/${esc(report.wallets_requested||0)} cüzdan penceresi tamamlandı.</span><small>Bu sonuç yalnız bounded son işlem penceresini kapsar; eski hareketlerin yokluğu anlamına gelmez.</small></div>`;
    const failures=wallets.filter(item=>!String(item.status||'').startsWith('complete'));
    const failureLine=failures.length?`<details class="live-evidence-gaps"><summary>Eksik cüzdan pencereleri (${failures.length})</summary>${failures.map(item=>`<div><span class="mono">${esc(short(item.wallet,30))}</span><b>${esc(String(item.status||'').replaceAll('_',' ').toUpperCase())}</b></div>`).join('')}</details>`:'';
    return`<article class="card live-evidence-card" id="full-scan-live-evidence"><div class="card-head"><div><span class="eyebrow">CANLI İŞLEM KANITI · BOUNDED RPC WINDOW</span><h2>${rows.length?`${rows.length} doğrulanabilir işlem satırı`:'Canlı işlem penceresi tamamlandı'}</h2><p class="muted">Açıklama yerine gerçek signature, slot, owner-resolved cüzdan ve token bakiyesi değişimi.</p></div><span class="badge ${rows.length?'ok':'warn'}">${esc(statusLabel)}</span></div><div class="live-evidence-metrics"><div><label>Cüzdan</label><b>${esc(report.wallets_completed||0)}/${esc(report.wallets_requested||0)}</b></div><div><label>İmza görüldü</label><b>${esc(report.signatures_seen||0)}</b></div><div><label>İşlem ayrıştırıldı</label><b>${esc(report.transactions_parsed||0)}</b></div><div><label>Mint hareketi</label><b>${esc(rows.length)}</b></div><div><label>RPC hata</label><b>${esc(report.rpc_failures||0)}</b></div></div>${launchLine}${table}${failureLine}</article>`;
  }
  return{render};
});
