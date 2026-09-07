'use strict';
const fs=require('node:fs');
const path=require('node:path');
const root=path.resolve(__dirname,'..');
const routes=fs.readFileSync(path.join(root,'internal','http','watchlist_routes.go'),'utf8');
const server=fs.readFileSync(path.join(root,'internal','http','server.go'),'utf8');
const aliases=fs.readFileSync(path.join(root,'internal','http','static_aliases.go'),'utf8');
const handler=fs.readFileSync(path.join(root,'internal','handlers','watchlist.go'),'utf8');
const monitor=fs.readFileSync(path.join(root,'internal','handlers','watchlist_monitor.go'),'utf8');
const migration=fs.readFileSync(path.join(root,'migrations','046_watchlist_alerts.sql'),'utf8');
const docs=fs.readFileSync(path.resolve(root,'..','..','docs','watchlist-alerts.md'),'utf8');
const dashboard=fs.readFileSync(path.join(root,'public','dashboard.html'),'utf8');
const workspaceJS=fs.readFileSync(path.join(root,'public','js','customer-workspace-v2.js'),'utf8');

function requireText(source,needle,label){if(!source.includes(needle))throw new Error(`${label}: missing ${needle}`);}
function forbid(source,pattern,label){if(pattern.test(source))throw new Error(`${label}: forbidden pattern ${pattern}`);}
function requireAbsent(relative,label){if(fs.existsSync(path.join(root,relative)))throw new Error(`${label}: retired file still exists: ${relative}`);}

requireText(routes,'func registerWatchlistRoutes','watchlist route registration function');
for(const route of ['"/api/watchlist", requiresDB(h, proMetered(','"/api/watchlist/refresh", requiresDB(h, proMetered(','"/api/watchlist/alerts", requiresDB(h, proMetered(','"/api/watchlist/", requiresDB(h, proMetered('])requireText(routes,route,`metered watchlist route ${route}`);
requireText(server,'return planTier("professional", next)','Professional watchlist server gate');
requireText(server,'return planTierAccess("professional", next)','Professional watchlist access gate');
requireText(server,'registerWatchlistRoutes(mux, h','watchlist route registration');
forbid(server,/koschTier\(|RequireTokenTier/,'legacy token watchlist authorization');

requireText(handler,'watchlistDefaultThreshold = 50','server threshold default');
requireText(handler,'watchlistMaxTargets       = 100','server target limit');
requireText(handler,'watchlistRefreshBatchMax  = 10','server batch refresh clamp');
requireText(handler,'"max_targets": watchlistMaxTargets','server-owned target capacity response');
requireText(handler,'if req.AlertThreshold == 0 {','zero-as-server-default contract');
requireText(handler,'req.AlertThreshold = watchlistDefaultThreshold','server threshold normalization');
requireText(handler,'if limit > watchlistRefreshBatchMax {','batch refresh clamp');
requireText(handler,'Status     string `json:"status"`','batch result status evidence');
requireText(handler,'"marked_read": count','authoritative reviewed count');

requireText(migration,"watchlist_targets_status_check CHECK (status IN ('active','paused'))",'target status enum');
requireText(migration,"watchlist_alerts_severity_check CHECK (severity IN ('info','low','medium','high','critical'))",'alert severity enum');
requireText(migration,"watchlist_alerts_status_check CHECK (status IN ('new','read'))",'alert review-state enum');

requireText(monitor,'services.AutomaticBackgroundScanningEnabled()','automatic background scanning gate');
requireText(monitor,'os.Getenv("WATCHLIST_MONITOR_ENABLED")','watchlist worker enable gate');
requireText(monitor,"WHERE status='active' AND COALESCE(next_check_at,now())<=now()",'due active target claim');
requireText(docs,'active **Professional SaaS plan**','documented Professional gate');
requireText(docs,'paid output-capacity enforcement remains server-owned','documented watchlist metering');
requireText(docs,'WATCHLIST_MONITOR_ENABLED','documented monitor enable gate');
requireText(docs,'**both** automatic background scanning and the watchlist monitor are explicitly enabled','documented dual background gate');
requireText(docs,'KOSCH holder balances and removed package labels do not grant or upgrade watchlist access.','documented KOSCH separation');

// Backend monitoring stays intact for a future real persistence plane, while the
// current stateless production process must not expose a standalone UI that can
// only fail its requiresDB gate.
requireAbsent('public/watchlist.html','standalone watchlist surface');
requireAbsent('public/js/customer-watchlist-v2.js','standalone watchlist runtime');
for(const route of ['/watchlist','/watchlist/','/watchlist.html'])requireText(aliases,`"${route}"`,'watchlist compatibility redirect');
requireText(aliases,'registerCanonicalRedirect(mux, route, "/dashboard#evidence")','watchlist dashboard evidence redirect');
requireText(dashboard,'Continuous monitoring','Dashboard monitoring capability label');
requireText(dashboard,'CONTINUOUS MONITORING ALERTS','Dashboard monitoring truth section');
requireText(dashboard,'PERSISTENCE OFF','Dashboard persistence boundary');
requireText(workspaceJS,"setKPI('workspaceWatchKpi','NOT LIVE'",'Dashboard watchlist non-live state');
requireText(workspaceJS,"setKPI('workspaceAlertsKpi','NOT LIVE'",'Dashboard alerts non-live state');
for(const forbiddenRoute of ['/api/watchlist','/api/watchlist/alerts','/api/watchlist/refresh']){
  if(workspaceJS.includes(forbiddenRoute))throw new Error(`Workspace must not call persistence-backed monitoring route while stateless: ${forbiddenRoute}`);
}
forbid(workspaceJS,/Math\.random\s*\(/,'Workspace synthetic monitoring evidence');

console.log('Watchlist evidence-state v2 backend + retired standalone surface + stateless workspace contract: ok');
