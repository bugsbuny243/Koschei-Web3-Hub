(()=>{
'use strict';
const $=id=>document.getElementById(id);
const form=$('txForm'),submit=$('submitTx'),empty=$('empty'),result=$('result');
const labelAction=value=>({allow:'DEVAM EDEBİLİRSİN',warn:'İMZALAMADAN İNCELE',block:'İMZALAMA',withhold:'KARAR BEKLETİLDİ'}[String(value||'').toLowerCase()]||'İNCELE');
const classFor=value=>{value=String(value||'unknown').toLowerCase();return value==='low'?'low':value==='medium'?'medium':value==='high'?'high':'critical'};
function renderList(id,items,formatter,fallback){const el=$(id);el.innerHTML='';const list=Array.isArray(items)&&items.length?items:[fallback];list.forEach(item=>{const li=document.createElement('li');li.textContent=formatter?formatter(item):String(item||'');el.appendChild(li)})}
async function fetchJSON(url,options){const response=await fetch(url,options);const data=await response.json().catch(()=>({}));if(!response.ok){const error=new Error(data.message||data.code||'simulation_failed');error.data=data;throw error}return data}
form.addEventListener('submit',async event=>{
 event.preventDefault();const transaction=$('transaction').value.trim();if(!transaction)return;
 submit.disabled=true;submit.textContent='Solana simülasyonu çalışıyor…';result.hidden=true;empty.hidden=false;empty.innerHTML='<h2>İşlem zincir üzerinde simüle ediliyor</h2><p>İşlem gönderilmiyor ve imzalanmıyor.</p>';
 try{
  const data=await fetchJSON('/api/public/transaction-simulate',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({transaction,encoding:'base64',network:'solana-mainnet',wallet:$('wallet').value.trim()})});
  empty.hidden=true;result.hidden=false;
  $('action').textContent=labelAction(data.action);$('action').className='action '+classFor(data.risk_level);
  $('risk').textContent=`${Number(data.risk_index||0)}/100`;$('summary').textContent=data.summary||'Simülasyon tamamlandı.';
  $('fingerprint').textContent=data.transaction_fingerprint||'—';$('units').textContent=Number(data.simulation&&data.simulation.units_consumed||0).toLocaleString('tr-TR');
  $('programCount').textContent=Array.isArray(data.program_ids)?data.program_ids.length:0;
  renderList('findings',data.findings,item=>`${String(item.severity||'').toUpperCase()} · ${item.title||item.code}: ${item.evidence||''}`,'Yüksek güvenli tehlikeli instruction sinyali bulunmadı.');
  renderList('programs',data.program_ids,null,'Çağrılan program kimliği loglardan çıkarılamadı.');
  $('warning').textContent=data.warning||'Read-only shadow mode.';
 }catch(error){
  empty.hidden=false;empty.innerHTML=`<h2>Simülasyon tamamlanamadı</h2><p>${String(error.message||'Solana RPC simülasyonu başarısız oldu.')}</p>`;
 }finally{submit.disabled=false;submit.textContent='İşlemi simüle et'}
});
})();
