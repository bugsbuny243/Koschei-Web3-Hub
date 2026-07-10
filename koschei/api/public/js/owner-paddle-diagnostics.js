(()=>{
'use strict';
let scheduled=false;
const retired=/paddle|shopier|payment|ödeme|checkout|webhook hatası|aktif paket|paket geliri|revenue/i;
function clean(){
  scheduled=false;
  document.querySelectorAll('.service-mini,.summary-row,.card,.kpi,.tab-btn,tbody tr').forEach(el=>{
    if(retired.test(el.textContent||'')) el.remove();
  });
  document.querySelectorAll('[data-tab],[data-action],[href]').forEach(el=>{
    const haystack=[el.textContent,el.getAttribute('data-tab'),el.getAttribute('data-action'),el.getAttribute('href')].join(' ');
    if(retired.test(haystack)) el.remove();
  });
}
function schedule(){if(scheduled)return;scheduled=true;requestAnimationFrame(clean)}
new MutationObserver(schedule).observe(document.documentElement,{subtree:true,childList:true});
document.addEventListener('click',schedule);
schedule();
})();
