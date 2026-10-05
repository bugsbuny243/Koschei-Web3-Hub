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
requireValue(manifest.product==='Koschei Web3 Hub','unified product identity changed');
requireValue(manifest.version==='security-ecosystem-v8-unified-professional','manifest version must be v8 unified Professional');
requireValue(manifest.surface==='Unified Security Ecosystem','canonical surface label missing');
requireValue(manifest.ecosystem?.runtime_integration_state==='federated_active','federated integration state changed');
requireValue(manifest.ecosystem?.projects?.some(project=>project.id==='koschei-web3-hub'),'Web3 project missing');
requireValue(manifest.ecosystem?.projects?.some(project=>project.id==='koschei-sentinel'),'Sentinel project missing');
requireValue(manifest.ecosystem?.projects?.some(project=>project.id==='koschei-lang'),'Lang project missing');
requireValue(manifest.ecosystem?.official_asset?.mint==='7X9V77axASFAV8hKqqn2EfyAz4Qz3tceN8iikfukLqy1','official KOSC mint mismatch');
requireValue(manifest.ecosystem?.official_asset?.symbol==='KOSC','official KOSC symbol mismatch');
requireValue(manifest.ecosystem?.official_asset?.single_official_token===true,'KOSC must remain the single official ecosystem token');
requireValue(manifest.ecosystem?.official_asset?.separate_module_tokens===false,'separate Sentinel/Lang tokens must stay disabled');
requireValue(manifest.ecosystem?.official_asset?.ecosystem_scope?.includes('koschei-web3-hub'),'KOSC Web3 scope missing');
requireValue(manifest.ecosystem?.official_asset?.ecosystem_scope?.includes('koschei-sentinel'),'KOSC Sentinel scope missing');
requireValue(manifest.ecosystem?.official_asset?.ecosystem_scope?.includes('koschei-lang'),'KOSC Lang scope missing');
requireValue(manifest.ecosystem?.official_asset?.holdings_grant_access===false,'KOSC holdings must not grant access');
requireValue(manifest.integration_policy?.arvis_deterministic_verdict_authority===true,'ARVIS deterministic authority changed');
requireValue(manifest.integration_policy?.sentinel_fabric_registered===true,'Sentinel Fabric registration missing');
requireValue(manifest.integration_policy?.sentinel_observe_first===true,'Sentinel observe-first boundary changed');
requireValue(manifest.integration_policy?.sentinel_customer_requests_enabled===false,'Sentinel customer model calls must stay gated until promotion');
requireValue(manifest.integration_policy?.sentinel_verdict_authority===false,'Sentinel must not gain verdict authority');
requireValue(manifest.integration_policy?.language_professional_package_included===true,'Lang Professional package inclusion missing');
requireValue(manifest.integration_policy?.language_runtime_build_dependency_enabled===false,'Lang must not become an implicit Web3 build dependency');
requireValue(manifest.integration_policy?.telegram_arvis_customer_delivery===true,'ARVIS Telegram delivery contract missing');
requireValue(manifest.provider_policy?.missing_provider_data==='unavailable_or_withheld_not_fabricated','missing-provider fail-closed policy changed');
requireValue(Array.isArray(manifest.immutable_rules)&&manifest.immutable_rules.includes('No evidence, no claim'),'no-evidence/no-claim rule missing');
requireValue(manifest.access_model?.professional_price_usd===199,'Professional price must remain USD 199');
requireValue(manifest.access_model?.settlement_channels?.includes('polar'),'Polar settlement channel missing');
requireValue(manifest.access_model?.settlement_channels?.includes('kosc'),'KOSC settlement channel missing');
requireValue(manifest.access_model?.included_products?.includes('Koschei Web3 Hub'),'Web3 Professional inclusion missing');
requireValue(manifest.access_model?.included_products?.includes('ARVIS'),'ARVIS Professional inclusion missing');
requireValue(manifest.access_model?.included_products?.includes('Koschei Sentinel'),'Sentinel Professional inclusion missing');
requireValue(manifest.access_model?.included_products?.includes('Koschei Lang'),'Lang Professional inclusion missing');
requireValue(manifest.access_model?.holdings_grant_access===false,'holder access must remain disabled');
requireValue(manifest.access_model?.premium?.includes('deep scan'),'premium canonical Deep Scan capability missing');
requireValue(manifest.access_model?.premium?.includes('ARVIS scan delivery to paired Telegram'),'ARVIS Telegram Professional capability missing');
requireValue(manifest.access_model?.premium?.includes('Koschei Lang licensed developer toolchain'),'Lang Professional capability missing');
requireValue(!manifest.access_model?.premium?.includes('security radar'),'legacy Security Radar capability must not be advertised as canonical');
const surfaces=Array.isArray(manifest.customer_surfaces)?manifest.customer_surfaces:[];
requireValue(surfaces.includes('/scan?mode=deep'),'canonical Deep Scan customer surface missing');
requireValue(surfaces.includes('/pricing'),'canonical Professional pricing surface missing');
requireValue(surfaces.includes('/api/customer/crypto-brief'),'customer messaging/pairing surface missing');
requireValue(!surfaces.includes('/security-radar'),'legacy security-radar must not be a canonical customer surface');
requireValue(!surfaces.includes('/kosch-access'),'legacy KOSCH alias must not be a canonical customer surface');

requireText(launches,'content="0;url=/scan?mode=deep"','launches canonical redirect');
requireText(launches,'href="/scan?mode=deep"','launches canonical action');
requireText(launches,"location.replace('/scan?mode=deep')",'launches JavaScript redirect');
if(launches.includes('/security-radar'))throw new Error('launches must not send users to legacy security-radar');

requireText(aliases,'[]string{"/security-radar", "/security-radar/", "/security-radar.html"}','legacy inbound radar aliases');
requireText(aliases,'registerScanModeRedirect(mux, route, "deep")','legacy radar deep-mode preservation');
requireText(aliases,'http.StatusPermanentRedirect','legacy alias permanent redirect contract');
console.log('canonical unified product contract: ok');
