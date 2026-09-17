(()=>{
'use strict';
if(window.__koscheiProfessionalDeepScanRoute)return;
window.__koscheiProfessionalDeepScanRoute=true;

const mode=new URLSearchParams(window.location.search||'').get('mode');
const path=(window.location.pathname||'').replace(/\.html$/,'').replace(/\/$/,'')||'/';
if(path!=='/scan'||mode!=='deep')return;

const previousFetch=window.fetch.bind(window);
const sameOrigin=url=>url.origin===window.location.origin;

window.fetch=async function(input,init){
  const raw=typeof input==='string'?input:(input&&input.url)||'';
  let url;
  try{url=new URL(raw,window.location.origin);}catch{return previousFetch(input,init);}
  const method=String(init?.method||'GET').toUpperCase();
  if(!sameOrigin(url)||url.pathname!=='/api/token/scan'||method!=='POST')return previousFetch(input,init);

  let body={};
  try{body=JSON.parse(String(init?.body||'{}'));}catch{}
  const target=String(body.mint||body.address||'').trim();
  const network=String(body.network||'solana-mainnet').trim()||'solana-mainnet';
  if(!target)return previousFetch(input,init);

  const detail=new URL('/api/v1/radar/detail',window.location.origin);
  detail.searchParams.set('target',target);
  detail.searchParams.set('network',network);
  const response=await previousFetch(detail.toString(),{
    method:'GET',
    credentials:'same-origin',
    cache:'no-store',
    headers:{'Accept':'application/json'},
    signal:init?.signal,
  });
  const text=await response.text();
  if(!response.ok){
    return new Response(text,{status:response.status,statusText:response.statusText,headers:{'Content-Type':response.headers.get('Content-Type')||'application/json'}});
  }
  let report={};
  try{report=JSON.parse(text);}catch{
    return new Response(JSON.stringify({error:'invalid_deep_scan_response'}),{status:502,headers:{'Content-Type':'application/json'}});
  }
  return new Response(JSON.stringify({investigation_report:report}),{
    status:200,
    headers:{'Content-Type':'application/json','X-Koschei-Deep-Route':'canonical-radar-detail'},
  });
};
})();
