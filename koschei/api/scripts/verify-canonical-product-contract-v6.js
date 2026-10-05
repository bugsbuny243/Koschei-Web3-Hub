'use strict';
const fs=require('node:fs');
const path=require('node:path');
const root=path.resolve(__dirname,'..');
const manifestPath=path.join(root,'public','security-ecosystem.json');
const launches=fs.readFileSync(path.join(root,'public','launches.html'),'utf8');
const aliases=fs.readFileSync(path.join(root,'internal','http','static_aliases.go'),'utf8');
const manifest=JSON.parse(fs.readFileSync(manifestPath,'utf8'));

function requireValue(condition,label){if(!condition)throw new Error(label);}
function requireText(source,needle,label){if(!source.includes(needle))throw new Error(`${label}: missing ${needle}`);}

requireValue(manifest.ok===true,'manifest must remain ok');
requireValue(manifest.product==='Koschei Web3 Hub','standalone Web3 product identity changed');
requireValue(manifest.engine==='ARVIS','ARVIS engine identity missing');
requireValue(manifest.version==='web3-v8-standalone-crypto-intelligence','manifest version must be standalone Web3 v8');
requireValue(manifest.surface==='Crypto Security & Intelligence','canonical Web3 surface label missing');
requireValue(manifest.product_boundary?.standalone_product===true,'Web3 must remain standalone');
requireValue(manifest.product_boundary?.sold_separately===true,'Web3 separate-sale policy missing');
requireValue(manifest.product_boundary?.shared_entitlement_with_sentinel===false,'Sentinel must not be bundled into Web3 entitlement');
requireValue(manifest.product_boundary?.shared_entitlement_with_lang===false,'Lang must not be bundled into Web3 entitlement');
requireValue(manifest.ecosystem?.runtime_integration_state==='web3_standalone','runtime integration boundary changed');
requireValue(Array.isArray(manifest.ecosystem?.projects)&&manifest.ecosystem.projects.length===1,'Web3 public manifest must not list Sentinel/Lang as bundled projects');
requireValue(manifest.ecosystem?.projects?.[0]?.id==='koschei-web3-hub','Web3 project identity missing');
requireValue(manifest.ecosystem?.official_asset?.mint==='7X9V77axASFAV8hKqqn2EfyAz4Qz3tceN8iikfukLqy1','official KOSC mint mismatch');
requireValue(manifest.ecosystem?.official_asset?.symbol==='KOSC','official KOSC symbol mismatch');
requireValue(manifest.ecosystem?.official_asset?.holdings_grant_access===false,'KOSC holdings must not grant access');
requireValue(manifest.standalone_policy?.web3_runtime_independent===true,'Web3 runtime independence changed');
requireValue(manifest.standalone_policy?.sentinel_is_included===false,'Sentinel must not be included in Web3');
requireValue(manifest.standalone_policy?.lang_is_included===false,'Lang must not be included in Web3');
requireValue(manifest.standalone_policy?.cross_product_interoperability_never_implies_shared_entitlement===true,'cross-product entitlement boundary changed');
requireValue(manifest.provider_policy?.missing_provider_data==='unavailable_or_withheld_not_fabricated','missing-provider fail-closed policy changed');
requireValue(Array.isArray(manifest.immutable_rules)&&manifest.immutable_rules.includes('No evidence, no claim'),'no-evidence/no-claim rule missing');
requireValue(manifest.access_model?.professional_price_usd===199,'Web3 Professional price must remain USD 199');
requireValue(manifest.access_model?.settlement_channels?.includes('polar'),'Polar settlement channel missing');
requireValue(manifest.access_model?.settlement_channels?.includes('kosc'),'KOSC settlement channel missing');
requireValue(manifest.access_model?.holdings_grant_access===false,'holder access must remain disabled');
requireValue(manifest.access_model?.premium?.includes('deep scan'),'premium canonical Deep Scan capability missing');
requireValue(manifest.access_model?.premium?.includes('ARVIS scan delivery to paired Telegram'),'paired Telegram delivery missing');
requireValue(!manifest.access_model?.premium?.includes('Koschei Sentinel'),'Sentinel must not leak into Web3 plan');
requireValue(!manifest.access_model?.premium?.includes('Koschei Lang'),'Lang must not leak into Web3 plan');
requireValue(!manifest.access_model?.premium?.includes('security radar'),'legacy Security Radar capability must not be advertised as canonical');
const surfaces=Array.isArray(manifest.customer_surfaces)?manifest.customer_surfaces:[];
requireValue(surfaces.includes('/scan?mode=deep'),'canonical Deep Scan customer surface missing');
requireValue(surfaces.includes('/pricing'),'canonical Web3 Professional pricing surface missing');
requireValue(!surfaces.includes('/security-radar'),'legacy security-radar must not be a canonical customer surface');
requireValue(!surfaces.includes('/kosch-access'),'legacy KOSCH alias must not be a canonical customer surface');

requireText(launches,'content="0;url=/scan?mode=deep"','launches canonical redirect');
requireText(launches,'href="/scan?mode=deep"','launches canonical action');
requireText(launches,"location.replace('/scan?mode=deep')",'launches JavaScript redirect');
if(launches.includes('/security-radar'))throw new Error('launches must not send users to legacy security-radar');

requireText(aliases,'[]string{"/security-radar", "/security-radar/", "/security-radar.html"}','legacy inbound radar aliases');
requireText(aliases,'registerScanModeRedirect(mux, route, "deep")','legacy radar deep-mode preservation');
requireText(aliases,'http.StatusPermanentRedirect','legacy alias permanent redirect contract');
console.log('canonical standalone Web3 product contract: ok');
