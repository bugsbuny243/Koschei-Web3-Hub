(function(root,factory){
  const api=factory();
  if(typeof module==='object'&&module.exports)module.exports=api;
  root.KoscheiVerdictCard=api;
})(typeof globalThis!=='undefined'?globalThis:this,function(){
  'use strict';
  const LANG={en:{gatherAge:'This token is {age} old. Koschei does not guess; this report updates as verified evidence accumulates. What can already be verified is listed below.',gatherNoAge:'Koschei does not guess; this report updates as verified evidence accumulates. What can already be verified is listed below.',disc:'Koschei reports verified on-chain capability and behavior. It does not predict intent and is not investment advice.',leverage:'Owner\'s leverage',checklist:'20 signals',noData:'no data',notYet:'not yet analyzed',signed:'Signed final verdict',rows:{mint:'Owner can mint unlimited new supply at any time.',freeze:'Owner can freeze holder wallets; frozen holders cannot sell.',holder:'A single wallet controls {value}% of circulating supply. One sell can crash the price.',lp:'Liquidity is not locked. Owner can withdraw the exit pool.',repeat:'This creator cluster previously launched {value} tokens.'}},tr:{gatherAge:'Bu token {age}. Koschei tahmin üretmez; doğrulanmış kanıt oluştukça bu rapor güncellenir. Şu an doğrulanabilenler aşağıda.',gatherNoAge:'Koschei tahmin üretmez; doğrulanmış kanıt oluştukça bu rapor güncellenir. Şu an doğrulanabilenler aşağıda.',disc:'Koschei doğrulanmış zincir üstü kabiliyet ve davranışı raporlar. Niyet tahmin etmez ve yatırım tavsiyesi değildir.',leverage:'Sahibinin Kozları',checklist:'20 sinyal',noData:'veri yok',notYet:'henüz analiz edilmedi',signed:'İmzalı final verdict',rows:{mint:'Owner her an sınırsız yeni arz basabilir.',freeze:'Owner holder cüzdanlarını dondurabilir; donan holder satış yapamaz.',holder:'Tek bir cüzdan dolaşımdaki arzın %{value} kadarını kontrol ediyor. Tek satış fiyatı çökertebilir.',lp:'Likidite kilitli değil. Owner çıkış havuzunu çekebilir.',repeat:'Bu creator cluster daha önce {value} token başlattı.'}}};
  const arr=v=>Array.isArray(v)?v:[]; const obj=v=>v&&typeof v==='object'&&!Array.isArray(v)?v:{};
  const upper=v=>String(v||'').toUpperCase();
  const first=(...xs)=>xs.find(x=>x!==undefined&&x!==null&&x!=='');
  const short=v=>{const s=String(v||'');return s.length>18?`${s.slice(0,8)}…${s.slice(-6)}`:s};
  const isVerified=e=>e?.verified===true||upper(e?.evidence_status)==='VERIFIED'||upper(e?.verification_status)==='VERIFIED'||upper(e?.status)==='VERIFIED';
  const isWatch=e=>['OBSERVED','INFERRED'].includes(upper(e?.evidence_status||e?.verification_status||e?.status));
  const metrics=e=>obj(e?.metrics||e?.signals||e);
  function findEvidence(payload,keys){
    const modules=arr(payload.modules).concat(arr(obj(payload.legacy_14_arm_radar).modules));
    const all=[...arr(payload.evidence),...modules, ...arr(obj(payload.behavior_signals).signals)];
    return all.find(e=>keys.some(k=>String(e?.module_id||e?.module||e?.rule_id||e?.id||'').toLowerCase().includes(k)));
  }
  const moduleBy=(payload,id)=>findEvidence(payload,[id]);
  function ageFrom(payload){
    const source=obj(payload.source_context), sig=obj(obj(payload.final_verdict).signals);
    const raw=first(source.launch_time,source.created_at,source.launch_timestamp,source.token_created_at,sig.launch_time,payload.launch_time);
    const t=raw?new Date(raw).getTime():NaN; if(!Number.isFinite(t))return '';
    const mins=Math.max(0,Math.round((Date.now()-t)/60000)); if(mins<60)return `${mins} minutes`;
    return `${Math.round(mins/60)} hours`;
  }
  function statusFor(e,triggered){ if(!e)return'gray'; if(isVerified(e))return triggered?'red':'green'; if(isWatch(e))return'yellow'; return'gray'; }
  function mapVerdictCard(payload,options={}){
    payload=obj(payload); const lang=LANG[options.lang]||LANG.en; const final=obj(payload.final_verdict); const signals=obj(final.signals||payload.signals); const distribution=obj(payload.holder_distribution); const structural=obj(payload.structural_memory); const holder=obj(obj(payload.legacy_14_arm_radar).holder_intelligence||payload.holder_intelligence);
    const authority=moduleBy(payload,'token_authority_scanner'); const authorityM=metrics(authority); const holderEv=moduleBy(payload,'holder_concentration')||moduleBy(payload,'holder'); const holderM={...holder,...metrics(holderEv)};
    const lpEv=moduleBy(payload,'pool_guardian')||moduleBy(payload,'liquidity'); const repeatEv=moduleBy(payload,'repeat_actor')||moduleBy(payload,'actor_dossier');
    const mintActive=first(authorityM.mint_authority_present,structural.mint_authority_present,signals.mint_authority_present)===true;
    const freezeActive=first(authorityM.freeze_authority_present,structural.freeze_authority_present,signals.freeze_authority_present)===true;
    const topPct=first(holderM.top_owner_percentage,holderM.owner_resolved_top_holder_pct,holderM.top_holder_percentage,distribution.top_1_percentage,structural.largest_holder_percentage);
    const lpUnlocked=first(metrics(lpEv).lp_unlocked,metrics(lpEv).liquidity_unlocked,metrics(lpEv).withdrawable)===true;
    const repeatCount=first(metrics(repeatEv).repeat_actor_matches,metrics(repeatEv).created_token_count,metrics(repeatEv).previous_token_count);
    const leverage=[]; const push=(id,text,e,value)=>{if(isVerified(e))leverage.push({id,text,value,evidence_anchor:`#evidence-${id}`})};
    if(mintActive)push('mint-authority',lang.rows.mint,authority,mintActive); if(freezeActive)push('freeze-authority',lang.rows.freeze,authority,freezeActive);
    if(Number(topPct)>0)push('top-owner',lang.rows.holder.replace('{value}',topPct),holderEv,topPct);
    if(lpUnlocked)push('lp-unlocked',lang.rows.lp,lpEv);
    if(Number(repeatCount)>=1)push('repeat-actor',lang.rows.repeat.replace('{value}',repeatCount),repeatEv,repeatCount);
    const noGrade=String(final.verdict||final.grade||'').toLowerCase().includes('no_grade_trigger')||!final.grade||final.grade==='-';
    const grade=String(final.grade||'').match(/[A-F]/i)?.[0]?.toUpperCase()||''; const age=ageFrom(payload);
    const header=noGrade?{state:'gathering',tone:'neutral',icon:'⏳',title:'EVIDENCE GATHERING',copy:age?lang.gatherAge.replace('{age}',age):lang.gatherNoAge}:{state:'graded',tone:({A:'green',B:'green',C:'yellow',D:'orange',F:'red'}[grade]||'neutral'),grade,title:grade||'—',copy:final.verdict||''};
    header.ruleset_version=final.ruleset_version||payload.ruleset_version||'—'; header.actor_ruleset_version=final.actor_ruleset_version||payload.actor_ruleset_version||'—'; header.signature_short=short(final.signature||payload.signature); header.generated_at=final.generated_at||payload.generated_at||'';
    const specs=[['launch','Launch time/age',moduleBy(payload,'launch')||{evidence_status:first(payload.source_context?.launch_time,payload.launch_time)?'VERIFIED':'EVIDENCE_PENDING'},age||lang.noData,false],['mint','Mint authority',authority,mintActive?'active':'revoked',mintActive],['freeze','Freeze authority',authority,freezeActive?'active':'revoked',freezeActive],['wash','Wash-trading / self-transfer volume',moduleBy(payload,'sybil')||moduleBy(payload,'trade_ledger'),first(signals.self_transfer_volume,'—'),false],['address','Address behavior',moduleBy(payload,'actor_dossier'),first(metrics(moduleBy(payload,'actor_dossier')).summary,'—'),false],['liquidity','Liquidity amount + lock status',lpEv,first(metrics(lpEv).liquidity_usd,metrics(lpEv).lock_status,'—'),lpUnlocked],['funding','Creator funding origin',moduleBy(payload,'funding'),first(metrics(moduleBy(payload,'funding')).origin,'—'),false],['concentration','Holder concentration (owner-resolved)',holderEv,topPct?`${topPct}%`:lang.noData,Number(topPct)>=50],['sniper','Sniper timing',moduleBy(payload,'sniper'),first(metrics(moduleBy(payload,'sniper')).timing,'—'),false],['first-buyer','First-buyer linkage (Sybil)',moduleBy(payload,'pump')||moduleBy(payload,'sybil'),first(metrics(moduleBy(payload,'pump')).first_buyer_linkage,'—'),false],['track','Creator track record',repeatEv,repeatCount?`${repeatCount} tokens`:lang.noData,Number(repeatCount)>=1],['creator-sell','Creator sell behavior',moduleBy(payload,'urd-c003'),first(metrics(moduleBy(payload,'urd-c003')).summary,'—'),!!moduleBy(payload,'urd-c003')?.triggered],['dominant-exit','Dominant holder exit',moduleBy(payload,'urd-c004'),first(metrics(moduleBy(payload,'urd-c004')).summary,'—'),!!moduleBy(payload,'urd-c004')?.triggered],['liq-move','Liquidity movement',moduleBy(payload,'liquidity_movement'),first(metrics(moduleBy(payload,'liquidity_movement')).summary,'—'),false],['program','Program/contract relations',moduleBy(payload,'program_relation'),first(metrics(moduleBy(payload,'program_relation')).summary,'—'),false],['metadata','Metadata/impersonation check',null,lang.notYet,false],['claim','Claim/airdrop surface',moduleBy(payload,'claim'),first(metrics(moduleBy(payload,'claim')).summary,'—'),false],['mev','MEV exposure',moduleBy(payload,'mev'),first(metrics(moduleBy(payload,'mev')).summary,'—'),false],['distribution','Launch distribution fairness',moduleBy(payload,'launch_distribution'),first(metrics(moduleBy(payload,'launch_distribution')).summary,'—'),false],['signed','Signed final verdict',{evidence_status:final.signature?'VERIFIED':'EVIDENCE_PENDING'},`${header.ruleset_version} · ${header.signature_short||lang.noData}`,false]];
    return {schema_version:'koschei-verdict-card-v1',actor_ruleset_version:header.actor_ruleset_version,unified_radar_ruleset_version:header.ruleset_version,header,leverage_title:lang.leverage,leverage,checklist_title:lang.checklist,checklist:specs.map(([id,label,e,value,trig])=>({id,label,value:value==='—'?lang.noData:value,status:statusFor(e,trig),evidence_anchor:`#evidence-${id}`})),disclaimer:lang.disc};
  }
  return {mapVerdictCard,LANG};
});
