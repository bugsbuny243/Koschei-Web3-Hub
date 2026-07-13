(()=>{
  'use strict';
  const kit=window.OwnerRadarKit;
  if(!kit||window.__ownerLaunchForensicsInstalled)return;
  window.__ownerLaunchForensicsInstalled=true;
  const render=kit.render.bind(kit);
  const esc=value=>String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
  const arr=value=>Array.isArray(value)?value:[];
  const obj=value=>value&&typeof value==='object'&&!Array.isArray(value)?value:{};
  const short=value=>{value=String(value||'');return value.length>22?`${value.slice(0,9)}…${value.slice(-7)}`:value||'—'};
  const labels={SNIPER_BOT:'SNIPER',RHYTHM_BOT:'RİTİM BOTU',FLIPPER:'HIZLI SATIŞ',ACCUMULATOR:'BİRİKTİRİCİ',ORGANIC:'ORGANİK'};
  function profileMap(data){return new Map(arr(data?.launch_forensics?.profiles).map(row=>[String(row.owner_wallet||'').trim(),row]));}
  function updateRows(root,data){
    const detail=[...root.querySelectorAll('details.owner-details')].find(node=>node.querySelector('summary b')?.textContent?.includes('Holder Intelligence'));
    const body=detail?.querySelector('tbody');
    if(!body)return;
    const holderRows=arr(data?.holder_intelligence?.rows);
    const profiles=profileMap(data);
    [...body.querySelectorAll('tr')].forEach((tr,index)=>{
      const holder=holderRows[index]||{};
      const profile=profiles.get(String(holder.owner_wallet||'').trim());
      if(!profile||Number(profile.trade_count||0)<=0)return;
      const cells=tr.querySelectorAll('td');
      if(cells.length<7)return;
      const labelCell=cells[6],badge=labelCell.querySelector('.badge'),sub=labelCell.querySelector('.muted.small');
      const base=labels[String(profile.label||'')]||String(profile.label||'İŞLEM GEÇMİŞİ');
      if(badge){badge.textContent=profile.creator_linked?`${base} · CREATOR BAĞLI`:base;badge.className=`badge ${profile.creator_linked||profile.label==='SNIPER_BOT'||profile.label==='RHYTHM_BOT'?'bad':profile.label==='FLIPPER'?'warn':'ok'}`;}
      const firstEvidence=arr(profile.evidence)[0]||'';
      if(sub)sub.textContent=`${Number(profile.buy_count||0)} alım · ${Number(profile.sell_count||0)} satış${firstEvidence?` · ${firstEvidence}`:''}`;
      const duration=cells[5];
      if(duration){
        const timing=Number(profile.minutes_after_launch||0)>0?`Lansmandan ${Number(profile.minutes_after_launch).toFixed(1)} dk sonra`:`Giriş sırası #${Number(profile.entry_rank||0)||'—'}`;
        duration.innerHTML=`${esc(timing)}<div class="muted small">${esc(profile.first_buy_time?new Date(profile.first_buy_time).toLocaleString('tr-TR'):`slot ${profile.first_buy_slot||'—'}`)}</div>`;
      }
    });
  }
  function appendSection(root,data){
    const f=obj(data?.launch_forensics);
    if(!Object.keys(f).length||root.querySelector('[data-launch-forensics]'))return;
    const holder=[...root.querySelectorAll('details.owner-details')].find(node=>node.querySelector('summary b')?.textContent?.includes('Holder Intelligence'));
    if(!holder)return;
    const timeline=arr(f.timeline);
    const section=document.createElement('details');
    section.className='owner-details section-gap';section.open=true;section.dataset.launchForensics='1';
    const status=f.available?'Doğrulanmış lansman geçmişi':'Lansman geçmişi kısmi';
    section.innerHTML=`<summary><span><b>Launch Forensics · İlk alıcı ve aktör hikâyesi</b><small>${esc(status)} · ${esc(f.data_source||'veri yok')}</small></span><span>⌄</span></summary>
      <div class="warning-box section-gap"><b>${esc(f.summary||'Launch window not captured — token predates live monitoring. Historical tiers apply.')}</b></div>
      <div class="metadata section-gap">
        <div><label>İşlem geçmişi çözülen</label><b>${Number(f.owners_with_trade_history||0)}/${Number(f.owners_requested||0)}</b></div>
        <div><label>Sniper</label><b>${Number(f.sniper_count||0)}</b></div>
        <div><label>Ritim botu</label><b>${Number(f.rhythm_bot_count||0)}</b></div>
        <div><label>Creator bağlantılı</label><b>${Number(f.creator_linked_count||0)}</b></div>
        <div><label>ATA RPC</label><b>${Number(f.rpc_calls_used||0)}/${Number(f.rpc_budget||0)}</b></div>
        <div><label>Funding RPC</label><b>${Number(f.funding_rpc_calls_used||0)}/${Number(f.funding_rpc_budget||0)}</b></div>
      </div>
      ${timeline.length?`<div class="clean-list section-gap">${timeline.map(row=>`<div class="summary-row"><span>#${Number(row.entry_rank||0)} · ${esc(short(row.wallet))}</span><b style="text-align:left">${esc(labels[String(row.label||'')]||row.label||'İŞLEM')} · ${Number(row.minutes_after_launch||0).toFixed(1)} dk${row.creator_linked?' · creator zinciri':''}</b><span class="badge ${row.creator_linked||row.label==='SNIPER_BOT'||row.label==='RHYTHM_BOT'?'bad':'ok'}">${esc(row.slot_offset!=null?`slot +${row.slot_offset}`:'kanıt')}</span></div>`).join('')}</div>`:''}
      ${arr(f.limitations).length?`<div class="warning-box section-gap">${arr(f.limitations).map(esc).join(' · ')}</div>`:''}`;
    holder.insertAdjacentElement('afterend',section);
  }
  window.OwnerRadarKit={...kit,render(root,data){render(root,data);updateRows(root,data);appendSection(root,data);}};
})();
