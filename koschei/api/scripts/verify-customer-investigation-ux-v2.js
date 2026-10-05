'use strict';
const fs=require('node:fs');
const path=require('node:path');
const root=path.resolve(__dirname,'..');
const html=fs.readFileSync(path.join(root,'public','scan.html'),'utf8');
const css=fs.readFileSync(path.join(root,'public','css','koschei.css'),'utf8');

function requireText(source,needle,label){if(!source.includes(needle))throw new Error(`${label}: missing ${needle}`);}
function forbidText(source,needle,label){if(source.includes(needle))throw new Error(`${label}: duplicate customer scanner UI returned: ${needle}`);}

requireText(html,'/css/koschei.css?v=1','scan html');
requireText(html,'ONE RADAR · ONE TARGET','single-radar heading');
requireText(html,'id="scanForm"','single request form');
requireText(html,'id="target"','single target input');
requireText(html,'id="scanNetwork"','network context');
requireText(html,'id="customerUniversalResultsWrap"','single result surface');
requireText(html,'/js/customer-scan-entry.js?v=1','target router');
requireText(html,'/js/customer-universal-address-scan-v1.js?v=3','single-radar controller');
requireText(html,'/js/public-solana-scan.js?v=14','legacy evidence adapter bootstrap');
requireText(html,'Missing evidence stays unknown.','evidence boundary');
requireText(html,'never signs, and never broadcasts.','custody boundary');

if((html.match(/<form\b/g)||[]).length!==1)throw new Error('scan html: exactly one customer request form is required');
for(const needle of [
  '<option value="token">Solana token</option>',
  '<option value="quick">Site or URL</option>',
  '<option value="transaction">Solana transaction before signing</option>',
  '<option value="spending">EVM token allowance</option>',
  '/js/customer-investigation-ux-v2.js?v=1',
  '/js/customer-arvis-premium-suite.js?v=2',
  '/js/customer-transaction-preflight-v1.js',
  '/js/evm-spending-intelligence.js',
  '/js/customer-command-center-v1.js'
])forbidText(html,needle,'scan html');

const routeIndex=html.indexOf('/js/customer-scan-entry.js?v=1');
const controllerIndex=html.indexOf('/js/customer-universal-address-scan-v1.js?v=3');
const legacyIndex=html.indexOf('/js/public-solana-scan.js?v=14');
if(routeIndex<0||controllerIndex<routeIndex||legacyIndex<controllerIndex)throw new Error('scan html: single-radar script order is invalid');

requireText(css,'.customer-result-summary','retained customer result summary styles');
requireText(css,'.customer-full-technical','retained technical disclosure styles');
requireText(css,'.customer-source-panels','retained source disclosure styles');
console.log('customer investigation UX v2 contract: true single radar ok');
