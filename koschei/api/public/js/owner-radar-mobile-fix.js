(()=>{
  'use strict';
  if(window.__ownerRadarMobileFixInstalled)return;
  window.__ownerRadarMobileFixInstalled=true;

  const esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  const text=node=>String(node?.textContent||'').trim();
  const norm=value=>String(value||'').trim().toLowerCase();

  function patchVerdictMapper(){
    const api=window.KoscheiVerdictCard;
    if(!api||api.__mobileFixed||typeof api.mapVerdictCard!=='function')return;
    const base=api.mapVerdictCard;
    api.mapVerdictCard=function(payload,options){
      const vm=base(payload,options);
      if(vm?.header?.state==='signed_finding'&&!vm.header.grade)vm.header.grade='✓';
      return vm;
    };
    api.__mobileFixed=true;
  }

  function coverageChip(label,count,kind){
    return`<span class="coverage-chip ${kind}">${count} ${esc(label)}</span>`;
  }

  function compactVerdictCard(root){
    const card=root.querySelector('#verdict-card');
    if(!card||card.dataset.mobileFixed==='1')return;
    card.dataset.mobileFixed='1';
    card.classList.add('radar-case-file');
    const blocks=[...card.querySelectorAll(':scope > .vc-block')];
    const checklist=blocks[1];
    if(!checklist)return;
    const rows=[...checklist.querySelectorAll('.vc-row')];
    const counts={green:0,yellow:0,gray:0,red:0};
    rows.forEach(row=>{for(const key of Object.keys(counts))if(row.classList.contains(key)){counts[key]++;break}});
    const coverage=document.createElement('div');
    coverage.className='evidence-coverage';
    coverage.innerHTML=coverageChip('doğrulandı',counts.green,'verified')+coverageChip('izleme',counts.yellow,'watch')+coverageChip('eksik',counts.gray,'pending')+coverageChip('kritik',counts.red,'critical');
    checklist.before(coverage);
    const details=document.createElement('details');
    details.className='owner-details report-drawer verdict-checklist-drawer';
    const list=checklist.querySelector('.vc-list');
    details.innerHTML='<summary><span><b>20 teknik sinyal</b><small>Tam kanıt kontrol listesini aç</small></span><span>⌄</span></summary>';
    if(list)details.appendChild(list);
    checklist.replaceWith(details);
  }

  function fixBehavior(root){
    const sections=[...root.querySelectorAll('article.card')];
    const section=sections.find(card=>text(card.querySelector('.eyebrow')).includes('DAVRANIŞ KURALLARI'));
    if(!section)return;
    section.classList.add('radar-case-file','behavior-section');
    const grid=section.querySelector('.grid.compact-grid');
    if(!grid)return;
    grid.classList.add('behavior-rule-grid');
    [...grid.children].forEach(card=>card.classList.add('behavior-rule-card'));
  }

  const pathTranslations={
    dominant_holder_exit:{title:'Baskın holder piyasa çıkışı',limited:'Owner-resolved holder oranı ve gözlenen likiditeye göre piyasa etki kapasitesi şu an sınırlı. Bu, satış niyeti iddiası değildir.',open:'Owner-resolved holder maddi piyasa etkisi oluşturabilecek kapasiteye sahip. Bu, satış niyeti kanıtı değildir.',unknown:'Owner-resolved çıkış kapasitesi yeterli kanıtla sınıflandırılamadı.'},
    mint_inflation:{title:'Mint ile arz artırımı',closed:'Mint authority iptal edildiği için bu yol mevcut kanıtta kapalıdır.',open:'Mint authority aktif; ek arz üretme yolu teknik olarak açıktır.',unknown:'Mint authority durumu doğrulanamadı.'},
    freeze_abuse:{title:'Freeze / hesap kısıtlama',closed:'Freeze authority iptal edildiği için bu yol mevcut kanıtta kapalıdır.',open:'Freeze authority aktif; hesap kısıtlama yolu teknik olarak açıktır.',unknown:'Freeze authority durumu doğrulanamadı.'},
    liquidity_removal:{title:'Likidite çekimi',closed:'LP burn veya lock kanıtı likidite çekim yolunu mevcut kapsamda kapatıyor.',open:'LP kontrol kanıtı likidite çekim yolunun açık olduğunu gösteriyor.',observed:'İşlem destekli likidite çekimi gözlendi.',unknown:'Likidite tutarı biliniyor; fakat LP sahibi, burn/lock ve unlock koşulları doğrulanmadığı için çekim yolu bilinmiyor.'},
    coordinated_holder_exit:{title:'Koordineli holder çıkışı',observed:'Sınırlandırılmış holder geçmişinde ortak çıkış ilişkisi gözlendi.',watch:'Funding, zamanlama veya holder bağlantıları koordineli çıkış için izleme sinyali oluşturuyor.',not_observed:'İncelenen sınırlı pencerede koordineli çıkış ilişkisi gözlenmedi; bu, koordinasyonun imkânsız olduğu anlamına gelmez.',unknown:'Koordineli çıkış için yeterli holder geçmişi yok.'},
    creator_sell_acceleration:{title:'Creator satış hızlanması',observed:'Creator-resolved satış penceresinde hızlanma gözlendi.',not_observed:'İncelenen pencerede creator satış hızlanması gözlenmedi.',unknown:'Creator kimliği veya trade-ledger kapsamı eksik olduğu için satış hızlanması değerlendirilemedi.'}
  };

  function fixThreat(root){
    const section=root.querySelector('#threat-anticipation');
    if(!section)return;
    section.classList.add('radar-case-file');
    const primary=[...section.querySelectorAll('.warning-box')].find(node=>text(node).startsWith('Ana maruziyet'));
    if(primary&&text(primary).includes('No evidence-backed threat pathway could be prioritized'))primary.innerHTML='<b>Ana maruziyet</b><br>Kanıta dayalı öncelikli tehdit yolu belirlenemedi.';
    const intro=[...section.querySelectorAll(':scope > p.muted')][0];
    if(intro&&/A risk-bearing owner controls|not intent|liquidation value/i.test(text(intro)))intro.textContent='Bu bölüm owner kontrol kapasitesini gösterir; satış niyeti veya garanti edilmiş satış geliri değildir.';
    const rows=[...section.querySelectorAll('.clean-list.section-gap > .summary-row')];
    rows.forEach(row=>{
      row.classList.add('threat-path-row');
      const idNode=row.querySelector(':scope > span');
      const copy=row.querySelector(':scope > b');
      const statusNode=row.querySelector(':scope > .badge');
      const id=norm(text(idNode));
      const status=norm(text(statusNode)).replace('gözlenmedi','not_observed').replace('bilinmiyor','unknown').replace('sınırlı','limited').replace('gözlendi','observed').replace('kapalı','closed').replace('açık','open').replace('izle','watch');
      const tr=pathTranslations[id];
      if(!tr||!copy)return;
      const small=copy.querySelector('small');
      copy.childNodes[0].textContent=tr.title;
      if(small)small.textContent=tr[status]||tr.unknown||small.textContent;
    });
    const topBadge=section.querySelector('.card-head > .badge');
    if(topBadge){
      const statuses=rows.map(row=>norm(text(row.querySelector(':scope > .badge'))));
      const active=statuses.some(value=>value==='açık'||value==='gözlendi'||value==='izle');
      const unknown=statuses.some(value=>value==='bilinmiyor');
      topBadge.className=`badge ${active?'bad':unknown?'warn':'ok'}`;
      topBadge.textContent=active?'AKTİF RİSK YOLU':unknown?'KISMİ KANIT':'YOLLAR SINIRLI / KAPALI';
    }
    const rugDetails=[...section.querySelectorAll('details')].find(node=>text(node.querySelector('summary b')).includes('Rug yolu özeti'));
    if(rugDetails){
      const paragraph=rugDetails.querySelector(':scope > .section-gap > p');
      if(paragraph&&/[A-Za-z]{5}/.test(text(paragraph))){
        const badges=rows.map(row=>norm(text(row.querySelector(':scope > .badge'))));
        const count=value=>badges.filter(item=>item===value).length;
        paragraph.textContent=`Açık: ${count('açık')}. Gözlenen: ${count('gözlendi')}. Kapalı: ${count('kapalı')}. İzleme: ${count('izle')}. Bilinmeyen: ${count('bilinmiyor')}.`;
      }
    }
  }

  function fixTechnicalSections(root){
    [...root.querySelectorAll('article.card')].forEach(card=>{
      const eyebrow=text(card.querySelector('.eyebrow'));
      if(eyebrow.includes('ESKİ 14 ARVIS KOLU'))card.classList.add('radar-case-file','legacy-section');
      if(eyebrow.includes('KOSCHEI DEFENSE NETWORK'))card.classList.add('radar-case-file','actor-section');
    });
  }

  function postProcess(root){
    root=typeof root==='string'?document.getElementById(root):root;
    if(!root)return;
    compactVerdictCard(root);
    fixBehavior(root);
    fixThreat(root);
    fixTechnicalSections(root);
  }

  function wrapOwnerRadar(){
    const kit=window.OwnerRadarKit;
    if(!kit||kit.__mobileReportFixed)return;
    const baseRender=kit.renderUnified;
    if(typeof baseRender==='function'){
      kit.renderUnified=function(root,payload){const result=baseRender(root,payload);postProcess(root);return result};
    }
    const baseScan=kit.scan;
    if(typeof baseScan==='function'){
      kit.scan=async function(target,rootId){const data=await baseScan(target,rootId);postProcess(rootId);return data};
    }
    kit.__mobileReportFixed=true;
    window.OwnerRadarKit=kit;
  }

  patchVerdictMapper();
  wrapOwnerRadar();
})();
