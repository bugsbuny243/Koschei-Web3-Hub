#!/usr/bin/env node

import { readFile } from 'node:fs/promises';

const baseURL=String(process.env.KOSCHEI_BASE_URL||'https://tradepigloball.co').replace(/\/$/,'');
const ownerSecret=String(process.env.KOSCHEI_OWNER_SECRET||'').trim();
const allowPartial=/^(1|true|yes)$/i.test(String(process.env.KOSCHEI_ACCEPTANCE_ALLOW_PARTIAL||''));
const inputPath=process.argv[2];

if(!ownerSecret)throw new Error('KOSCHEI_OWNER_SECRET is required');
if(!inputPath)throw new Error('usage: node scripts/run-production-investigation-acceptance.mjs <targets.json>');

const targets=JSON.parse(await readFile(inputPath,'utf8'));
if(!Array.isArray(targets)||targets.length<1||targets.length>10)throw new Error('targets.json must contain 1 to 10 real token targets');

const results=[];
for(const [index,item] of targets.entries()){
  const target=String(item?.target||item?.mint||'').trim();
  const profile=String(item?.profile||'standard_traded_token').trim();
  if(!target)throw new Error(`target ${index+1} is empty`);
  const started=Date.now();
  const response=await fetch(`${baseURL}/api/owner/arvis/acceptance`,{
    method:'POST',
    headers:{'Content-Type':'application/json','x-koschei-secret':ownerSecret},
    body:JSON.stringify({target,profile,network:'solana-mainnet'})
  });
  const payload=await response.json().catch(()=>({error:'invalid_json_response'}));
  const acceptance=payload?.acceptance||{};
  const row={
    target,
    profile,
    http_status:response.status,
    status:String(acceptance.status||'fail'),
    duration_ms:Date.now()-started,
    blockers:Array.isArray(acceptance.blockers)?acceptance.blockers:[],
    warnings:Array.isArray(acceptance.warnings)?acceptance.warnings:[],
    metrics:acceptance.metrics||{},
    caller_parity:acceptance.caller_parity||{}
  };
  results.push(row);
  process.stdout.write(`${JSON.stringify(row)}\n`);
}

const summary={
  version:'koschei-production-acceptance-run-v1',
  base_url:baseURL,
  targets:results.length,
  passed:results.filter(item=>item.status==='pass').length,
  partial:results.filter(item=>item.status==='partial').length,
  failed:results.filter(item=>item.status==='fail').length,
  allow_partial:allowPartial,
  blocker_codes:[...new Set(results.flatMap(item=>item.blockers.map(blocker=>blocker.code)).filter(Boolean))].sort(),
  warning_codes:[...new Set(results.flatMap(item=>item.warnings.map(warning=>warning.code)).filter(Boolean))].sort()
};
process.stdout.write(`${JSON.stringify({summary})}\n`);
if(summary.failed>0||(!allowPartial&&summary.partial>0))process.exitCode=1;
