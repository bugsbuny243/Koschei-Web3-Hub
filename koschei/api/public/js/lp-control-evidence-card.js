(()=>{
  'use strict';
  const esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  const obj=value=>value&&typeof value==='object'&&!Array.isArray(value)?value:{};
  const arr=value=>Array.isArray(value)?value:[];
  const text=value=>String(value??'').trim();
  const short=(value,length=36)=>{const raw=text(value);return raw.length>length?`${raw.slice(0,16)}…${raw.slice(-12)}`:raw||'—'};
  const number=(value,digits=8)=>Number.isFinite(Number(value))?new Intl.NumberFormat('en-US',{maximumFractionDigits:digits}).format(Number(value)):'—';
  const pct=value=>Number.isFinite(Number(value))?`${number(value,4)}%`:'—';
  const statusLabel=(value,lang)=>{
    const key=text(value).toLowerCase();
    const tr={burned:'LP YAKIMI GÖZLENDİ',locked_until:'SÜRELİ KİLİT DOĞRULANDI',permanently_locked:'KALICI KİLİT GÖZLENDİ',held_by_creator:'CREATOR LP PAYI GÖZLENDİ',unverified:'KONTROL SAHİBİ DOĞRULANMADI',observed:'GÖZLENDİ',complete_no_movement_observed:'PENCEREDE HAREKET YOK',partial_no_movement_observed:'KISMİ PENCERE',source_unavailable:'KAYNAK YOK',not_applicable:'UYGULANAMAZ'};
    const en={burned:'LP BURN OBSERVED',locked_until:'TIME LOCK VERIFIED',permanently_locked:'PERMANENT LOCK OBSERVED',held_by_creator:'CREATOR LP SHARE OBSERVED',unverified:'CONTROL OWNER UNVERIFIED',observed:'OBSERVED',complete_no_movement_observed:'NO MOVEMENT IN WINDOW',partial_no_movement_observed:'PARTIAL WINDOW',source_unavailable:'SOURCE UNAVAILABLE',not_applicable:'NOT APPLICABLE'};
    return (lang==='tr'?tr:en)[key]||key.replaceAll('_',' ').toUpperCase()||'UNKNOWN';
  };
  const copy=value=>`<button type="button" class="lp-ref" data-copy-ref="${esc(text(value))}" title="${esc(text(value))}">${esc(short(value))}</button>`;
  function movementRows(lp,lang){
    return arr(lp.liquidity_movements).map(row=>`<tr><td><b>${esc(statusLabel(row.kind,lang))}</b><small>${esc(text(row.verification_status)||'—')} · ${esc(text(row.block_time)||'—')}</small></td><td>${copy(row.actor_wallet)}<small>${esc(text(row.creator_relation)||'—')}</small></td><td>${esc(number(row.token_delta))}</td><td>${esc(number(row.quote_delta))}</td><td>${copy(row.signature)}<small>slot ${esc(row.slot||'—')}</small></td></tr>`).join('');
  }
  function render(payload,options={}){
    const lang=options.lang==='tr'?'tr':'en';
    const lp=obj(payload?.lp_control||payload?.investigation_report?.lp_control);
    if(!Object.keys(lp).length||(!text(lp.pool_address)&&text(lp.status)==='not_applicable'))return'';
    const position=text(lp.control_model)==='position_nft';
    const movements=movementRows(lp,lang);
    const title=lang==='tr'?'LİKİDİTE KONTROL KANITI':'LIQUIDITY CONTROL EVIDENCE';
    const snapshot=lang==='tr'?'POOL VE REZERV SNAPSHOT':'POOL AND RESERVE SNAPSHOT';
    const control=lang==='tr'?'KONTROL YÜZEYİ':'CONTROL SURFACE';
    const movementTitle=lang==='tr'?'ADD / REMOVE LİKİDİTE İŞLEMLERİ':'ADD / REMOVE LIQUIDITY TRANSACTIONS';
    return `<article class="card lp-control-card" id="lp-control-evidence"><div class="card-head"><div><span class="eyebrow">${title}</span><h2>${esc(text(lp.pool_type)||'Pool')}</h2><p class="muted">${esc(text(lp.control_model)||'unresolved')} · ${esc(text(lp.position_model)||'—')}</p></div><span class="badge ${['burned','locked_until','permanently_locked'].includes(text(lp.status))?'ok':'warn'}">${esc(statusLabel(lp.status,lang))}</span></div><div class="lp-address-grid"><div><label>Pool</label>${copy(lp.pool_address)}</div><div><label>Program</label>${copy(lp.pool_program)}</div><div><label>Read slot</label><b>${esc(lp.read_slot||'—')}</b></div><div><label>Canonical</label><b>${lp.canonical_pool?'YES':'—'}</b></div></div><h3>${snapshot}</h3><div class="lp-metrics"><div><label>Token vault</label>${copy(lp.token_vault)}<b>${esc(number(lp.token_reserve))}</b></div><div><label>Quote vault</label>${copy(lp.quote_vault)}<b>${esc(number(lp.quote_reserve))}</b></div>${Number(lp.virtual_quote_reserve)>0?`<div><label>Virtual quote reserve</label><b>${esc(number(lp.virtual_quote_reserve))}</b></div>`:''}<div><label>Reserve value</label><b>${Number(lp.reserve_liquidity_usd)>0?`$${esc(number(lp.reserve_liquidity_usd,2))}`:'—'}</b></div></div><h3>${control}</h3>${position?`<div class="lp-metrics"><div><label>Pool liquidity raw</label><b>${esc(text(lp.pool_liquidity_raw)||'—')}</b></div><div><label>Permanent lock raw</label><b>${esc(text(lp.permanent_locked_liquidity_raw)||'—')}</b></div><div><label>Permanent locked share</label><b>${esc(pct(lp.permanent_locked_share_pct))}</b></div><div><label>Ownership model</label><b>POSITION NFT</b></div></div>`:`<div class="lp-metrics"><div><label>LP mint</label>${copy(lp.lp_mint)}<b>${esc(number(lp.lp_supply))}</b></div><div><label>Burn share</label><b>${esc(pct(lp.burned_share_pct))}</b></div><div><label>Creator LP share</label><b>${esc(pct(lp.creator_lp_share_pct))}<small>${esc(text(lp.creator_relation)||'—')}</small></b></div><div><label>Dominant LP owner</label>${copy(lp.dominant_lp_owner)}<b>${esc(pct(lp.dominant_lp_share_pct))}<small>${esc(text(lp.dominant_lp_classification)||'—')}</small></b></div><div><label>Locker / unlock</label><b>${esc(short(lp.locker_account))}<small>${esc(text(lp.locked_until)||'—')}</small></b></div></div>`}<div class="lp-movement-head"><h3>${movementTitle}</h3><span class="badge ${movements?'ok':'warn'}">${movements?`${arr(lp.liquidity_movements).length} OBSERVED`:statusLabel(lp.movement_status,lang)}</span></div>${movements?`<div class="lp-table-wrap"><table><thead><tr><th>Type</th><th>Actor wallet</th><th>Token Δ</th><th>Quote Δ</th><th>Signature / slot</th></tr></thead><tbody>${movements}</tbody></table></div>`:`<p class="lp-window-line">${esc(lp.movement_window_parsed||0)} parsed · ${esc(lp.movement_window_signatures||0)} signatures · ${esc(lp.movement_window_failures||0)} failures</p>`}</article>`;
  }
  window.KoscheiLPControlCard={render};
})();
