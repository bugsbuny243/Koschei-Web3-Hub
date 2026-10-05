const fs = require('fs');

const overlay = fs.readFileSync('public/js/customer-transaction-preflight-v1.js', 'utf8');
const scan = fs.readFileSync('public/scan.html', 'utf8');
const command = fs.readFileSync('public/js/customer-command-center-v1.js', 'utf8');
new Function(overlay);

// Keep the retired Professional preflight implementation testable as a rollback/internal
// artifact, but do not mount it on the customer single-radar surface. Customer target
// dispatch is now owned by the canonical ARVIS Radar flow.
const required = [
  '/api/customer/web3/transaction-preflight',
  '/api/customer/web3/transaction-state-recheck',
  "script.src='/js/koschei-auth.js?v=33'",
  'auth.apiCall(path,options)',
  'customerAPI(endpoint',
  'customerAPI(recheckEndpoint',
  "credentials:'same-origin'",
  "event.stopImmediatePropagation()",
  "form.addEventListener('submit'",
  'data-customer-transaction-preflight-result',
  'guard_complete',
  'program_policy',
  'intent_policy',
  'automatic_decode_complete',
  'cpi_asset_flow_complete',
  'authority_surface_complete',
  'threat_history_complete',
  'WITHHOLD — EVIDENCE INCOMPLETE',
  'Numeric risk scores are not the authority',
  "const safe=response.ok&&data?.ok===true&&data?.safe_to_proceed===true;",
  'if(pendingRecheck===snapshot)clearPendingRecheck();',
  "transaction.value=''"
];
for (const marker of required) {
  if (!overlay.includes(marker)) throw new Error(`missing retained preflight implementation marker: ${marker}`);
}
for (const forbidden of [
  '/api/public/transaction-simulate',
  'localStorage',
  'sessionStorage',
  'X-API-Key',
  'Authorization: Bearer'
]) {
  if (overlay.includes(forbidden)) throw new Error(`retained preflight implementation violates boundary: ${forbidden}`);
}
if (!overlay.includes("},true);")) throw new Error('retained transaction submit interception must run in capture phase');

for (const marker of [
  'customer-single-radar',
  'ONE RADAR · ONE TARGET',
  'id="target"',
  'id="scanNetwork"',
  'id="radarResults"',
  'ARVIS uses the target format and network to choose the investigation path.'
]) {
  if (!scan.includes(marker)) throw new Error(`single ARVIS Radar contract missing: ${marker}`);
}
if (scan.includes('/js/customer-transaction-preflight-v1.js')) {
  throw new Error('customer scan must not mount the retired Professional transaction preflight overlay');
}
if (scan.includes('Solana transaction before signing') || scan.includes('Professional Transaction Preflight')) {
  throw new Error('customer scan must not expose a duplicate transaction/preflight scanner mode');
}
if (!command.includes("{label:'ARVIS Radar',href:'/scan',mode:'primary'}") || command.includes("href:'/scan?mode=transaction'")) {
  throw new Error('transaction investigation must remain inside the single ARVIS Radar, not a duplicate menu entry');
}
console.log('customer transaction capability retained behind the single ARVIS Radar contract');
