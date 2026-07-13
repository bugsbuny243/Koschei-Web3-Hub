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
function tokenVerdict(data,mint){
  const safety=clamp(data.score);
  const risk=clamp(100-safety);
  const evidence=unique((data.findings||[]).map(trFinding)).slice(0,3);
  if(!evidence.length)evidence.push('Canlı Solana RPC taraması tamamlandı; belirgin authority veya yoğunluk kanıtı dönmedi.');
  const coverage=[
    'Canlı token supply',
    'Mint/freeze authority',
    'Top token-account yoğunluğu',
    data.token_2022?'Token-2022 extension analizi':'SPL token program kontrolü'
  ];
  return {
    target:mint,risk,grade:grade(risk),riskLevel:level(risk),action:action(risk),decision:decision(risk),
    evidence,coverage,official:mint===OFFICIAL_KOSCH_MINT,
    message:`ARVIS canlı Solana kanıtıyla ${grade(risk)} notu üretti. ${action(risk)}.`,
    explorer:`https://solscan.io/token/${encodeURIComponent(mint)}`
  };
}
function preflightVerdict(data,value){
  const risk=clamp(data.score);
  return {
    target:value,risk,grade:grade(risk),riskLevel:level(risk),action:action(risk),decision:data.decision||decision(risk),
    evidence:unique(data.reasons).slice(0,3),coverage:['URL/intent heuristics','İmza ve izin dili','Koschei yapısal hafızası (varsa)'],
    official:value===OFFICIAL_KOSCH_MINT,message:data.human_message||`ARVIS ön kontrolü ${grade(risk)} notu üretti.`,explorer:''
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
  lastShareURL=`${location.origin}/scan?mint=${encodeURIComponent(v.target)}`;
  share.hidden=false;
  openExplorer.hidden=!v.explorer;
  if(v.explorer)openExplorer.href=v.explorer;
  history.replaceState({},'',`/scan?mint=${encodeURIComponent(v.target)}`);
}
async function runScan(){
  const value=target.value.trim();
  if(!value)return;
  submit.disabled=true;submit.textContent='ARVIS zinciri tarıyor…';
  empty.hidden=false;empty.innerHTML='<h2>Canlı Solana kanıtı toplanıyor</h2><p>Authority, supply ve holder yoğunluğu kontrol ediliyor.</p>';
  result.hidden=true;share.hidden=true;openExplorer.hidden=true;
  try{
    const tokenMode=kind.value==='token';
    const response=await fetch(tokenMode?'/api/token/scan':'/api/arvis/preflight',{
      method:'POST',headers:{'Content-Type':'application/json'},
      body:JSON.stringify(tokenMode?{mint:value,network:'solana-mainnet'}:{target:value,kind:kind.value,intent:note.value.trim(),note:note.value.trim()})
    });
    const data=await response.json().catch(()=>({}));
    if(!response.ok)throw new Error(data.error||'scan_failed');
    render(tokenMode?tokenVerdict(data,value):preflightVerdict(data,value));
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
