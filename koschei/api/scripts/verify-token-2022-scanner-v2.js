'use strict';
const fs=require('node:fs');
const path=require('node:path');
const root=path.resolve(__dirname,'..');
const html=fs.readFileSync(path.join(root,'public','token-2022-scanner.html'),'utf8');
const js=fs.readFileSync(path.join(root,'public','js','token-2022-scanner-v2.js'),'utf8');
const css=fs.readFileSync(path.join(root,'public','css','koschei.css'),'utf8');
const server=fs.readFileSync(path.join(root,'internal','http','server.go'),'utf8');
const scanner=fs.readFileSync(path.join(root,'internal','handlers','token_scanner.go'),'utf8');
const extensions=fs.readFileSync(path.join(root,'internal','handlers','token_2022_extensions.go'),'utf8');

function requireText(source,needle,label){if(!source.includes(needle))throw new Error(`${label}: missing ${needle}`);}
function forbid(source,pattern,label){if(pattern.test(source))throw new Error(`${label}: forbidden pattern ${pattern}`);}

requireText(html,'<html lang="en">','Token-2022 scanner language');
requireText(html,'Professional SaaS plan or higher','Professional SaaS access copy');
requireText(html,'Dedicated /api/v1/token/extensions','dedicated route copy');
requireText(html,'Unresolved extension state → WITHHOLD','withhold policy copy');
requireText(html,'/scan?mode=deep','canonical Deep Scan route');
requireText(html,'/pricing','canonical plans route');
requireText(html,'KOSCH holdings and legacy holder tiers do not authorize or upgrade this surface.','explicit token separation');
requireText(html,'/js/koschei-auth.js?v=33','frozen auth client');
requireText(html,'/js/token-2022-scanner-v2.js?v=1','external Token-2022 controller');
if(html.includes('/security-radar'))throw new Error('Token-2022 scanner must not advertise legacy security-radar');
if(html.includes('/kosch-access'))throw new Error('Token-2022 scanner must not advertise legacy kosch-access');
forbid(html,/Basic-tier|Basic tier|Verify KOSCH Access|customer-session \+ KOSCH|premium Basic-tier gate|KOSCH protected/i,'stale token-backed commercial access claim');
forbid(html,/<script(?![^>]*\bsrc=)[^>]*>/i,'inline runtime script');
forbid(html,/\son[a-z]+\s*=/i,'inline event handler');

requireText(server,'planTier := func(plan string, next http.HandlerFunc) http.HandlerFunc','central SaaS plan gate');
requireText(server,'return handlers.RequireAuth(h.RequirePlanTier(plan, h.EnforcePlanOutput(next)))','central SaaS plan + output enforcement');
requireText(server,'mux.HandleFunc("/api/v1/token/extensions", solana(risk(requiresDB(h, planTier("professional", method("POST", h.TokenScan))))))','dedicated Professional TokenScan route');
requireText(server,'mux.HandleFunc("/api/token/scan", solana(risk(method("POST", h.TokenScan))))','public compatibility TokenScan route');
forbid(server,/koschTier\(|RequireTokenTier|EnforceScanQuota/,'legacy token authorization in boot chain');

// Field/tag presence is a wire-contract check, not a gofmt whitespace snapshot.
const scanFields={
  RiskLevel:'risk_level',Extensions:'extensions',ExtensionRiskPenalty:'extension_risk_penalty',
  ExtensionResolutionStatus:'extension_resolution_status',ExtensionEvidenceComplete:'extension_evidence_complete',
  TransferBehavior:'transfer_behavior',VisibilityLimitations:'visibility_limitations',
  CompatibilityWarnings:'compatibility_warnings',FinalPolicy:'final_policy',
  HolderAnalysisStatus:'holder_analysis_status',VerdictWithheld:'verdict_withheld',
  TokenHeuristicPenaltyScore:'token_heuristic_penalty_score',
  NotAFinalRiskScore:'not_a_final_risk_score',FinalScoreDisabledByPolicy:'final_score_disabled_by_policy',
  Arms:'arms',Coverage:'coverage',ArmCount:'arm_count'
};
for(const [field,tag] of Object.entries(scanFields)){
  const rx=new RegExp(`\\b${field}\\s+[^\\n]+\\x60json:"${tag}"\\x60`);
  if(!rx.test(scanner))throw new Error(`TokenScan response missing wire field ${field}: ${tag}`);
}
if(scanner.includes('`json:"score"`'))throw new Error('legacy final-looking score wire field must not be emitted');
requireText(scanner,'if holderPolicy == "withhold" {','holder evidence withhold');
requireText(scanner,'policy = "withhold"','holder policy downgrade');
if(!/VerdictWithheld:\s+holderPolicy == "withhold" \|\| !extensionEvidenceComplete/.test(scanner))throw new Error('authoritative withheld response missing');
requireText(scanner,'missing evidence is not treated as a safety signal','holder missing-evidence rule');
requireText(extensions,'Name        string         `json:"name"`','extension name field');
requireText(extensions,'Severity    string         `json:"severity"`','extension severity field');
requireText(extensions,'RiskPenalty int            `json:"risk_penalty"`','extension penalty field');
requireText(extensions,'Summary     string         `json:"summary"`','extension summary field');

requireText(js,"KoscheiAuth.apiCall('/api/v1/token/extensions'",'dedicated premium Token-2022 request');
if(js.includes("apiCall('/api/token/scan'"))throw new Error('premium Token-2022 page must not call generic compatibility scan route');
requireText(js,"KoscheiAuth.requireAuth('/login.html')",'customer session requirement');
requireText(js,"data?.verdict_withheld===true||data?.extension_evidence_complete===false",'withheld evidence display gate');
requireText(js,"raw==='allow'&&(penalty===null||level!=='low')",'incomplete allow downgrade');
requireText(js,"penalty===null?'UNAVAILABLE':penalty",'missing heuristic penalty display');
requireText(js,"largest===null?'UNAVAILABLE'",'missing largest-holder display');
requireText(js,"topTen===null?'UNAVAILABLE'",'missing top-ten display');
requireText(js,"penalty===null?'UNAVAILABLE'",'missing extension penalty display');
requireText(js,'Do not interpret a failed or unavailable scan as a zero-risk token.','degraded fail-closed copy');
requireText(js,"value===null||value===undefined||String(value).trim()===''",'null numeric guard');

forbid(js,/\bfetch\s*\(/,'raw fetch bypassing KoscheiAuth');
forbid(js,/Authorization/i,'manual bearer header');
forbid(js,/\blocalStorage\b|\bsessionStorage\b/,'browser auth/access persistence');
forbid(js,/\.innerHTML\s*=/,'API-derived innerHTML');
forbid(js,/Math\.random\s*\(/,'synthetic extension evidence');
forbid(js,/(?:score|largest_holder_percent|top_ten_percent|extension_risk_penalty)\s*\|\|\s*0/,'missing evidence zero fallback');
forbid(js,/(?:largest_holder_percent|top_ten_percent|extension_risk_penalty)\s*\?\?\s*0/,'missing evidence zero fallback');
forbid(js,/\b(?:signMessage|signTransaction|signAllTransactions|signAndSendTransaction|sendTransaction)\b/,'wallet authority');

requireText(css,'.token2022-verdict.good','allow state styles');
requireText(css,'.token2022-verdict.warn','withhold/warn state styles');
requireText(css,'.token2022-verdict.bad','block state styles');
requireText(css,'.token2022-extension.high','high extension styles');
requireText(css,'@media(max-width:650px)','mobile Token-2022 layout');
console.log('Token-2022 scanner v2 + Professional SaaS contract: ok');
