(()=>{
'use strict';
let scheduled=false;
function clean(){
  scheduled=false;
  document.querySelectorAll('.service-mini').forEach(el=>{if(/paddle/i.test(el.textContent||''))el.remove()});
  const revenue=document.getElementById('revenueContent');
  if(revenue){
    revenue.querySelectorAll('.card').forEach(el=>{if(/paddle|webhook hatası/i.test(el.textContent||''))el.remove()});
    revenue.querySelectorAll('.kpi-label').forEach(el=>{if(/aktif paket/i.test(el.textContent||''))el.textContent='Aktif paket'});
  }
  const security=document.getElementById('securityContent');
  if(security){
    security.querySelectorAll('.card.kpi').forEach(el=>{if(/paddle/i.test(el.textContent||''))el.remove()});
    security.querySelectorAll('tbody tr').forEach(el=>{if(/paddle/i.test(el.textContent||''))el.remove()});
  }
  const system=document.getElementById('systemContent');
  if(system)system.querySelectorAll('.summary-row,.service-mini').forEach(el=>{if(/paddle/i.test(el.textContent||''))el.remove()});
}
function schedule(){if(scheduled)return;scheduled=true;requestAnimationFrame(clean)}
new MutationObserver(schedule).observe(document.documentElement,{subtree:true,childList:true});
document.addEventListener('click',schedule);
schedule();
})();
