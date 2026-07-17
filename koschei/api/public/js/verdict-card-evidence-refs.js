(function(root,factory){
  const api=factory(root.KoscheiVerdictCard,typeof module==='object'&&module.exports?require('./verdict-card-market-context.js'):null);
  if(typeof module==='object'&&module.exports)module.exports=api;
  if(api&&api.mapVerdictCard)root.KoscheiVerdictCard=api;
})(typeof globalThis!=='undefined'?globalThis:this,function(browserBase,nodeBase){
  'use strict';
  const base=browserBase||nodeBase;if(!base||typeof base.mapVerdictCard!=='function')return base;const rawMap=base.mapVerdictCard;
  const obj=v=>v&&typeof v==='object'&&!Array.isArray(v)?v:{};const arr=v=>Array.isArray(v)?v:[];const text=v=>String(v??'').trim();
  const unique=values=>[...new Set(values.map(text).filter(Boolean))].sort();
  const transactions=values=>{const seen=new Set(),out=[];for(const value of values){const v=obj(value),signature=text(v.signature),slot=Number(v.slot||0);const key=`${signature}:${slot}`;if((signature||slot)&&!seen.has(key)){seen.add(key);out.push({signature,slot})}}return out};
  const merge=(...refs)=>({wallets:unique(refs.flatMap(r=>arr(r?.wallets))),accounts:unique(refs.flatMap(r=>arr(r?.accounts))),transactions:transactions(refs.flatMap(r=>arr(r?.transactions))),evidence_keys:unique(refs.flatMap(r=>arr(r?.evidence_keys)))});
  const refsPresent=r=>arr(r.wallets).length+arr(r.accounts).length+arr(r.transactions).length+arr(r.evidence_keys).length>0;
  function resolvedOwner(payload){const holder=obj(payload.holder_intelligence);for(const row of arr(holder.rows)){if(row?.owner_resolved===true&&row?.risk_bearing===true&&text(row.owner_wallet))return text(row.owner_wallet)}return''}
  function firstTransactions(payload,side=''){return arr(payload.transaction_evidence).filter(row=>!side||text(row.direction).toLowerCase()===side).slice(0,8).map(row=>({signature:text(row.signature),slot:Number(row.slot||0)}))}
  function mapVerdictCard(input,options={}){
    const payload=obj(input.investigation_report||input),vm=rawMap(input,options),target=text(payload.target),source=obj(payload.source_context),lp=obj(payload.lp_control),final=obj(payload.final_verdict),actor=obj(payload.actor_investigation),holder=obj(payload.holder_intelligence),launch=obj(payload.launch_forensics),context=obj(payload.holder_concentration_context),owner=resolvedOwner(payload),creator=text(source.creator_wallet);
    const baseRef={wallets:[],accounts:unique([target]),transactions:[],evidence_keys:[]};
    const ownerRef=merge(baseRef,{wallets:[owner]});const creatorRef=merge(baseRef,{wallets:[creator]});
    const lpRef=merge(baseRef,{accounts:[lp.pool_address,lp.lp_mint,lp.token_vault,lp.quote_vault],transactions:lp.read_slot?[{slot:Number(lp.read_slot)}]:[],evidence_keys:arr(lp.evidence_keys)});
    const signedRef=merge(baseRef,{transactions:final.signature?[{signature:text(final.signature)}]:[]});
    const byId={
      launch:merge(baseRef,{transactions:launch.launch_slot?[{slot:Number(launch.launch_slot)}]:[]}),mint:baseRef,freeze:baseRef,
      wash:merge(baseRef,{transactions:firstTransactions(payload)}),address:merge(baseRef,{wallets:[creator,owner,text(actor.wallet)]}),liquidity:lpRef,
      funding:creatorRef,concentration:ownerRef,sniper:merge(baseRef,{wallets:arr(launch.profiles).map(p=>p.owner_wallet),transactions:firstTransactions(payload,'buy')}),
      'first-buyer':merge(creatorRef,{wallets:arr(launch.profiles).filter(p=>p.creator_linked).map(p=>p.owner_wallet),transactions:firstTransactions(payload,'buy')}),
      track:creatorRef,'creator-sell':merge(creatorRef,{transactions:firstTransactions(payload,'sell')}),'dominant-exit':ownerRef,'liq-move':lpRef,program:baseRef,
      distribution:merge(baseRef,{wallets:arr(launch.profiles).map(p=>p.owner_wallet),transactions:firstTransactions(payload)}),signed:signedRef
    };
    for(const row of vm.checklist){row.refs=merge(byId[row.id]||baseRef);if((row.state==='verified'||row.state==='observed')&&!refsPresent(row.refs)){row.state='arm_pending';row.status='gray';row.value=options.lang==='tr'?'Kanıt referansı bu taramada tamamlanmadı':'Evidence reference did not complete in this scan'} }
    const concentration=vm.checklist.find(row=>row.id==='concentration');if(concentration&&context.available===true){const line=options.lang==='tr'?`Owner payı %${context.top_share_pct}, taranan corpus içindeki en yoğun üst %${context.top_percentile} diliminde (n=${Number(context.sample_count||0).toLocaleString('tr-TR')})`:`Top-owner share of ${context.top_share_pct}% sits in the top ${context.top_percentile}% most concentrated tokens scanned (n=${Number(context.sample_count||0).toLocaleString('en-US')})`;concentration.value=`${concentration.value} · ${line}`;concentration.detail=concentration.detail?`${concentration.detail} · ${line}`:line}
    return vm;
  }
  return{...base,mapVerdictCard,refsPresent};
});
