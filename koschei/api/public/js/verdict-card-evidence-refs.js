(function(root,factory){
  const api=factory(root.KoscheiVerdictCard,typeof module==='object'&&module.exports?require('./verdict-card-market-context.js'):null);
  if(typeof module==='object'&&module.exports)module.exports=api;
  if(api&&api.mapVerdictCard)root.KoscheiVerdictCard=api;
})(typeof globalThis!=='undefined'?globalThis:this,function(browserBase,nodeBase){
  'use strict';
  const base=browserBase||nodeBase;
  if(!base||typeof base.mapVerdictCard!=='function')return base;
  const rawMap=base.mapVerdictCard;
  const obj=value=>value&&typeof value==='object'&&!Array.isArray(value)?value:{};
  const arr=value=>Array.isArray(value)?value:[];
  const text=value=>String(value??'').trim();
  const unique=values=>[...new Set(values.map(text).filter(Boolean))].sort();
  const positiveSlots=values=>[...new Set(values.map(Number).filter(value=>Number.isSafeInteger(value)&&value>0))].sort((a,b)=>a-b);
  const number=value=>{const parsed=Number(value);return Number.isFinite(parsed)?parsed:null};
  function normalize(value){
    value=obj(value);
    return{
      wallets:unique(arr(value.wallets)),
      accounts:unique(arr(value.accounts)),
      signatures:unique(arr(value.signatures)),
      slots:positiveSlots(arr(value.slots)),
      evidence_keys:unique(arr(value.evidence_keys))
    };
  }
  function refsPresent(value){
    value=normalize(value);
    return value.wallets.length+value.accounts.length+value.signatures.length+value.slots.length+value.evidence_keys.length>0;
  }
  function mapVerdictCard(input,options={}){
    const payload=obj(input.investigation_report||input);
    const vm=rawMap(input,options);
    const referenceMap=obj(payload.evidence_references);
    for(const row of arr(vm?.checklist)){
      row.refs=normalize(referenceMap[row.id]);
      if((row.state==='verified'||row.state==='observed')&&!refsPresent(row.refs)){
        row.state='arm_pending';
        row.status='gray';
        row.value=options.lang==='tr'?'Kanıt referansı bu taramada tamamlanmadı':'Evidence reference did not complete in this scan';
      }
    }
    const corpus=obj(payload.holder_concentration_context),concentration=arr(vm?.checklist).find(row=>row.id==='concentration');
    if(concentration&&corpus.available===true&&number(corpus.top_percentile)!==null&&number(corpus.sample_count)!==null){
      const locale=options.lang==='tr'?'tr-TR':'en-US';
      const share=new Intl.NumberFormat(locale,{maximumFractionDigits:4}).format(Number(corpus.top_share_pct));
      const percentile=new Intl.NumberFormat(locale,{maximumFractionDigits:2}).format(Number(corpus.top_percentile));
      const sample=new Intl.NumberFormat(locale,{maximumFractionDigits:0}).format(Number(corpus.sample_count));
      const line=options.lang==='tr'?`Owner payı %${share}; taranan farklı mint corpus’unda en yoğun üst %${percentile} diliminde (n=${sample})`:`Owner share ${share}%; top ${percentile}% most concentrated among distinct scanned mints (n=${sample})`;
      concentration.value=`${concentration.value} · ${line}`;
      concentration.detail=concentration.detail?`${concentration.detail} · ${line}`:line;
      concentration.corpus_context=corpus;
    }
    const coverage={verified:0,observed:0,window_open:0,not_applicable:0,arm_pending:0};
    for(const row of arr(vm?.checklist))coverage[row.state]=(coverage[row.state]||0)+1;
    const labels=options.lang==='tr'?{verified:'doğrulanmış',observed:'gözlenen',window_open:'izleme penceresinde',not_applicable:'uygulanamaz',arm_pending:'bekleyen'}:{verified:'verified',observed:'observed',window_open:'in monitoring window',not_applicable:'not applicable',arm_pending:'pending'};
    coverage.text=`${coverage.verified} ${labels.verified} · ${coverage.observed} ${labels.observed} · ${coverage.window_open} ${labels.window_open} · ${coverage.not_applicable} ${labels.not_applicable} · ${coverage.arm_pending} ${labels.arm_pending}`;
    vm.coverage=coverage;
    return vm;
  }
  return{...base,mapVerdictCard,refsPresent,normalizeEvidenceRefs:normalize};
});
