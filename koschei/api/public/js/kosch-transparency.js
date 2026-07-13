(()=>{
'use strict';
const MINT='HHPpU9u56Bwxov12nf7DXUCuv6h1q5j1xgGS3yukpump';
const $=id=>document.getElementById(id);
const clamp=n=>Math.max(0,Math.min(100,Math.round(Number(n)||0)));
const grade=r=>r>=85?'F':r>=70?'E':r>=50?'D':r>=35?'C':r>=20?'B':'A';
const riskAction=r=>r>=85?'UZAK DUR':r>=65?'YÜKSEK DİKKAT':r>=35?'DİKKAT':'İZLE';
const authority=v=>String(v||'').trim()?'AÇIK':'KAPALI';
async function fetchJSON(url,options){const response=await fetch(url,options);const data=await response.json().catch(()=>({}));if(!response.ok)throw new Error(data.error||'request_failed');return data}
function setText(id,value){const el=$(id);if(el)el.textContent=value}
function setStatus(id,value,positive){const el=$(id);if(!el)return;el.textContent=value;el.dataset.state=positive?'good':'warn'}
function renderHistory(data){const list=$('history');if(!list)return;list.innerHTML='';const items=Array.isArray(data&&data.items)?data.items:[];if(!items.length){const li=document.createElement('li');li.textContent='Henüz public imzalı verdict geçmişi yok.';list.appendChild(li);return}items.slice(0,5).forEach(item=>{const li=document.createElement('li');const when=item.created_at?new Date(item.created_at).toLocaleString('tr-TR'):'';li.textContent=`${item.grade||grade(item.risk_index)} · ${clamp(item.risk_index)}/100 · ${String(item.risk_level||'').toUpperCase()} · ${when}`;list.appendChild(li)})}
async function load(){
 setText('liveState','CANLI VERİ ALINIYOR');
 try{
  const scanPromise=fetchJSON('/api/token/scan',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({mint:MINT,network:'solana-mainnet'})});
  const badgePromise=fetchJSON(`/api/v1/risk/badge?address=${encodeURIComponent(MINT)}&network=solana-mainnet`).catch(()=>null);
  const historyPromise=fetchJSON(`/api/public/scan-history?mint=${encodeURIComponent(MINT)}&limit=8`).catch(()=>({items:[]}));
  const [scan,badge,history]=await Promise.all([scanPromise,badgePromise,historyPromise]);
  const signed=Boolean(badge&&badge.ok&&badge.signed&&badge.signature);
  const risk=signed?clamp(badge.risk_index):clamp(100-clamp(scan.score));
  setText('grade',signed?(badge.grade||grade(risk)):grade(risk));setText('risk',`${risk}/100`);setText('action',riskAction(risk));
  setText('verdictState',signed?'İMZALI ARVIS VERDICT':'CANLI ÖN DEĞERLENDİRME');
  setText('signature',signed?`${badge.signature.slice(0,18)}…${badge.signature.slice(-12)}`:'İmza henüz yok');
  setStatus('mintAuthority',authority(scan.mint_authority),!scan.mint_authority);
  setStatus('freezeAuthority',authority(scan.freeze_authority),!scan.freeze_authority);
  setText('tokenProgram',scan.token_program||'—');setText('topOne',`${Number(scan.largest_holder_percent||0).toFixed(2)}%`);setText('topTen',`${Number(scan.top_ten_percent||0).toFixed(2)}%`);
  setText('supply',scan.supply||'—');setText('decimals',String(scan.decimals??'—'));setText('extensionCount',String(Array.isArray(scan.extensions)?scan.extensions.length:0));
  const findings=$('findings');findings.innerHTML='';(scan.findings||[]).slice(0,6).forEach(value=>{const li=document.createElement('li');li.textContent=String(value);findings.appendChild(li)});if(!findings.children.length){const li=document.createElement('li');li.textContent='Canlı tarama belirgin ek bulgu döndürmedi.';findings.appendChild(li)}
  renderHistory(history);setText('updatedAt',new Date().toLocaleString('tr-TR'));setText('liveState','CANLI · SOLANA MAINNET');
 }catch(error){setText('liveState','CANLI VERİ ALINAMADI');setText('verdictState','KANIT BEKLENİYOR');}
}
$('copyMint').addEventListener('click',async()=>{try{await navigator.clipboard.writeText(MINT);$('copyMint').textContent='Mint kopyalandı'}catch{$('copyMint').textContent='Kopyalanamadı'}});
load();
})();
