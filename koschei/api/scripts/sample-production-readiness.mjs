#!/usr/bin/env node
import {mkdir,writeFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';

// Public snapshots prove only what was observed. This collector never converts
// elapsed time, a healthy HTTP response or zero work into a GA acceptance pass.
const base='https://tradepigloball.co';
const output=process.argv[2]||'diagnostics/production-sample.json';
const expected=String(process.env.GITHUB_SHA||'');
const startedAt=new Date().toISOString();
async function read(path,headers={}) {
  const start=performance.now();
  try {
    const response=await fetch(base+path,{headers,redirect:'error',signal:AbortSignal.timeout(8000),cache:'no-store'});
    const raw=await response.text();
    if(raw.length>512*1024) return {status:response.status,error:'response_too_large'};
    let body;try{body=JSON.parse(raw)}catch{return {status:response.status,error:'invalid_json'}};
    return {status:response.status,duration_ms:Math.round(performance.now()-start),body};
  } catch { return {status:null,error:'transport_unavailable'} }
}
const [version,runtime,health]=await Promise.all([
  read('/api/version'),read('/fabric/security-center/runtime-health'),read('/health')
]);
const secret=String(process.env.KOSCHEI_OWNER_SECRET||'');
let campaign={status:null,error:'owner_credential_unavailable'};
if(secret) {
  const response=await read('/api/owner/campaigns',{'x-koschei-secret':secret});
  // Never publish campaign addresses, source payloads or operator inventory.
  campaign={status:response.status,error:response.error,runtime:response.body?.runtime};
}
const revision=version.body?.build?.revision||version.body?.revision||'';
const missing=['archive_restore_proof','reorg_recovery_proof','mttd_mttr_measurement','cost_per_event_measurement','continuous_24h_window'];
if(!Array.isArray(runtime.body?.entries)) missing.push('runtime_snapshot');
if(campaign.status!==200) missing.push('durable_queue_measurement');
if(!revision||!expected||revision!==expected) missing.push('exact_deployed_revision');
const sample={version:'koschei.production-observation.v1',started_at:startedAt,observed_at:new Date().toISOString(),
  expected_revision:expected,deployed_revision:revision,readiness:'UNKNOWN',missing_evidence:missing,
  workflow_run_id:String(process.env.GITHUB_RUN_ID||''),version_response:version,runtime_response:runtime,
  health_response:health,campaign_runtime:campaign};
const payload=JSON.stringify(sample,null,2)+'\n';
await mkdir(output.slice(0,output.lastIndexOf('/'))||'.',{recursive:true});
await writeFile(output,payload);
await writeFile(output+'.sha256',createHash('sha256').update(payload).digest('hex')+'\n');
console.log(JSON.stringify({readiness:sample.readiness,deployed_revision:revision,missing_evidence:missing}));
