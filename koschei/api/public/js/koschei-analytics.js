var KoscheiAnalytics=(function(){
'use strict';
function authJwt(){try{return window.KoscheiAuth?.getJwt?.()||''}catch{return''}}
function authEmail(){try{return window.KoscheiAuth?.getEmail?.()||null}catch{return null}}
function track(eventName,metadata={}){try{const headers={'Content-Type':'application/json'},jwt=authJwt();if(jwt)headers.Authorization='Bearer '+jwt;fetch('/api/analytics/event',{method:'POST',headers,keepalive:true,body:JSON.stringify({event_name:eventName,email:authEmail(),path:location.pathname,metadata:metadata||{}})}).catch(()=>{})}catch{}}
async function syncKOSCHAccess(){try{if(!authJwt())return;const r=await fetch('/api/auth/premium-access',{headers:{Authorization:'Bearer '+authJwt()},credentials:'same-origin'}),d=await r.json().catch(()=>({})),a=d.access||{};document.querySelectorAll('[data-kosch-access-status]').forEach(el=>{el.textContent=a.active?`KOSCH ${String(a.token_tier||'basic').toUpperCase()} · AKTİF`:'KOSCH doğrulaması gerekli';el.dataset.active=a.active?'true':'false'})}catch{}}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',syncKOSCHAccess);else syncKOSCHAccess();
return{track,syncKOSCHAccess};
}());
