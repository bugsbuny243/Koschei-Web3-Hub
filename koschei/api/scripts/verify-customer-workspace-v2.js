'use strict';
const fs=require('node:fs');
const path=require('node:path');
const root=path.resolve(__dirname,'..');
const html=fs.readFileSync(path.join(root,'public','dashboard.html'),'utf8');
const js=fs.readFileSync(path.join(root,'public','js','customer-workspace-v2.js'),'utf8');
const css=fs.readFileSync(path.join(root,'public','css','koschei-dashboard.css'),'utf8');

function requireText(source,needle,label){if(!source.includes(needle))throw new Error(`${label}: missing ${needle}`);}
function forbid(source,pattern,label){if(pattern.test(source))throw new Error(`${label}: forbidden pattern ${pattern}`);}

// Current customer workspace surface. The dashboard is intentionally a clean,
// scoped product surface rather than the retired command-universe shell.
requireText(html,'/css/koschei-dashboard.css?v=1','dashboard scoped style');
requireText(html,'/js/customer-workspace-v2.js?v=2','dashboard account-data controller');
requireText(html,'/js/koschei-dashboard.js?v=3','dashboard presentation controller');
requireText(html,'id="workspaceLatestReport"','latest investigation mount');
requireText(html,'id="workspaceAlerts"','alerts mount');
requireText(html,'id="workspaceLiveState"','live account-state mount');
requireText(html,'RECENT CANONICAL INVESTIGATION','recent canonical investigation copy');
requireText(html,'Investigation jobs','history KPI copy');
requireText(html,'Professional access','Professional access KPI');
requireText(html,'Customer security workspace','customer workspace boundary');
requireText(html,'Security Overview','current dashboard headline');
requireText(html,'ARVIS intelligence map','ARVIS intelligence surface');
requireText(html,'Live operational truth','operational truth surface');
requireText(html,'No fake telemetry','no synthetic telemetry boundary');
requireText(html,'Missing evidence remains unknown.','evidence-gap boundary');
requireText(html,'Solana is the live chain core.','live-chain boundary');
requireText(html,'Other chains <em>architecture direction</em></span><b>NOT LIVE</b>','future-chain boundary');
requireText(html,'Status: building. No live capability is fabricated in the customer interface.','defense-validation capability boundary');
requireText(html,'Status: expansion architecture. Networks beyond the current live core are not presented as active coverage.','cross-chain capability boundary');
forbid(html,/PROFESSIONAL · ARVIS COMMAND UNIVERSE|customer-command-universe-v2\.js|customer-command-center-v1\.js|id="workspaceMissionControl"/,'retired command-universe contract');
forbid(html,/ARVIS early access|Preview monitored targets|STARTER\+|ENTERPRISE\+/i,'retired commercial or preview copy');
forbid(html,/holder access|Checking holder access/i,'legacy holder access copy');

// Account state remains sourced from authenticated server APIs. Missing sources
// stay unavailable instead of being filled with synthetic metrics.
requireText(js,"read('/api/auth/premium-access')",'Professional access source');
requireText(js,"read('/api/v1/radar/jobs/')",'canonical history source');
requireText(js,"read('/api/watchlist')",'watchlist source');
requireText(js,"read('/api/watchlist/alerts')",'alerts source');
requireText(js,'if(!KoscheiAuth.isLoggedIn())','signed-out privacy boundary');
requireText(js,"state.dataset.state='signed_out'",'signed-out UI state');
requireText(js,"data.schema_version!=='koschei-customer-investigation-history-v1'",'history schema boundary');
requireText(js,"data.source!=='web3_jobs'",'history source boundary');
requireText(js,"data.job_type!=='canonical_investigation'",'history job-type boundary');
requireText(js,"if(signed===true&&signature&&ruleset)return'SIGNED'",'strict signed evidence state');
requireText(js,"if(signed===true)return'SIGNATURE INCOMPLETE'",'incomplete signed evidence state');
requireText(js,'historyAvailable=Array.isArray(investigationHistory)','history unavailable-not-empty boundary');
requireText(js,"const plan=text(access.plan||'none').toUpperCase()",'plan projection');
requireText(js,'access.outputs_remaining','remaining capacity');
requireText(js,'access.outputs_total','total capacity');
requireText(js,'No active paid SaaS entitlement.','inactive server entitlement boundary');
requireText(js,'Professional plan required.','Professional watchlist boundary');
requireText(js,"encodeURIComponent(target)",'safe target navigation');
requireText(js,"!text(item.read_at)",'existing alert unread handling');
requireText(js,'availableSources=[accessResult.ok,historyAvailable,watchResult.ok,alertsResult.ok]','source availability truth');
if(js.includes('/api/v1/unified/reports'))throw new Error('workspace must not call removed unified-reports frontend contract');
if(js.includes('/api/v1/investigations/history'))throw new Error('workspace must use the canonical radar jobs collection');
if(/token_tier|token_amount|holder access/i.test(js))throw new Error('workspace must not derive access from token holdings');
if(js.includes('Math.random('))throw new Error('workspace must not fabricate live metrics');
if(/\bfetch\s*\(/.test(js))throw new Error('workspace account data must use KoscheiAuth.apiCall instead of unauthenticated fetch');

// Scoped dashboard bundle must retain the live-account and evidence card styles.
for(const marker of ['.workspace-live','.workspace-alert','.workspace-report-card','.workspace-kpi','.intel-map','.status-row'])requireText(css,marker,`dashboard style ${marker}`);

console.log('customer workspace current evidence-first contract: ok');
