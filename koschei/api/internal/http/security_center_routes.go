package http

import (
	"encoding/json"
	"html/template"
	"net/http"

	"koschei/api/internal/runtimehealth"
	"koschei/api/internal/securitycenter"
)

func registerSecurityCenterRoutes(mux *http.ServeMux, registries ...*runtimehealth.Registry) {
	var registry *runtimehealth.Registry
	if len(registries) > 0 {
		registry = registries[0]
	}
	mux.HandleFunc("/fabric/security-center/capabilities", method(http.MethodGet, securityCenterCapabilities))
	mux.HandleFunc("/fabric/security-center/runtime-health", method(http.MethodGet, securityCenterRuntimeHealth(registry)))
	mux.HandleFunc("/fabric/security-center", method(http.MethodGet, securityCenterSurface))
}

func securityCenterRuntimeHealth(registry *runtimehealth.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(registry.Snapshot())
	}
}

func securityCenterCapabilities(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(securitycenter.Current())
}

var securityCenterPage = template.Must(template.New("security-center").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex,nofollow"><title>Koschei Global Crypto Security Center</title>
<style>
:root{color-scheme:dark;--bg:#03070b;--panel:#09131c;--line:#1b3540;--text:#e9fff8;--muted:#87a7a1;--green:#67f5bd;--cyan:#60ddff;--amber:#ffd36a}*{box-sizing:border-box}body{margin:0;background:radial-gradient(circle at 50% -10%,#0a2a2b 0,#03070b 42%);color:var(--text);font-family:Inter,system-ui,sans-serif}.wrap{width:min(1500px,calc(100% - 28px));margin:auto;padding:22px 0 70px}a{color:var(--cyan);text-decoration:none}.top{display:flex;justify-content:space-between;gap:16px;align-items:center;padding:12px 14px;border:1px solid var(--line);border-radius:16px;background:#071016dd;position:sticky;top:8px;z-index:10;backdrop-filter:blur(16px)}.brand{font-weight:950}.nav{display:flex;gap:12px;flex-wrap:wrap}.hero{display:grid;grid-template-columns:minmax(0,1.05fr) minmax(420px,.95fr);gap:18px;margin-top:18px}.card{border:1px solid var(--line);border-radius:22px;background:linear-gradient(145deg,#0a1721e8,#050b11ee);padding:20px}.eyebrow{color:var(--green);font:850 11px ui-monospace,monospace;letter-spacing:.1em}.hero h1{font-size:clamp(46px,7vw,84px);line-height:.9;letter-spacing:-.065em;margin:18px 0}.hero p{max-width:850px;color:#b7cec8;line-height:1.7}.truth{display:flex;gap:8px;flex-wrap:wrap;margin-top:18px}.truth span,.tag{padding:7px 9px;border:1px solid #67f5bd2d;border-radius:999px;color:#bfffe5;background:#67f5bd0a;font:800 10px ui-monospace,monospace}.world{min-height:440px;padding:0;overflow:hidden;position:relative}.world canvas{width:100%;height:440px;display:block}.world-note{position:absolute;left:18px;bottom:16px;right:18px;padding:11px 13px;border:1px solid #ffffff13;border-radius:14px;background:#02070bd9;color:#a8c1ba;font-size:12px}.section{margin-top:18px}.section h2{margin:0 0 7px;font-size:28px}.section>p{color:var(--muted);margin:0 0 14px}.grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}.cap h3{margin:8px 0;font-size:18px}.meta{display:grid;gap:6px;color:var(--muted);font-size:12px}.meta b{color:#dff9f1}.surfaces{display:flex;flex-wrap:wrap;gap:5px;margin-top:12px}.surface{font:750 9px ui-monospace,monospace;color:#a5e9ff;border:1px solid #60ddff24;border-radius:8px;padding:5px 7px;background:#60ddff08}.tables{display:grid;grid-template-columns:1fr 1fr;gap:14px}.table-wrap{overflow:auto;border:1px solid var(--line);border-radius:18px}.table{width:100%;border-collapse:collapse;min-width:680px}.table th,.table td{padding:10px 12px;border-bottom:1px solid #17303a;text-align:left;vertical-align:top;font-size:12px}.table th{color:#83a6a0;font:800 10px ui-monospace,monospace}.state{font:900 10px ui-monospace,monospace}.active{color:var(--green)}.preserved_registered,.preserved_disabled{color:var(--amber)}.footer{margin-top:22px;color:#6f8a84;font:750 10px ui-monospace,monospace;line-height:1.7}@media(max-width:1100px){.hero,.tables{grid-template-columns:1fr}.grid{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:720px){.wrap{width:min(100% - 14px,1500px)}.top{align-items:flex-start;flex-direction:column}.grid{grid-template-columns:1fr}.hero h1{font-size:48px}.world{min-height:330px}.world canvas{height:330px}}
</style></head><body><main class="wrap">
<nav class="top"><div class="brand"><span data-cipher="KOSCHEI GLOBAL CRYPTO SECURITY CENTER">KOSCHEI GLOBAL CRYPTO SECURITY CENTER</span></div><div class="nav"><a href="/fabric">Fabric</a><a href="/fabric/networks">Networks</a><a href="/scan">Intelligence Desk</a><a href="/dashboard">Console</a></div></nav>
<section class="hero"><article class="card"><span class="eyebrow">FEDERATED SECURITY CONTROL PLANE · ADDITIVE</span><h1>One evidence graph. Many security surfaces.</h1><p>{{.Purpose}}</p><div class="truth"><span>preserve existing: {{.PreserveExisting}}</span><span>breaking changes: {{.BreakingChanges}}</span><span>{{len .Networks}} registered networks</span><span>{{len .Capabilities}} security capabilities</span></div><p style="margin-top:18px"><b>Decision boundary:</b> {{.DecisionAuthority}}</p><p style="margin-top:10px"><b>Evidence rule:</b> {{.EvidenceRule}}</p></article><article class="card world"><canvas data-koschei-security-world aria-label="Koschei security network flow visualization"></canvas><div class="world-note">Visualization only. Runtime availability and evidence maturity are reported separately; this canvas never represents live telemetry.</div></article></section>
<section class="section"><h2>Security capability graph</h2><p>Every capability keeps its own authority boundary, activation gate and operational surface.</p><div class="grid">{{range .Capabilities}}<article class="card cap"><span class="eyebrow">{{.Layer}} · {{.Domain}}</span><h3>{{.ID}}</h3><div class="meta"><span>mode: <b>{{.Mode}}</b></span><span>authority: <b>{{.EvidenceAuthority}}</b></span><span>activation: <b>{{.Activation}}</b></span>{{if .NetworkIDs}}<span>networks: <b>{{len .NetworkIDs}}</b></span>{{end}}</div><div class="surfaces">{{range .BackendSurfaces}}<span class="surface">{{.}}</span>{{end}}{{range .FrontendSurfaces}}<span class="surface">{{.}}</span>{{end}}</div></article>{{end}}</div></section>
<section class="section tables"><article><h2>Registered networks</h2><p>Catalog presence is not a live-health claim.</p><div class="table-wrap"><table class="table"><thead><tr><th>Network</th><th>Family</th><th>Collector</th><th>Node telemetry</th></tr></thead><tbody>{{range .Networks}}<tr><td><b>{{.Name}}</b><br><span class="eyebrow">{{.ID}}</span></td><td>{{.Family}}<br>{{.ConsensusFamily}}</td><td>{{.CollectorStatus}}</td><td>{{.NodeTelemetryStatus}}</td></tr>{{end}}</tbody></table></div></article><article><h2>Preserved runtime assets</h2><p>No registered module is deleted merely because it is not currently co-executed.</p><div class="table-wrap"><table class="table"><thead><tr><th>Asset</th><th>State</th><th>Surface / reason</th></tr></thead><tbody>{{range .AssetBindings}}<tr><td><code>{{.Asset}}</code><br><span class="eyebrow">{{.Kind}}</span></td><td class="state {{.State}}">{{.State}}</td><td><b>{{.Surface}}</b><br>{{.Reason}}</td></tr>{{end}}</tbody></table></div></article></section>
<div class="footer">Schema {{.SchemaVersion}} · Repository capability metadata, not deployment health · Unknown stays unknown · No private-key custody.</div>
</main><script src="/js/cipher.js?v=2"></script><script src="/js/koschei-security-world.js?v=2"></script><script src="/js/feedback-button.js?v=2"></script></body></html>`))

func securityCenterSurface(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := securityCenterPage.Execute(w, securitycenter.Current()); err != nil {
		http.Error(w, "security center unavailable", http.StatusInternalServerError)
	}
}
