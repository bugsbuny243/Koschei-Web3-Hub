'use strict';

const fs=require('node:fs');
const path=require('node:path');

const root=path.resolve(__dirname,'..');
const source=fs.readFileSync(path.join(root,'public','js','evm-spending-intelligence.js'),'utf8');

function requireText(needle,label){
  if(!source.includes(needle))throw new Error(`${label}: missing ${needle}`);
}
function forbid(pattern,label){
  if(pattern.test(source))throw new Error(`${label}: forbidden ${pattern}`);
}

requireText('function validateObservedAllowance(payload,expected)','approval response binding gate');
requireText('function validateSpenderAuthority(authority,expected)','nested authority binding gate');
requireText("payload?.schema_version!=='koschei-approval-scan-v1'",'approval schema binding');
requireText("norm(data.network)!==norm(expected.network)",'network binding');
requireText("norm(data.token)!==norm(expected.token)",'token binding');
requireText("norm(data.owner)!==norm(expected.owner)",'owner binding');
requireText("norm(data.spender)!==norm(expected.spender)",'spender binding');
requireText("norm(authority.network)!==norm(expected.network)",'authority network binding');
requireText("norm(authority.spender)!==norm(expected.spender)",'authority spender binding');
requireText("authority.trust?.observed!==true",'authority observed trust gate');
requireText("authority.trust?.verified===true||authority.trust?.authorized===true||authority.trust?.finalized===true",'authority trust promotion rejection');
requireText("norm(data.evidence_status)!=='observed'||norm(data.live_availability)!=='checked'",'observed evidence gate');
requireText("typeof data.amount!=='string'||!/^\\d+$/.test(data.amount)",'amount evidence gate');
requireText("data.trust?.observed!==true",'observed trust gate');
requireText("data.trust?.verified===true||data.trust?.authorized===true||data.trust?.finalized===true",'trust promotion rejection');
requireText('validateSpenderAuthority(data.spender_authority,expected);','nested authority pre-render validation');
requireText('validateObservedAllowance(data,values);','pre-render validation');
requireText('const controller=new AbortController();','bounded approval transport');
requireText('setTimeout(()=>controller.abort(),15000)','15 second approval timeout');
requireText("cache:'no-store',credentials:'same-origin',signal:controller.signal",'approval fetch transport policy');
requireText("controller.signal.aborted?'approval evidence timed out after 15 seconds'",'timeout failure copy');
requireText('clearTimeout(timer);','approval timeout cleanup');
requireText('result.hidden=true;','error hides stale result');
requireText('CURRENT ALLOWANCE ≠ APPROVAL PROVENANCE','provenance boundary copy');
requireText('APPROVAL ≠ SAFE SPENDER','safety boundary copy');
forbid(/data\.amount\s*\|\|\s*['"]0['"]/, 'missing amount must not become zero');
forbid(/data\.evidence_status\s*\|\|\s*['"]unknown['"]/, 'missing evidence must fail before render');

console.log('EVM spending intelligence fail-closed contract: ok');
