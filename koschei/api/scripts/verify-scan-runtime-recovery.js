'use strict';

const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');

const rootDir=path.resolve(__dirname,'..');
const ownerSource=fs.readFileSync(path.join(rootDir,'public','js','owner-court-ui.js'),'utf8');
const publicSource=fs.readFileSync(path.join(rootDir,'public','js','public-solana-scan.js'),'utf8');
const ownerHTML=fs.readFileSync(path.join(rootDir,'public','owner-production.html'),'utf8');
const dashboardHTML=fs.readFileSync(path.join(rootDir,'public','dashboard.html'),'utf8');
const dashboardSource=fs.readFileSync(path.join(rootDir,'public','js','koschei-dashboard.js'),'utf8');
const serverSource=fs.readFileSync(path.join(rootDir,'internal','http','server.go'),'utf8');
const unifiedHandlerSource=fs.readFileSync(path.join(rootDir,'internal','handlers','owner_unified_radar.go'),'utf8');

function assert(condition,message){if(!condition)throw new Error(message)}

async function ownerScenario(status,payload){
  const resultRoot={innerHTML:'',querySelector:()=>null,insertAdjacentHTML:()=>{}};
  let directCalls=0;
  let jobCalls=0;
  const directResult={ok:true,source:'live_direct_scan'};
  const window={OwnerRadarKit:{
    scan:async(target,root)=>{
      directCalls++;
      assert(target==='test-mint','direct fallback target changed');
      assert(root===resultRoot,'direct fallback result root changed');
      return directResult;
    },
    renderUnified:()=>{}
  }};
  const context={
    window,
    document:{getElementById:id=>id==='result'?resultRoot:null},
    fetch:async()=>{
      jobCalls++;
      return{ok:status>=200&&status<300,status,json:async()=>payload};
    },
    setTimeout,
    clearTimeout,
    Promise
  };
  vm.runInNewContext(ownerSource,context,{filename:'owner-court-ui.js'});
  let result;
  let error;
  try{result=await window.OwnerRadarKit.scan('test-mint','result')}catch(caught){error=caught}
  return{result,error,directCalls,jobCalls};
}

(async()=>{
  const unavailable=await ownerScenario(503,{error:'database unavailable'});
  assert(!unavailable.error,`stateless owner fallback failed: ${unavailable.error?.message}`);
  assert(unavailable.jobCalls===1,'canonical owner job was not attempted exactly once');
  assert(unavailable.directCalls===1,'live owner scan fallback did not run exactly once');
  assert(unavailable.result?.source==='live_direct_scan','live owner scan result was not returned');

  const invalid=await ownerScenario(422,{ok:false,error:'unsupported_canonical_job_target'});
  assert(Boolean(invalid.error),'invalid owner target error was swallowed');
  assert(invalid.directCalls===0,'invalid owner target incorrectly fell back to a second scan');

  assert(serverSource.includes('mux.HandleFunc("/api/owner/arvis/scan", ownerOnly(h, method("POST", h.OwnerUnifiedRadarScan)))'),'owner direct scan is still blocked by requiresDB before stateless fallback');
  assert(serverSource.includes('mux.HandleFunc("/api/owner/radar/unified", ownerOnly(h, method("POST", h.OwnerUnifiedRadarScan)))'),'owner unified scan is still blocked by requiresDB before stateless fallback');
  assert(!serverSource.includes('mux.HandleFunc("/api/owner/arvis/scan", requiresDB'),'owner direct scan unexpectedly regained a database gate');
  assert(!serverSource.includes('mux.HandleFunc("/api/owner/radar/unified", requiresDB'),'owner unified scan unexpectedly regained a database gate');
  assert(unifiedHandlerSource.includes('ownerUnifiedWalletRadarStateless'),'wallet scan has no stateless execution path');
  assert(unifiedHandlerSource.includes('"execution_mode": "stateless_live"'),'stateless wallet response does not expose execution mode');

  // Keep the retained legacy renderer's bounded server-budget contract until
  // that renderer is separately proven orphaned and removed.
  const timeout=publicSource.match(/const TOKEN_SCAN_TIMEOUT_MS=(\d+);/);
  assert(timeout,'retained public token renderer timeout constant is missing');
  assert(Number(timeout[1])>=180000,'retained public token renderer aborts before the server investigation budget');
  assert(publicSource.includes("url.includes('/api/token/scan')?TOKEN_SCAN_TIMEOUT_MS"),'retained public token request does not use the extended timeout');

  assert(ownerHTML.includes('/js/owner-court-ui.js?v=4'),'owner scan recovery cache key is stale');
  assert(dashboardHTML.includes('/js/koschei-dashboard.js?v=5'),'customer dashboard runtime cache key is stale');
  assert(dashboardHTML.includes('id="transaction-preflight"'),'customer dashboard lost the canonical preflight mount');
  assert(dashboardSource.includes("fetch('/api/public/transaction-simulate'"),'customer dashboard lost stateless transaction simulation');
  assert(!fs.existsSync(path.join(rootDir,'public','scan.html')),'retired classic scan page returned');

  console.log('stateless owner fallback, retained token timeout and dashboard preflight contracts: ok');
})().catch(error=>{
  console.error(error);
  process.exitCode=1;
});
