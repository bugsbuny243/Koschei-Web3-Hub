(()=>{
  'use strict';
  const kit=window.OwnerRadarKit;
  if(!kit||window.__ownerHolderTierLabelsInstalled)return;
  window.__ownerHolderTierLabelsInstalled=true;
  const render=kit.render.bind(kit);
  const esc=value=>String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
  const arr=value=>Array.isArray(value)?value:[];
  const activity=new Set(['common_exit_recipient_observed','token_outflow_observed','acquisition_observed_no_outflow_in_bounded_window']);
  function apply(root,data){
    const detail=[...root.querySelectorAll('details.owner-details')].find(node=>node.querySelector('summary b')?.textContent?.includes('Holder Intelligence'));
    const body=detail?.querySelector('tbody');
    if(!body)return;
    const holderRows=arr(data?.holder_intelligence?.rows);
    const observed=new Map(arr(data?.holder_cluster?.wallets).map(row=>[String(row.wallet||'').trim(),row]));
    [...body.querySelectorAll('tr')].forEach((tr,index)=>{
      const holder=holderRows[index]||{};
      const meta=observed.get(String(holder.owner_wallet||'').trim());
      if(!meta)return;
      const cells=tr.querySelectorAll('td');
      if(cells.length<7)return;
      const tier=meta.tier==='deep'?'deep':'shallow';
      const sig=Number(meta.signatures_fetched??meta.signatures_observed??0);
      const tx=Number(meta.txs_parsed??meta.parsed_transactions??0);
      const behavior=String(holder.behavior||'');
      const hasActivity=activity.has(behavior);
      const labelCell=cells[6];
      const badge=labelCell.querySelector('.badge');
      const sub=labelCell.querySelector('.muted.small');
      if(!hasActivity){
        if(tier==='deep'){
if(badge)badge.textContent='DERİN TARAMA · HAREKET YOK';
if(sub)sub.textContent=`${sig} imza · ${tx} tx incelendi`;
        }else{
if(badge){badge.textContent='STANDART TARAMA';badge.title='Standart pencerede hedef-token hareketi gözlenmedi; bu, hareket olmadığı anlamına gelmez.';}
if(sub)sub.textContent=`Sığ pencere — derin analiz bu cüzdana uygulanmadı · ${sig} imza · ${tx} tx incelendi`;
        }
      }else if(sub){
        sub.textContent=`${sig} imza · ${tx} tx incelendi · ${tier==='deep'?'derin':'standart'} pencere`;
      }
      const duration=cells[5];
      if(duration&&duration.textContent.trim().startsWith('Bilinmiyor')){
        const last=duration.querySelector('.muted.small')?.textContent||'Son hareket zamanı gözlenmedi';
        duration.innerHTML=`${tier==='deep'?'Derin pencere tarandı':'Standart pencere tarandı'}<div class="muted small">${esc(last)}</div>`;
      }
    });
  }
  window.OwnerRadarKit={...kit,render(root,data){render(root,data);apply(root,data);}};
})();
