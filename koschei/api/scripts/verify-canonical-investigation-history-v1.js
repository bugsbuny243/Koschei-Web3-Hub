'use strict';
const fs=require('node:fs');
const path=require('node:path');
const root=path.resolve(__dirname,'..');
const migration=fs.readFileSync(path.join(root,'migrations','021_web3_async_jobs.sql'),'utf8');
const jobTypes=fs.readFileSync(path.join(root,'internal','jobs','types.go'),'utf8');
const store=fs.readFileSync(path.join(root,'internal','jobs','history.go'),'utf8');
const handler=fs.readFileSync(path.join(root,'internal','handlers','customer_investigation_history.go'),'utf8');
const jobsHandler=fs.readFileSync(path.join(root,'internal','handlers','web3_jobs.go'),'utf8');
const server=fs.readFileSync(path.join(root,'internal','http','server.go'),'utf8');
const inventory=fs.readFileSync(path.join(root,'internal','http','route_inventory.go'),'utf8');
const aliases=fs.readFileSync(path.join(root,'internal','http','static_aliases.go'),'utf8');
const dashboard=fs.readFileSync(path.join(root,'public','dashboard.html'),'utf8');
const workspaceJS=fs.readFileSync(path.join(root,'public','js','customer-workspace-v2.js'),'utf8');

function requireText(source,needle,label){if(!source.includes(needle))throw new Error(`${label}: missing ${needle}`);}
function forbid(source,pattern,label){if(pattern.test(source))throw new Error(`${label}: forbidden pattern ${pattern}`);}
function requireAbsent(relative,label){if(fs.existsSync(path.join(root,relative)))throw new Error(`${label}: retired file still exists: ${relative}`);}

requireText(migration,'CREATE TABLE IF NOT EXISTS web3_jobs','durable job table');
requireText(migration,'result_payload JSONB','durable result payload');
requireText(migration,'CREATE INDEX IF NOT EXISTS web3_jobs_user_created_idx ON web3_jobs (user_id, queued_at DESC);','account history index');
for(const state of ['StatusQueued    = "queued"','StatusRunning   = "running"','StatusCompleted = "completed"','StatusFailed    = "failed"'])requireText(jobTypes,state,`durable job state ${state}`);
requireText(store,'func (s *Store) ListByUser(ctx context.Context, userID, jobType string, limit int) ([]Job, error)','account-scoped history method');
requireText(store,'if userID == ""','required account scope');
requireText(store,'if limit <= 0 {','default history limit');
requireText(store,'if limit > MaxHistoryLimit {','maximum history limit');
requireText(store,"WHERE user_id=$1 AND ($2='' OR job_type=$2)",'user and job-type query scope');
requireText(store,'ORDER BY queued_at DESC,id DESC','canonical history ordering');
requireText(store,'scanJob(rows)','shared job scanner contract');

requireText(handler,'func (h *Handler) CustomerInvestigationHistory','customer history handler');
requireText(handler,'h.RequirePlanTier("professional", h.customerInvestigationHistoryRead)(w, r)','Professional history gate');
requireText(handler,'h.JobStore.ListByUser(r.Context(), claims.Sub, CanonicalInvestigationJobType, 100)','canonical account history query');
requireText(handler,'ResultAvailable bool','result availability evidence');
requireText(handler,'if json.Unmarshal(job.ResultPayload, &result) == nil && result != nil','result payload parse boundary');
requireText(handler,'"schema_version": "koschei-customer-investigation-history-v1"','versioned history envelope');
requireText(handler,'"source": "web3_jobs"','durable source marker');
requireText(handler,'"job_type": CanonicalInvestigationJobType','canonical job-type marker');
requireText(handler,'"history": items','history collection envelope');
forbid(handler,/RequireTokenTier|KOSCH access|EnforceScanQuota|RequirePlanTier\("starter"/,'history handler must use Professional entitlement without token or legacy plan coupling');

requireText(jobsHandler,'if id == "" {','empty job-id dispatch boundary');
requireText(jobsHandler,'if canonicalHistoryCollectionPath(r.URL.Path) {','radar jobs collection isolation');
requireText(jobsHandler,'h.CustomerInvestigationHistory(w, r)','history collection delegation');
requireText(jobsHandler,'func canonicalHistoryCollectionPath(path string) bool','collection path predicate');
requireText(jobsHandler,'return strings.TrimSuffix(strings.TrimSpace(path), "/") == "/api/v1/radar/jobs"','canonical-only collection predicate');
requireText(jobsHandler,'writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})','legacy empty collection remains not found');
requireText(server,'/api/v1/radar/jobs/','existing radar jobs GET route');
if(server.includes('/api/v1/investigations/history'))throw new Error('server: do not add a parallel history endpoint; use the existing radar jobs collection');
requireText(inventory,'"GET /api/v1/radar/jobs/"','machine-readable radar jobs GET route');
if(inventory.includes('/api/v1/investigations/history'))throw new Error('inventory: parallel history endpoint must not be advertised');

// The durable backend contract remains preserved for a future persistence plane,
// but current production is intentionally stateless. Customer history therefore
// belongs to the Customer Panel truth surface and must not survive as a separate
// control that can only return dependency-unavailable responses.
requireAbsent('public/reports.html','standalone reports surface');
requireAbsent('public/js/customer-reports-v2.js','standalone reports runtime');
for(const route of ['/reports', '/reports/', '/reports.html'])requireText(aliases,`"${route}"`,'reports compatibility redirect');
requireText(aliases,'registerCanonicalRedirect(mux, route, "/dashboard#evidence")','reports dashboard evidence redirect');

requireText(dashboard,'Durable history','Workspace durable-history capability label');
requireText(dashboard,'PERSISTENCE OFF','Workspace persistence boundary');
requireText(dashboard,'id="workspaceLatestReport"','Workspace persistence-truth mount');
requireText(dashboard,'/js/customer-workspace-v2.js?v=3','Workspace stateless controller');
if(dashboard.includes('Signed Report Vault'))throw new Error('Workspace must not advertise every durable job as a signed report');
requireText(workspaceJS,"read('/api/me')",'Workspace stateless identity source');
requireText(workspaceJS,'renderPersistenceBoundary','Workspace explicit persistence boundary');
requireText(workspaceJS,"setKPI('workspaceReportsKpi','NOT LIVE'",'Workspace history non-live state');
for(const forbiddenRoute of ['/api/v1/radar/jobs/','/api/v1/investigations/history','/api/v1/unified/reports']){
  if(workspaceJS.includes(forbiddenRoute))throw new Error(`Workspace must not call persistence-backed history route while stateless: ${forbiddenRoute}`);
}
forbid(workspaceJS,/Math\.random\s*\(/,'Workspace synthetic history evidence');
console.log('canonical investigation history backend + retired standalone surface + stateless workspace contract: ok');
