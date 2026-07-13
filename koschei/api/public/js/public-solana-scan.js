(()=>{
'use strict';
const OFFICIAL_KOSCH_MINT='HHPpU9u56Bwxov12nf7DXUCuv6h1q5j1xgGS3yukpump';
const $=id=>document.getElementById(id);
const form=$('scanForm'),submit=$('submit'),target=$('target'),kind=$('kind'),note=$('note');
const empty=$('empty'),result=$('result'),share=$('shareResult'),openExplorer=$('openExplorer');
let lastShareURL='';
const clamp=n=>Math.max(0,Math.min(100,Math.round(Number(n)||0)));
const grade=r=>r>=85?'F':r>=70?'E':r>=50?'D':r>=35?'C':r>=20?'B':'A';
const level=r=>r>=85?'critical':r>=65?'high':r>=35?'medium':'low';
const action=r=>r>=85?'UZAK DUR':r>=65?'YÜKSEK DİKKAT':r>=35?'DİKKAT':'İZLE';
const decision=r=>r>=85?'blocked':r>=65?'warn':r>=35?'review':'allow';
const unique=items=>[...new Set((items||[]).map(v=>String(v||'').trim()).filter(Boolean))];
const trFinding=value=>{
  const v=String(value||'').trim();
  const map={
    'Mint authority is active and can create additional supply.':'Mint authority açık; ek token basılabilir.',
    'Mint authority is disabled.':'Mint authority kapalı.',
    'Freeze authority is active and can freeze token accounts.':'Freeze authority açık; token hesapları dondurulabilir.',
    'Freeze authority is disabled.':'Freeze authority kapalı.',
    'The largest token account controls at least half of the supply.':'En büyük token hesabı arzın en az yarısını kontrol ediyor.',
    'The largest token account has a significant concentration.':'En büyük token hesabında belirgin arz yoğunluğu var.',
    'The ten largest token accounts control most of the supply.':'İlk 10 token hesabı arzın büyük bölümünü kontrol ediyor.'
  };
  return map[v]||v;
};
function tokenVerdict(tokenData,badgeData,mint){
  const fallbackRisk=clamp(100-clamp(tokenData.score));
  const signed=Boolean(badgeData&&badgeData.ok&&badgeData.signed&&badgeData.signature);
  const risk=signed?clamp(badgeData.risk_index):fallbackRisk;
  const evidence=unique([
    ...(tokenData.findings||[]).map(trFinding),
    signed&&badgeData.verdict?String(badgeData.verdict):'',
    signed&&badgeData.recommendation?`ARVIS aksiyonu: ${badgeData.recommendation}`:''
  ]).slice(0,3);
  if(!evidence.length)evidence.push('Canlı Solana RPC taraması tamamlandı; belirgin authority veya yoğunluk kanıtı dönmedi.');
  return {
    target:mint,risk,grade:signed?(badgeData.grade||grade(risk)):grade(risk),riskLevel:signed?(badgeData.risk_level||level(risk)):level(risk),
    action:action(risk),decision:decision(risk),evidence,
    coverage:['Canlı token supply','Mint/freeze authority','Top token-account yoğunluğu',tokenData.token_2022?'Token-2022 extension analizi':'SPL token program kontrolü',signed?'İmzalı ARVIS verdict':'ARVIS imzası alınamadı'],
    official:mint===OFFICIAL_KOSCH_MINT,
    message:signed?`ARVIS canlı Solana kanıtıyla imzalı ${badgeData.grade||grade(risk)} notu üretti. ${action(risk)}.`:'Canlı temel tarama tamamlandı; imzalı ARVIS verdict alınamadığı için sonuç ön değerlendirmedir.',
    explorer:`https://solscan.io/token/${encodeURIComponent(mint)}`,
    signed,signature:signed?String(badgeData.signature):'',ruleVersion:signed?String(badgeData.rule_version||''):''
  };
}
function preflightVerdict(data,value){
  const risk=clamp(data.score);
  return {
    target:value,risk,grade:grade(risk),riskLevel:level(risk),action:action(risk),decision:data.decision||decision(risk),
    evidence:unique(data.reasons).slice(0,3),coverage:['URL/intent heuristics','İmza ve izin dili','Koschei yapısal hafızası (varsa)'],
    official:value===OFFICIAL_KOSCH_MINT,message:data.human_message||`ARVIS ön kontrolü ${grade(risk)} notu üretti.`,explorer:'',signed:false,signature:'',ruleVersion:''
  };
}
function setList(id,items,fallback){
  const el=$(id);el.innerHTML='';
  (items&&items.length?items:[fallback]).forEach(text=>{const li=document.createElement('li');li.textContent=text;el.appendChild(li)});
}
function render(v){
  empty.hidden=true;result.hidden=false;
  $('grade').textContent=v.grade;
  $('riskScore').textContent=`${v.risk}/100`;
  $('action').textContent=v.action;
  $('action').className=`badge ${v.riskLevel}`;
  $('message').textContent=v.message;
  $('scanTarget').textContent=v.target;
  $('officialKosch').hidden=!v.official;
  setList('evidence',v.evidence,'Doğrulanmış kanıt dönmedi.');
  setList('coverage',v.coverage,'Kapsam bilgisi dönmedi.');
  const meta=$('verdictMeta');
  if(meta){
    meta.textContent=v.signed?`İmzalı · ${v.ruleVersion||'ARVIS rule'} · ${v.signature.slice(0,16)}…${v.signature.slice(-10)}`:'İmza yok · ön değerlendirme';
    meta.dataset.signed=v.signed?'true':'false';
  }
  lastShareURL=`${location.origin}/scan?mint=${encodeURIComponent(v.target)}`;
  share.hidden=false;
  openExplorer.hidden=!v.explorer;
  if(v.explorer)openExplorer.href=v.explorer;
  history.replaceState({},'',`/scan?mint=${encodeURIComponent(v.target)}`);
}
async function fetchJSON(url,options){
  const response=await fetch(url,options);
  const data=await response.json().catch(()=>({}));
  if(!response.ok)throw new Error(data.error||'scan_failed');
  return data;
}
async function runScan(){
  const value=target.value.trim();
  if(!value)return;
  submit.disabled=true;submit.textContent='ARVIS zinciri tarıyor…';
  empty.hidden=false;empty.innerHTML='<h2>Canlı Solana kanıtı toplanıyor</h2><p>Authority, supply, holder yoğunluğu ve imzalı verdict kontrol ediliyor.</p>';
  result.hidden=true;share.hidden=true;openExplorer.hidden=true;
  try{
    const tokenMode=kind.value==='token';
    if(tokenMode){
      const tokenPromise=fetchJSON('/api/token/scan',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({mint:value,network:'solana-mainnet'})});
      const badgePromise=fetchJSON(`/api/v1/risk/badge?address=${encodeURIComponent(value)}&network=solana-mainnet`).catch(()=>null);
      const [tokenData,badgeData]=await Promise.all([tokenPromise,badgePromise]);
      render(tokenVerdict(tokenData,badgeData,value));
    }else{
      const data=await fetchJSON('/api/arvis/preflight',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({target:value,kind:kind.value,intent:note.value.trim(),note:note.value.trim()})});
      render(preflightVerdict(data,value));
    }
  }catch(error){
    empty.hidden=false;empty.innerHTML='<h2>Tarama tamamlanamadı</h2><p>Bu hedef için canlı kanıt alınamadı. İşlem yapmadan önce tekrar dene.</p>';
  }finally{submit.disabled=false;submit.textContent='Ücretsiz ARVIS taraması'}
}
form.addEventListener('submit',event=>{event.preventDefault();runScan()});
share.addEventListener('click',async()=>{
  const payload={title:'Koschei ARVIS Solana güvenlik taraması',text:`ARVIS sonucu: ${$('grade').textContent} · ${$('riskScore').textContent} · ${$('action').textContent}`,url:lastShareURL};
  try{if(navigator.share){await navigator.share(payload)}else{await navigator.clipboard.writeText(lastShareURL);share.textContent='Link kopyalandı'}}catch{}
});
const params=new URLSearchParams(location.search);
const pathMint=location.pathname.startsWith('/scan/')?decodeURIComponent(location.pathname.slice(6).split('/')[0]||''):'';
const initial=pathMint||params.get('mint')||params.get('target')||'';
if(initial){target.value=initial;kind.value='token';runScan()}
})();
