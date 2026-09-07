'use strict';
const fs=require('node:fs');
const path=require('node:path');
const root=path.resolve(__dirname,'..');
const html=fs.readFileSync(path.join(root,'public','pricing.html'),'utf8');
const css=fs.readFileSync(path.join(root,'public','css','koschei.css'),'utf8');
const universe=fs.readFileSync(path.join(root,'public','css','koschei.css'),'utf8');
const planAccess=fs.readFileSync(path.join(root,'internal','handlers','plan_access.go'),'utf8');
const premiumAccess=fs.readFileSync(path.join(root,'internal','handlers','premium_access_status.go'),'utf8');
const checkoutGate=fs.readFileSync(path.join(root,'internal','handlers','polar_checkout_gate.go'),'utf8');
const billingRoutes=fs.readFileSync(path.join(root,'internal','http','billing_routes.go'),'utf8');

function requireText(source,needle,label){if(!source.includes(needle))throw new Error(`${label}: missing ${needle}`);}
function forbid(source,pattern,label){if(pattern.test(source))throw new Error(`${label}: forbidden pattern ${pattern}`);}

requireText(html,'<html lang="en">','pricing language');
requireText(html,'CURRENT DEPLOYMENT · COMMERCIAL TRUTH','deployment truth disclosure');
requireText(html,'Do not sell what the runtime cannot deliver.','commercial truth headline');
requireText(html,'Professional remains the current server-side entitlement contract','current entitlement disclosure');
requireText(html,'Commercial authorization exists. Commercial checkout is not live.','checkout non-live headline');
requireText(html,'Starter and Enterprise remain product-package directions, not active sale packages in this deployment.','planned package boundary');
requireText(html,'NOT LIVE · Durable customer investigation history','history non-live disclosure');
requireText(html,'NOT LIVE · Watchlist persistence and stored alerts','monitoring non-live disclosure');
requireText(html,'NOT LIVE · DB-backed secure checkout in the current stateless process','checkout non-live disclosure');
requireText(html,'The server decides access. The browser never invents it.','server-side entitlement authority');
requireText(html,'PROFESSIONAL</strong>','Professional policy card');
requireText(html,'NOT LIVE</strong>','checkout policy card');
requireText(html,'FAIL-CLOSED','evidence boundary card');
requireText(html,'/dashboard#capabilities','Customer Panel capability route');
requireText(html,'/dashboard#transaction-preflight','live preflight route');
requireText(html,'/dashboard#evidence','runtime truth route');
requireText(html,'/css/koschei.css?v=1','universe stylesheet');

forbid(html,/data-polar-plan/i,'checkout action while persistence is unavailable');
forbid(html,/polar-checkout-v1\.js/i,'checkout runtime while persistence is unavailable');
forbid(html,/\/arvis-chat(?:["?#])/,'retired ARVIS standalone navigation');
forbid(html,/\/reports(?:["?#])/,'retired reports standalone navigation');
forbid(html,/\/watchlist(?:["?#])/,'retired watchlist standalone navigation');
forbid(html,/\/scan\?mode=deep/,'retired Deep Scan navigation');
forbid(html,/<h2>Free Core<\/h2>/i,'free investigation tier');
forbid(html,/Request early access/i,'retired early access form');
forbid(html,/STARTER\+|ENTERPRISE\+/i,'inactive paid tier badges');
forbid(html,/\$299\s*\/\s*month|\$999\s*\/\s*month|\$4,999\s*\/\s*month/i,'invented or retired hard-coded pricing');
forbid(html,/paddle/i,'retired Paddle billing surface');
forbid(html,/data-koschei-checkout/i,'retired browser checkout action');
forbid(html,/<script(?![^>]*\bsrc=)[^>]*>/i,'inline runtime script');
forbid(html,/\son[a-z]+\s*=/i,'inline event handler');

requireText(billingRoutes,'"/api/polar/checkout", requiresDB(h, handlers.RequireAuth','checkout requires application persistence');
requireText(checkoutGate,'KOSCHEI_COMMERCIAL_CHECKOUT_ENABLED','server readiness flag');
requireText(checkoutGate,'h.PolarCheckout(w, r)','Polar checkout delegation');
requireText(planAccess,'func canonicalSaaSPlan(plan string) string','canonical SaaS plan mapping');
requireText(planAccess,'func (h *Handler) RequirePlanTier','customer entitlement authorization');
requireText(planAccess,'FROM entitlements','entitlement source');
requireText(planAccess,"status='active'",'active entitlement requirement');
requireText(planAccess,'EnforcePlanOutput','entitlement output metering');
requireText(premiumAccess,'Source:           "entitlement"','premium access entitlement source');
requireText(premiumAccess,'OutputsRemaining: evaluation.OutputsRemaining','remaining capacity response');
forbid(premiumAccess,/token_(?:tier|amount)/i,'asset-backed premium access fields');

requireText(css,'.pricing-plans','pricing layout');
requireText(css,'.pricing-policy-grid','pricing contract layout');
requireText(css,'@media(max-width:620px)','mobile pricing layout');
requireText(universe,'body.koschei-universe','universe body contract');
requireText(universe,'.universe-entry','universe entry contract');
console.log('pricing deployment truth + server entitlement contract: ok');
