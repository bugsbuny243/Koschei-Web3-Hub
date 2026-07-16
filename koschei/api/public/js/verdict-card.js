(function(root,factory){
  const api=factory();
  if(typeof module==='object'&&module.exports)module.exports=api;
  root.KoscheiVerdictCard=api;
})(typeof globalThis!=='undefined'?globalThis:this,function(){
  'use strict';
  const LANG={
    en:{
      gatherAge:'This token is {age} old. Koschei does not guess; this report updates as verified evidence accumulates. What can already be verified is listed below.',
      gatherNoAge:'Koschei does not guess; this report updates as verified evidence accumulates. What can already be verified is listed below.',
      signedNoGrade:'The scan is complete and signed. No grade-changing rule was triggered. Missing evidence is not a positive safety signal.',
      signedFinding:'SIGNED FINDING',
      gathering:'EVIDENCE GATHERING',
      disc:'Koschei reports verified on-chain capability and behavior. It does not predict intent and is not investment advice.',
      leverage:'Owner control surface',checklist:'20 signals',noData:'no data',notYet:'not yet analyzed',active:'active',revoked:'revoked',tokens:'tokens',
      labels:{launch:'Launch time / age',mint:'Mint authority',freeze:'Freeze authority',wash:'Wash-trading / self-transfer volume',address:'Address behavior',liquidity:'Liquidity amount + lock status',funding:'Creator funding origin',concentration:'Holder concentration (owner-resolved)',sniper:'Sniper timing',firstBuyer:'First-buyer linkage (Sybil)',track:'Creator track record',creatorSell:'Creator sell behavior',dominantExit:'Dominant holder exit',liqMove:'Liquidity movement',program:'Program / contract relations',metadata:'Metadata / impersonation check',claim:'Claim / airdrop surface',mev:'MEV exposure',distribution:'Launch distribution fairness',signed:'Signed final verdict'},
      rows:{mint:'Owner can mint additional supply while mint authority remains active.',freeze:'Owner can freeze holder accounts while freeze authority remains active.',holder:'A single owner-resolved wallet controls {value}% of circulating supply and can materially affect market price.',lp:'Liquidity is verified as unlocked; the withdrawal path remains technically available.',repeat:'This creator cluster previously launched {value} tokens.'}
    },
    tr:{
      gatherAge:'Bu token {age}. Koschei tahmin üretmez; doğrulanmış kanıt oluştukça bu rapor güncellenir. Şu an doğrulanabilenler aşağıda.',
      gatherNoAge:'Koschei tahmin üretmez; doğrulanmış kanıt oluştukça bu rapor güncellenir. Şu an doğrulanabilenler aşağıda.',
      signedNoGrade:'Tarama tamamlandı ve imzalandı. Grade değiştiren kural tetiklenmedi. Eksik kanıt, olumlu güvenlik sinyali değildir.',
      signedFinding:'İMZALI BULGU',
      gathering:'KANIT TOPLANIYOR',
      disc:'Koschei doğrulanmış zincir üstü kabiliyet ve davranışı raporlar. Niyet tahmin etmez ve yatırım tavsiyesi değildir.',
      leverage:'Owner kontrol yüzeyi',checklist:'20 sinyal',noData:'veri yok',notYet:'henüz analiz edilmedi',active:'aktif',revoked:'iptal edildi',tokens:'token',
      labels:{launch:'Başlangıç zamanı / yaş',mint:'Mint authority',freeze:'Freeze authority',wash:'Wash-trading / self-transfer hacmi',address:'Adres davranışı',liquidity:'Likidite miktarı + kilit durumu',funding:'Creator funding kaynağı',concentration:'Holder yoğunluğu (owner-resolved)',sniper:'Sniper zamanlaması',firstBuyer:'İlk alıcı bağlantısı (Sybil)',track:'Creator geçmişi',creatorSell:'Creator satış davranışı',dominantExit:'Baskın holder çıkışı',liqMove:'Likidite hareketi',program:'Program / kontrat ilişkileri',metadata:'Metadata / taklit kontrolü',claim:'Claim / airdrop yüzeyi',mev:'MEV maruziyeti',distribution:'Launch dağılım adaleti',signed:'İmzalı final verdict'},
      rows:{mint:'Mint authority aktif kaldığı sürece owner ek arz basabilir.',freeze:'Freeze authority aktif kaldığı sürece owner holder hesaplarını dondurabilir.',holder:'Owner-resolved tek bir cüzdan dolaşımdaki arzın %{value} kadarını kontrol ediyor ve piyasa fiyatını maddi ölçüde etkileyebilir.',lp:'Likiditenin kilitsiz olduğu doğrulandı; çekim yolu teknik olarak açıktır.',repeat:'Bu creator cluster daha önce {value} token başlattı.'}
    }
  };
  const arr=v=>Array.isArray(v)?v:[];
  const obj=v=>v&&typeof v==='object'&&!Array.isArray(v)?v:{};
  const upper=v=>String(v||'').toUpperCase();
  const first=(...xs)=>xs.find(x=>x!==undefined&&x!==null&&x!=='');
  const short=v=>{const s=String(v||'');return s.length>18?`${s.slice(0,8)}…${s.slice(-6)}`:s};
  const isVerified=e=>e?.verified===true||upper(e?.evidence_status)==='VERIFIED'||upper(e?.verification_status)==='VERIFIED'||upper(e?.status)==='VERIFIED';
  const isWatch=e=>['OBSERVED','INFERRED'].includes(upper(e?.evidence_status||e?.verification_status||e?.status));
  const metrics=e=>obj(e?.metrics||e?.signals||e);
  const num=v=>{const n=Number(v);return Number.isFinite(n)?n:null};
  function findEvidence(payload,keys){
    const modules=arr(payload.modules).concat(arr(obj(payload.legacy_14_arm_radar).modules));
    const all=[...arr(payload.evidence),...modules,...arr(obj(payload.behavior_signals).signals)];
    return all.find(e=>keys.some(k=>String(e?.module_id||e?.module||e?.rule_id||e?.id||'').toLowerCase().includes(k)));
  }
  const moduleBy=(payload,id)=>findEvidence(payload,[id]);
  function ageFrom(payload,isTR){
    const source=obj(payload.source_context),sig=obj(obj(payload.final_verdict).signals);
    const raw=first(source.launch_time,source.created_at,source.launch_timestamp,source.token_created_at,sig.launch_time,payload.launch_time);
    const t=raw?new Date(raw).getTime():NaN;if(!Number.isFinite(t))return'';
    const mins=Math.max(0,Math.round((Date.now()-t)/60000));
    if(mins<60)return isTR?`${mins} dakika`:`${mins} minutes`;
    const hours=Math.round(mins/60);return isTR?`${hours} saat`:`${hours} hours`;
  }
  const knownValue=(value,lang)=>{const text=String(value??'').trim().toLowerCase();return text!==''&&text!=='—'&&text!==lang.noData.toLowerCase()&&text!==lang.notYet.toLowerCase()};
  function statusFor(e,triggered,value,lang){if(!e||!knownValue(value,lang))return'gray';if(isVerified(e))return triggered?'red':'green';if(isWatch(e))return'yellow';return'gray'}
  function mapVerdictCard(payload,options={}){
    payload=obj(payload);const langKey=options.lang==='tr'?'tr':'en';const lang=LANG[langKey];const final=obj(payload.final_verdict);const signals=obj(final.signals||payload.signals);const distribution=obj(payload.holder_distribution);const structural=obj(payload.structural_memory);const holder=obj(obj(payload.legacy_14_arm_radar).holder_intelligence||payload.holder_intelligence);
    const authority=moduleBy(payload,'token_authority_scanner');const authorityM=metrics(authority);const holderEv=moduleBy(payload,'holder_concentration')||moduleBy(payload,'holder');const holderM={...holder,...metrics(holderEv)};
    const lpEv=moduleBy(payload,'pool_guardian')||moduleBy(payload,'liquidity');const repeatEv=moduleBy(payload,'repeat_actor')||moduleBy(payload,'actor_dossier');
    const mintActive=first(authorityM.mint_authority_present,structural.mint_authority_present,signals.mint_authority_present)===true;
    const freezeActive=first(authorityM.freeze_authority_present,structural.freeze_authority_present,signals.freeze_authority_present)===true;
    const explicitOwnerResolved=holderM.owner_resolved===true||holderM.owner_resolved_holder===true||holderM.owner_resolved_top_holder===true;
    const ownerResolvedPct=first(holderM.top_owner_percentage,holderM.owner_resolved_top_holder_pct,explicitOwnerResolved?first(holderM.top_holder_percentage,holderM.largest_holder_percentage):undefined);
    const rawTopPct=first(holderM.top_holder_percentage,distribution.top_1_percentage,structural.largest_holder_percentage);
    const ownerResolvedTopNum=num(ownerResolvedPct),rawTopNum=num(rawTopPct);const concentrationIsRaw=ownerResolvedTopNum===null&&rawTopNum!==null;
    const concentrationPct=ownerResolvedTopNum!==null?ownerResolvedPct:rawTopPct;const concentrationValue=concentrationPct?`${concentrationPct}%${concentrationIsRaw?' (raw)':''}`:lang.noData;const concentrationTriggered=ownerResolvedTopNum!==null&&ownerResolvedTopNum>=50;
    const lpUnlocked=first(metrics(lpEv).lp_unlocked,metrics(lpEv).liquidity_unlocked,metrics(lpEv).withdrawable)===true;const repeatCount=first(metrics(repeatEv).repeat_actor_matches,metrics(repeatEv).created_token_count,metrics(repeatEv).previous_token_count);
    const leverage=[];const push=(id,text,e,value)=>{if(isVerified(e))leverage.push({id,text,value,evidence_anchor:`#evidence-${id}`})};
    if(mintActive)push('mint-authority',lang.rows.mint,authority,mintActive);if(freezeActive)push('freeze-authority',lang.rows.freeze,authority,freezeActive);if(ownerResolvedTopNum!==null&&ownerResolvedTopNum>=50)push('top-owner',lang.rows.holder.replace('{value}',ownerResolvedPct),holderEv,ownerResolvedPct);if(lpUnlocked)push('lp-unlocked',lang.rows.lp,lpEv);if(Number(repeatCount)>=1)push('repeat-actor',lang.rows.repeat.replace('{value}',repeatCount),repeatEv,repeatCount);
    const noGrade=String(final.verdict||final.grade||'').toLowerCase().includes('no_grade_trigger')||!final.grade||final.grade==='-';const grade=String(final.grade||'').match(/[A-F]/i)?.[0]?.toUpperCase()||'';const age=ageFrom(payload,langKey==='tr');const signed=final.signed===true||Boolean(final.signature);
    let header;
    if(noGrade&&signed)header={state:'signed_finding',tone:'neutral',icon:'✓',title:lang.signedFinding,copy:lang.signedNoGrade};
    else if(noGrade)header={state:'gathering',tone:'neutral',icon:'⏳',title:lang.gathering,copy:age?lang.gatherAge.replace('{age}',age):lang.gatherNoAge};
    else header={state:'graded',tone:({A:'green',B:'green',C:'yellow',D:'orange',F:'red'}[grade]||'neutral'),grade,title:grade||'—',copy:final.verdict||''};
    header.ruleset_version=final.ruleset_version||payload.ruleset_version||'—';header.actor_ruleset_version=final.actor_ruleset_version||payload.actor_ruleset_version||'—';header.signature_short=short(final.signature||payload.signature);header.generated_at=final.generated_at||payload.generated_at||'';
    const L=lang.labels;
    const specs=[
      ['launch',L.launch,moduleBy(payload,'launch')||{evidence_status:first(payload.source_context?.launch_time,payload.launch_time)?'VERIFIED':'EVIDENCE_PENDING'},age||lang.noData,false],
      ['mint',L.mint,authority,mintActive?lang.active:lang.revoked,mintActive],['freeze',L.freeze,authority,freezeActive?lang.active:lang.revoked,freezeActive],
      ['wash',L.wash,moduleBy(payload,'sybil')||moduleBy(payload,'trade_ledger'),first(signals.self_transfer_volume,'—'),false],['address',L.address,moduleBy(payload,'actor_dossier'),first(metrics(moduleBy(payload,'actor_dossier')).summary,'—'),false],
      ['liquidity',L.liquidity,lpEv,first(metrics(lpEv).liquidity_usd,metrics(lpEv).lock_status,'—'),lpUnlocked],['funding',L.funding,moduleBy(payload,'funding'),first(metrics(moduleBy(payload,'funding')).origin,'—'),false],
      ['concentration',L.concentration,holderEv,concentrationValue,concentrationTriggered,concentrationIsRaw?'yellow':undefined],['sniper',L.sniper,moduleBy(payload,'sniper'),first(metrics(moduleBy(payload,'sniper')).timing,'—'),false],
      ['first-buyer',L.firstBuyer,moduleBy(payload,'pump')||moduleBy(payload,'sybil'),first(metrics(moduleBy(payload,'pump')).first_buyer_linkage,'—'),false],['track',L.track,repeatEv,repeatCount?`${repeatCount} ${lang.tokens}`:lang.noData,Number(repeatCount)>=1],
      ['creator-sell',L.creatorSell,moduleBy(payload,'urd-c003'),first(metrics(moduleBy(payload,'urd-c003')).summary,'—'),!!moduleBy(payload,'urd-c003')?.triggered],['dominant-exit',L.dominantExit,moduleBy(payload,'urd-c004'),first(metrics(moduleBy(payload,'urd-c004')).summary,'—'),!!moduleBy(payload,'urd-c004')?.triggered],
      ['liq-move',L.liqMove,moduleBy(payload,'liquidity_movement'),first(metrics(moduleBy(payload,'liquidity_movement')).summary,'—'),false],['program',L.program,moduleBy(payload,'program_relation'),first(metrics(moduleBy(payload,'program_relation')).summary,'—'),false],
      ['metadata',L.metadata,null,lang.notYet,false],['claim',L.claim,moduleBy(payload,'claim'),first(metrics(moduleBy(payload,'claim')).summary,'—'),false],['mev',L.mev,moduleBy(payload,'mev'),first(metrics(moduleBy(payload,'mev')).summary,'—'),false],['distribution',L.distribution,moduleBy(payload,'launch_distribution'),first(metrics(moduleBy(payload,'launch_distribution')).summary,'—'),false],
      ['signed',L.signed,{evidence_status:final.signature?'VERIFIED':'EVIDENCE_PENDING'},`${header.ruleset_version} · ${header.signature_short||lang.noData}`,false]
    ];
    return{schema_version:'koschei-verdict-card-v2',actor_ruleset_version:header.actor_ruleset_version,unified_radar_ruleset_version:header.ruleset_version,header,leverage_title:lang.leverage,leverage,checklist_title:lang.checklist,checklist:specs.map(([id,label,e,value,trig,statusOverride])=>({id,label,value:value==='—'?lang.noData:value,status:statusOverride||statusFor(e,trig,value==='—'?lang.noData:value,lang),evidence_anchor:`#evidence-${id}`})),disclaimer:lang.disc};
  }
  return{mapVerdictCard,LANG};
});
