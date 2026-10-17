(()=>{
  'use strict';
  const kit=window.OwnerRadarKit;
  if(!kit||window.__ownerSourceEvidenceHealthInstalled)return;
  window.__ownerSourceEvidenceHealthInstalled=true;

  const obj=value=>value&&typeof value==='object'&&!Array.isArray(value)?value:{};
  const arr=value=>Array.isArray(value)?value:[];
  const rootFor=value=>typeof value==='string'?document.getElementById(value):value;
  const count=value=>typeof value==='number'&&Number.isSafeInteger(value)&&value>=0?String(value):'bilinmiyor';
  const safeLabel=value=>String(value||'unknown').replace(/[^a-zA-Z0-9_.-]/g,'').slice(0,64)||'unknown';
  function reportOf(payload){
    const envelope=obj(payload);
    return obj(envelope.investigation_report||envelope.report||envelope);
  }
  function row(parent,label,value){
    const p=document.createElement('p');
    const strong=document.createElement('strong');
    strong.textContent=label+': ';
    const span=document.createElement('span');
    span.textContent=String(value);
    p.appendChild(strong);p.appendChild(span);parent.appendChild(p);
  }
  function render(root,payload){
    root=rootFor(root);
    if(!root)return;
    const card=root.querySelector('.investigation-coverage-card')||root.querySelector('#verdict-card');
    if(!card)return;
    card.querySelector('.owner-source-evidence-health')?.remove();
    const report=reportOf(payload);
    const coverage=obj(report.investigation_coverage);
    const arms=arr(coverage.arms);
    const live=obj(report.full_scan_live_evidence);
    if(!arms.length&&!Object.keys(live).length)return;

    const section=document.createElement('details');
    section.className='owner-source-evidence-health';
    const title=document.createElement('summary');
    title.textContent='Kanıt sağlığı · tamamlanan kol, alt kontrol ve RPC kapsamı ayrı';
    section.appendChild(title);

    if(arms.length){
      row(section,'ARVIS collector',count(coverage.completed)+'/'+count(coverage.capability_total)+
        ' tamamlandı · '+count(coverage.evidence_pending)+' bekliyor · '+
        count(coverage.not_applicable)+' uygulanamaz · '+
        count(coverage.source_unavailable)+' kol kaynağı alınamadı');
      const degraded=arms.filter(arm=>arm&&arm.execution_status==='completed'&&
        (String(arm.evidence_status||'').startsWith('partial_')||
         String(arm.evidence_status||'').includes('deadline')));
      row(section,'Tamamlanan ama kısmi kaynak kanıtı taşıyan kollar',String(degraded.length)+
        (degraded.length?' — '+degraded.slice(0,14).map(arm=>safeLabel(arm.module_id)).join(', '):''));
      const unconfirmed=arms.filter(arm=>arm&&arm.execution_status==='completed'&&
        arm.subchecks&&typeof arm.subchecks==='object'&&arm.subchecks.required_checks_passed!==true);
      row(section,'Geçtiği onaylanmayan zorunlu alt kontrol grupları',String(unconfirmed.length)+
        ' tamamlanmış kol; bilinmiyor, başarısız olarak yorumlanmaz');
    }else{
      row(section,'ARVIS modül ayrıntısı','Kanonik arm durumları yok; tam kanıt sonucu çıkarılmadı');
    }

    if(Object.keys(live).length){
      row(section,'Canlı cüzdan işlemleri',count(live.wallets_completed)+'/'+
        count(live.wallets_requested)+' cüzdan · '+count(live.rpc_failures)+
        ' RPC hatası · durum: '+safeLabel(live.status));
    }else{
      row(section,'Canlı cüzdan işlemleri','Bu raporda kullanılabilir sağlık özeti bulunamadı');
    }
    row(section,'Kapsam ayrımı','Kol düzeyinde source_unavailable=0 olması, ayrı canlı işlem akışında sıfır RPC hatası olduğu anlamına gelmez.');

    const gaps=arr(obj(report.investigation_transparency).collection_gaps);
    const v1Unsupported=gaps.some(item=>{
      const text=String(obj(item).reason||'');
      return text.includes('Transaction version (1) is not supported')||text.includes('helius RPC error -32015');
    });
    if(v1Unsupported)row(section,'Helius discovery','Desteklenmeyen işlem sürümü nedeniyle kısmi kaynak; bu görünüm v1 işlem desteği açmaz veya kanıtı doğrulamaz.');

    const cluster=obj(report.holder_cluster);
    if(typeof cluster.confidence==='string'&&cluster.confidence.trim()){
      row(section,'Holder cluster confidence',safeLabel(cluster.confidence)+
        ' — yalnız holder_cluster alt sistemine ait; imzalı final grade confidence değildir.');
    }
    const final=obj(report.final_verdict);
    if(final.signed===true){
      row(section,'İmzalı verdict',safeLabel(final.grade)+
        ' — imza metadata bilgisi kanıt alt kontrollerinin tamamlandığı veya zincir doğrulaması değildir.');
    }
    card.appendChild(section);
  }
  const oldRender=kit.renderUnified;
  const oldScan=kit.scan;
  window.OwnerRadarKit={...kit,
    renderUnified(root,payload){const result=oldRender?.(root,payload);render(root,payload);return result;},
    async scan(target,root){const payload=await oldScan(target,root);render(root,payload);return payload;}
  };
})();
