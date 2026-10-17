(()=>{
  'use strict';
  const kit=window.OwnerRadarKit;
  if(!kit||window.__ownerVerdictCoverageInstalled)return;
  window.__ownerVerdictCoverageInstalled=true;
  const rootFor=value=>typeof value==='string'?document.getElementById(value):value;
  const esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  const short=value=>{const text=String(value??'');return text.length>24?`${text.slice(0,10)}…${text.slice(-8)}`:text};
  const chip=(type,value)=>{const raw=String(value??'').trim();return raw?`<button type="button" class="vc-evidence-ref" data-copy-ref="${esc(raw)}" title="Kopyala: ${esc(raw)}"><span>${esc(type)}</span><b>${esc(short(raw))}</b></button>`:''};
  function refsHTML(refs={}){
    const items=[];
    (Array.isArray(refs.wallets)?refs.wallets:[]).forEach(value=>items.push(chip('wallet',value)));
    (Array.isArray(refs.accounts)?refs.accounts:[]).forEach(value=>items.push(chip('account',value)));
    (Array.isArray(refs.signatures)?refs.signatures:[]).forEach(value=>items.push(chip('signature',value)));
    (Array.isArray(refs.slots)?refs.slots:[]).forEach(value=>items.push(chip('slot',value)));
    (Array.isArray(refs.evidence_keys)?refs.evidence_keys:[]).forEach(value=>items.push(chip('evidence',value)));
    return items.length?`<div class="vc-evidence-refs" aria-label="Kanıt referansları">${items.join('')}</div>`:'';
  }
  const integer=value=>typeof value==='number'&&Number.isSafeInteger(value)&&value>=0?value:null;
  function runtimeArmCoverage(payload){
    const input=payload&&typeof payload==='object'?payload:{};
    const report=input.investigation_report||input.report||input;
    const coverage=report.investigation_coverage||report.legacy_14_arm_radar?.investigation_coverage;
    if(!coverage||typeof coverage!=='object')return 'ARVIS çalışma kapsamı: bu yanıtta kanonik sayaç yok; kontrol listesi 14 collectorın sonucu değildir.';
    const fields=['capability_total','completed','evidence_pending','not_applicable','source_unavailable','insufficient_evidence'];
    const counts=fields.map(key=>integer(coverage[key]));
    if(counts.some(value=>value===null)||counts[0]===0||
      counts[1]+counts[2]+counts[3]+counts[4]+counts[5]!==counts[0]){
      return 'ARVIS çalışma kapsamı: kanonik sayaçlar tutarsız veya eksik; tamamlandı sonucu üretilmedi.';
    }
    const [total,completed,pending,na,unavailable,insufficient]=counts;
    const gap=Array.isArray(coverage.critical_gaps)?coverage.critical_gaps.length:0;
    const terminal=(coverage.status==='complete_investigation'?'tam':'kısmi/eksik');
    return 'ARVIS collector kapsamı: '+completed+'/'+total+' tamamlandı · '+pending+
      ' kanıt bekliyor · '+na+' uygulanamaz · '+unavailable+' kaynak hatası · '+
      insufficient+' yetersiz kanıt · '+gap+' kritik boşluk · durum: '+terminal+
      '. İmza ve grade, collector kapsamının tamamlandığını kanıtlamaz.';
  }
  function installStyles(){
    if(document.getElementById('ownerVerdictEvidenceRefStyles'))return;
    const style=document.createElement('style');style.id='ownerVerdictEvidenceRefStyles';
    style.textContent='.vc-evidence-refs{display:flex;flex-wrap:wrap;gap:6px;margin:-3px 0 10px 28px}.vc-evidence-ref{display:inline-flex;align-items:center;gap:6px;min-height:28px;padding:5px 8px;border:1px solid #ffffff1f;border-radius:9px;background:#ffffff08;color:inherit;font:700 10px var(--owner-mono,monospace);cursor:pointer;max-width:100%}.vc-evidence-ref span{color:#7ea0b4;text-transform:uppercase;font-size:8px}.vc-evidence-ref b{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.vc-evidence-ref:active{transform:translateY(1px)}@media(max-width:640px){.vc-evidence-refs{margin-left:0}.vc-evidence-ref{max-width:100%}}';
    document.head.appendChild(style);
  }
  function installCopy(root){
    if(root.dataset.evidenceRefCopyInstalled==='true')return;
    root.dataset.evidenceRefCopyInstalled='true';
    root.addEventListener('click',async event=>{
      const button=event.target.closest('[data-copy-ref]');if(!button||!root.contains(button))return;
      try{await navigator.clipboard.writeText(button.dataset.copyRef||'');const previous=button.innerHTML;button.textContent='Kopyalandı';setTimeout(()=>{button.innerHTML=previous},900)}catch{}
    });
  }
  function attach(root,payload){
    root=rootFor(root);if(!root||!window.KoscheiVerdictCard)return;
    const card=root.querySelector('#verdict-card'),host=card?.querySelector('.vc-header>div:last-child');
    if(!host)return;
    installStyles();installCopy(root);
    card.querySelectorAll('.vc-evidence-refs').forEach(node=>node.remove());
    host.querySelector('.vc-coverage')?.remove();
    host.querySelector('.vc-arm-runtime')?.remove();
    const vm=window.KoscheiVerdictCard.mapVerdictCard(payload,{lang:'tr'});
    const runtime=document.createElement('div');
    runtime.className='vc-arm-runtime';
    runtime.setAttribute('aria-label','ARVIS kanonik collector kapsamı');
    runtime.textContent=runtimeArmCoverage(payload);
    host.appendChild(runtime);
    if(vm?.coverage?.text)host.insertAdjacentHTML('beforeend',`<div class="vc-coverage">Kontrol listesi (14 ARVIS kolundan ayrı): ${esc(vm.coverage.text)}</div>`);
    for(const row of Array.isArray(vm?.checklist)?vm.checklist:[]){
      const rowElement=card.querySelector(`#evidence-${CSS.escape(String(row.id||''))}`);if(!rowElement)continue;
      const refs=refsHTML(row.refs);if(refs)rowElement.insertAdjacentHTML('afterend',refs);
    }
  }
  const baseRender=kit.renderUnified;
  const renderUnified=(root,payload)=>{const result=baseRender?.(root,payload);attach(root,payload);return result};
  const baseScan=kit.scan;
  const scan=async(target,rootId)=>{const payload=await baseScan(target,rootId);attach(rootId,payload);return payload};
  window.OwnerRadarKit={...kit,renderUnified,scan};
})();
