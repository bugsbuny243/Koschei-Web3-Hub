'use strict';

const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');

const rootDir=path.resolve(__dirname,'..');
const ownerSource=fs.readFileSync(path.join(rootDir,'public','js','owner-court-ui.js'),'utf8');
const deepRouteSource=fs.readFileSync(path.join(rootDir,'public','js','professional-deep-scan-route.js'),'utf8');
const globalShellSource=fs.readFileSync(path.join(rootDir,'public','js','koschei-global-shell.js'),'utf8');
const publicSource=fs.readFileSync(path.join(rootDir,'public','js','public-solana-scan.js'),'utf8');
const ownerHTML=fs.readFileSync(path.join(rootDir,'public','owner-production.html'),'utf8');
const scanHTML=fs.readFileSync(path.join(rootDir,'public','scan.html'),'utf8');
const serverSource=fs.readFileSync(path.join(rootDir,'internal','http','server.go'),'utf8');
const unifiedHandlerSource=fs.readFileSync(path.join(rootDir,'internal','handlers','owner_unified_radar.go'),'utf8');

function assert(condition,message){if(!condition)throw new Error(message)}

class FakeResponse{
  constructor(body='',init={}){
    this._body=String(body??'');
    this.status=Number(init.status||200);
    this.statusText=String(init.statusText||'');
    this.ok=this.status>=200&&this.status<300;
    const raw=init.headers||{};
    this.headers={get:name=>Object.entries(raw).find(([key])=>key.toLowerCase()===String(name).toLowerCase())?.[1]||null};
  }
  async text(){return this._body}
  async json(){return JSON.parse(this._body||'{}')}
}

async function ownerScenario(status,payload){
  const resultRoot={innerHTML:'',querySelector:()=>null,insertAdjacentHTML:()=>{}};
  const calls=[];
  let rendered=null;
  const window={OwnerRadarKit:{lastScan:null,renderUnified:(root,data)=>{assert(root===resultRoot,'owner render root changed');rendered=data;}}};
  const context={
    window,
    document:{getElementById:id=>id==='result'?resultRoot:null},
    fetch:async(input,init={})=>{
      calls.push({input:String(input),init});
      return new FakeResponse(JSON.stringify(payload),{status,headers:{'Content-Type':'application/json'}});
    },
    Promise,
  };
  vm.runInNewContext(ownerSource,context,{filename:'owner-court-ui.js'});
  let result,error;
  try{result=await window.OwnerRadarKit.scan('test-mint','result')}catch(caught){error=caught}
  return{result,error,calls,rendered,lastScan:window.OwnerRadarKit.lastScan};
}

async function customerDeepScenario(){
  const calls=[];
  const window={
    location:{origin:'https://tradepigloball.co',pathname:'/scan',search:'?mode=deep'},
    fetch:async(input,init={})=>{
      const url=new URL(String(input),window.location.origin);
      calls.push({url:url.toString(),method:String(init.method||'GET').toUpperCase()});
      assert(url.pathname==='/api/v1/radar/detail','Professional deep scan did not use canonical radar detail');
      assert(url.searchParams.get('target')==='test-mint','canonical customer target changed');
      return new FakeResponse(JSON.stringify({target:'test-mint',final_verdict:{grade:'B',signed:true}}),{status:200,headers:{'Content-Type':'application/json'}});
    },
  };
  const context={window,URL,URLSearchParams,Response:FakeResponse};
  vm.runInNewContext(deepRouteSource,context,{filename:'professional-deep-scan-route.js'});
  const response=await window.fetch('/api/token/scan',{
    method:'POST',
    credentials:'same-origin',
    body:JSON.stringify({mint:'test-mint',network:'solana-mainnet'}),
  });
  const data=await response.json();
  return{calls,data,response};
}

(async()=>{
  const owner=await ownerScenario(200,{target:'test-mint',court:{status:'ready'}});
  assert(!owner.error,`owner unified deep scan failed: ${owner.error?.message}`);
  assert(owner.calls.length===1,'owner deep scan made more than one transport request');
  assert(new URL(owner.calls[0].input,'https://tradepigloball.co').pathname==='/api/owner/radar/unified','owner deep scan did not use canonical live unified route');
  assert(!ownerSource.includes("fetch('/api/owner/radar/jobs'"),'owner deep scan still attempts optional canonical job route first');
  assert(owner.result?.target==='test-mint','owner unified result was not returned');
  assert(owner.lastScan?.target==='test-mint','owner latest scan state was not retained');

  const owner404=await ownerScenario(404,{error:'not_found'});
  assert(Boolean(owner404.error),'owner unified 404 was swallowed instead of surfacing the real route failure');
  assert(owner404.calls.length===1,'owner 404 triggered a legacy secondary route');

  const customer=await customerDeepScenario();
  assert(customer.calls.length===1,'customer deep scan made more than one transport request');
  assert(customer.data?.investigation_report?.target==='test-mint','customer deep scan response was not normalized to the existing UI contract');
  assert(customer.response.headers.get('X-Koschei-Deep-Route')==='canonical-radar-detail','customer deep route provenance header missing');
  assert(!customer.calls.some(call=>new URL(call.url).pathname==='/api/token/scan'),'customer deep scan still hit legacy token scan route');

  assert(serverSource.includes('mux.HandleFunc("/api/owner/arvis/scan", ownerOnly(h, method("POST", h.OwnerUnifiedRadarScan)))'),'owner direct scan route lost stateless handler');
  assert(serverSource.includes('mux.HandleFunc("/api/owner/radar/unified", ownerOnly(h, method("POST", h.OwnerUnifiedRadarScan)))'),'owner unified route lost stateless handler');
  assert(serverSource.includes('mux.HandleFunc("/api/v1/radar/detail"'),'Professional radar detail route is missing');
  assert(unifiedHandlerSource.includes('ownerUnifiedWalletRadarStateless'),'wallet scan has no stateless execution path');
  assert(unifiedHandlerSource.includes('"execution_mode": "stateless_live"'),'stateless wallet response does not expose execution mode');

  const timeout=publicSource.match(/const TOKEN_SCAN_TIMEOUT_MS=(\d+);/);
  assert(timeout&&Number(timeout[1])>=180000,'public token scan aborts before the server investigation budget');
  assert(globalShellSource.includes("if(path.indexOf('/api/v1/radar/detail')===0)return 210000"),'Professional deep detail timeout is below full investigation budget');
  assert(globalShellSource.includes("['/pricing','Professional']"),'Professional package is not exposed in global navigation');
  assert(globalShellSource.includes('koschei-professional-strip'),'Professional access is not exposed on customer work surfaces');
  assert(globalShellSource.includes('/js/professional-deep-scan-route.js?v=1'),'canonical Professional deep route loader is missing');
  assert(ownerHTML.includes('/js/owner-court-ui.js?v=5'),'owner deep scan cache key is stale');
  assert(scanHTML.includes('/js/public-solana-scan.js?v=13'),'public scan runtime cache key is stale');

  console.log('Professional customer and owner deep scan canonical routing contracts: ok');
})().catch(error=>{
  console.error(error);
  process.exitCode=1;
});
