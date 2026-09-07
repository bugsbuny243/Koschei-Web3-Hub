'use strict';
const fs=require('node:fs');
const path=require('node:path');
const root=path.resolve(__dirname,'..');
const manifest=JSON.parse(fs.readFileSync(path.join(root,'public','security-ecosystem.json'),'utf8'));
const aliases=fs.readFileSync(path.join(root,'internal','http','static_aliases.go'),'utf8');
const home=fs.readFileSync(path.join(root,'public','index.html'),'utf8');
const dashboard=fs.readFileSync(path.join(root,'public','dashboard.html'),'utf8');
const shell=fs.readFileSync(path.join(root,'public','js','koschei-global-shell.js'),'utf8');

function requireValue(condition,label){if(!condition)throw new Error(label);}
function requireText(source,needle,label){if(!source.includes(needle))throw new Error(`${label}: missing ${needle}`);}
function rejectText(source,needle,label){if(source.includes(needle))throw new Error(`${label}: contains retired ${needle}`);}

requireValue(manifest.ok===true,'manifest must remain ok');
requireValue(manifest.product==='Koschei ARVIS','product identity changed');
requireValue(manifest.version==='security-ecosystem-v7-two-surface-boundary','manifest version must be v7 two-surface boundary');
requireValue(manifest.surface==='Security Validation & Risk Intelligence','canonical product surface label changed');

requireValue(manifest.ecosystem?.runtime_integration_state==='incubation_only','external-project runtime integration boundary changed');
requireValue(manifest.ecosystem?.official_asset?.identity_only===true,'KOSCH must remain identity-only');
requireValue(manifest.incubation_policy?.sentinel_runtime_integrated===false,'Sentinel runtime boundary changed');
requireValue(manifest.incubation_policy?.language_runtime_integrated===false,'Koschei Lang runtime boundary changed');
requireValue(manifest.incubation_policy?.sentinel_verdict_authority===false,'Sentinel must not gain verdict authority');
requireValue(manifest.provider_policy?.missing_provider_data==='unavailable_or_withheld_not_fabricated','missing-provider fail-closed policy changed');
requireValue(Array.isArray(manifest.immutable_rules)&&manifest.immutable_rules.includes('No evidence, no claim'),'no-evidence/no-claim rule missing');

const surfaces=Array.isArray(manifest.customer_surfaces)?manifest.customer_surfaces:[];
requireValue(surfaces.length===2,'canonical customer surface count must be exactly two');
requireValue(surfaces[0]==='/'&&surfaces[1]==='/dashboard','canonical customer surfaces must be Home and Customer Panel');
requireValue(!surfaces.some(value=>String(value).startsWith('/scan')),'retired Scan Center must not be canonical');
requireValue(!surfaces.includes('/security-radar'),'legacy Security Radar must not be canonical');
requireValue(!surfaces.includes('/safe-check'),'legacy Safe Check must not be canonical');

const primary=manifest.access_model?.primary_product_surfaces||[];
requireValue(Array.isArray(primary)&&primary.length===2&&primary[0]==='/'&&primary[1]==='/dashboard','access model must expose only two primary product surfaces');
requireValue(manifest.access_model?.live_customer_capabilities?.includes('read-only Solana transaction simulation'),'live read-only transaction simulation capability missing');
requireValue(manifest.access_model?.persistence_backed_capabilities==='withheld_when_runtime_persistence_is_unavailable','persistence truth boundary changed');
const planned=manifest.access_model?.planned_packages||[];
requireValue(JSON.stringify(planned)===JSON.stringify(['starter','professional','enterprise']),'planned package names changed');

requireText(home,'See the blind spot.','Home surface contract');
requireText(dashboard,'id="transaction-preflight"','Customer Panel preflight mount');
requireText(dashboard,'LIVE · SOLANA MAINNET · READ ONLY','Customer Panel live capability truth');
requireText(dashboard,'NO SIGNING · NO BROADCAST','transaction authority boundary');

requireText(shell,"var links=[['/','Home'],['/dashboard','Customer Panel'],['/live','Live SOC'],['/cases','Cases']];",'two-surface global navigation');
requireText(shell,'installBoundedAPIFetch();','bounded API fetch compatibility');
requireText(shell,'translate(document.body);','translation compatibility');
rejectText(shell,"['/scan','Token Scan']",'retired scanner global navigation');
rejectText(shell,"['/scan?mode=deep','Deep Scan']",'retired deep-scan global navigation');
rejectText(shell,"['/transaction-shield','Transaction Shield']",'retired transaction-shield global navigation');
rejectText(shell,"['/safe-check','Safe Check']",'retired safe-check global navigation');

requireText(aliases,'"/scan", "/scan/", "/scan.html", "/transaction-shield"','transaction compatibility routes');
requireText(aliases,'registerCanonicalRedirect(mux, route, "/dashboard#transaction-preflight")','transaction compatibility target');
requireText(aliases,'"/security-radar", "/security-radar/", "/security-radar.html"','radar compatibility routes');
requireText(aliases,'"/launches", "/launches/", "/launches.html"','launches compatibility routes');
requireText(aliases,'registerCanonicalRedirect(mux, route, "/dashboard#capabilities")','capability compatibility target');
rejectText(aliases,'registerScanModeRedirect','retired scan-mode router');

for(const retired of [
  'public/scan.html',
  'public/launches.html',
  'public/security-ecosystem.html',
  'public/exposure-report.html',
  'public/feedback.html',
  'public/token-vesting.html',
]){
  if(fs.existsSync(path.join(root,retired)))throw new Error(`retired standalone product surface returned: ${retired}`);
}

console.log('canonical product contract v7 two-surface boundary: ok');
