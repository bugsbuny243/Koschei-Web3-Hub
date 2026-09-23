package http

import (
	"html/template"
	"net/http"
	"time"
)

type globalRadarOperatorView struct {
	Snapshot globalRadarSnapshot
}

var globalRadarOperatorTemplate = template.Must(template.New("global-radar").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Koschei Global Crypto Radar</title>
<style>
*{box-sizing:border-box}
:root{color-scheme:dark}
body{margin:0;background:#041013;color:#e8ffff;font:15px/1.5 system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}
a{color:#68fff0;text-decoration:none}a:hover{text-decoration:underline}
.shell{max-width:1320px;margin:auto;padding:26px}
.top{display:flex;justify-content:space-between;gap:18px;align-items:center;flex-wrap:wrap;border-bottom:1px solid #174147;padding-bottom:18px}
.brand{letter-spacing:.42em;font-size:13px;color:#b9d9d8}.nav{display:flex;gap:18px;flex-wrap:wrap}
.hero{padding:42px 0 26px;text-align:center}
.hero h1{font-size:clamp(2rem,6vw,4.8rem);line-height:.98;margin:8px 0 16px;letter-spacing:.02em}
.hero h1 span{color:#61fff2}.hero p{max-width:850px;margin:auto;color:#a9cecf;font-size:1.05rem}
.badges{display:flex;justify-content:center;gap:9px;flex-wrap:wrap;margin-top:22px}
.badge{border:1px solid #22585e;background:#071c20;border-radius:999px;padding:7px 12px;color:#b8f8f3}
.dashboard{display:grid;grid-template-columns:minmax(270px,.8fr) minmax(360px,1.8fr) minmax(270px,.8fr);gap:18px;align-items:stretch}
.panel{border:1px solid #174147;background:linear-gradient(180deg,#071b1f,#061317);border-radius:18px;padding:20px;box-shadow:0 18px 80px #00141666}
.panel h2{font-size:1rem;letter-spacing:.12em;text-transform:uppercase;color:#6ff8ed;margin:0 0 16px}
.metric{display:flex;justify-content:space-between;gap:12px;padding:11px 0;border-bottom:1px solid #12363b}.metric:last-child{border-bottom:0}
.metric strong{font-size:1.2rem}.muted{color:#86aeb0}
.radar{min-height:370px;position:relative;display:flex;align-items:center;justify-content:center;overflow:hidden;background:radial-gradient(circle at center,#0c4e4c55 0 2%,transparent 3% 100%),linear-gradient(180deg,#07191c,#041014)}
.rings{width:min(72vw,440px);aspect-ratio:1;border:1px solid #37cfc455;border-radius:50%;position:relative;box-shadow:inset 0 0 60px #00e6d511}
.rings:before,.rings:after{content:"";position:absolute;border:1px solid #37cfc444;border-radius:50%;inset:16%}
.rings:after{inset:33%}
.sweep{position:absolute;width:50%;height:1px;background:linear-gradient(90deg,#6ffff5,transparent);left:50%;top:50%;transform-origin:left center;transform:rotate(-24deg);box-shadow:0 0 16px #52fff2}
.core{position:absolute;text-align:center}.core strong{display:block;font-size:3rem;color:#7efff5}.core span{color:#8bc0c1}
.truth{display:grid;gap:10px}.truth div{padding:11px;border:1px solid #183f44;border-radius:10px;background:#061519}
.pipeline{margin-top:18px;display:grid;grid-template-columns:repeat(5,1fr);gap:9px}.step{border:1px solid #1c5157;border-radius:12px;padding:13px;text-align:center;background:#071a1e}.step b{display:block;color:#75fff4}.step small{color:#82aaac}
.table-wrap{margin-top:18px;overflow-x:auto}.network-table{width:100%;border-collapse:collapse;min-width:850px}.network-table th,.network-table td{text-align:left;padding:12px;border-bottom:1px solid #153b40}.network-table th{color:#72e9e1;font-size:.78rem;text-transform:uppercase;letter-spacing:.08em}.network-table td{color:#c7dddd}
.footer-note{margin-top:18px;color:#83a8aa;font-size:.9rem}
@media(max-width:900px){.dashboard{grid-template-columns:1fr}.radar{order:-1}.pipeline{grid-template-columns:1fr 1fr}}
</style>
</head>
<body>
<div class="shell">
<header class="top">
<div class="brand">K O S C H E I</div>
<nav class="nav"><a href="/fabric">Fabric</a><a href="/fabric/networks">Networks</a><a href="/fabric/radar/global">JSON contract</a><a href="/scan">Investigation</a></nav>
</header>
<section class="hero">
<div class="brand">OBSERVE · CONNECT · VERIFY</div>
<h1>KOSCHEI GLOBAL <span>CRYPTO RADAR</span></h1>
<p>A multi-network evidence control plane. Coverage, graph relationships, attack paths and ARVIS evidence are shown only when the underlying evidence contract supports them.</p>
<div class="badges"><span class="badge">No fabricated global totals</span><span class="badge">No wallet geolocation</span><span class="badge">Missing evidence = UNKNOWN</span><span class="badge">Hypotheses cannot mint ARVIS evidence</span></div>
</section>

<section class="dashboard">
<aside class="panel">
<h2>Coverage</h2>
<div class="metric"><span>Registered networks</span><strong>{{.Snapshot.Summary.RegisteredNetworks}}</strong></div>
<div class="metric"><span>Implementation-ready</span><strong>{{.Snapshot.Summary.ImplementationReady}}</strong></div>
<div class="metric"><span>Configuration required</span><strong>{{.Snapshot.Summary.ConfigurationRequired}}</strong></div>
<div class="metric"><span>Live checked</span><strong>{{.Snapshot.Summary.LiveChecked}}</strong></div>
<div class="metric"><span>Families</span><strong>{{range .Snapshot.Summary.NetworkFamilies}}{{.}} {{end}}</strong></div>
</aside>

<main class="panel radar" aria-label="Decorative radar visualization; not a geographic wallet map">
<div class="rings"><div class="sweep"></div></div>
<div class="core"><strong>{{.Snapshot.Summary.RegisteredNetworks}}</strong><span>registered network adapters</span></div>
</main>

<aside class="panel">
<h2>Truth boundary</h2>
<div class="truth">
<div><b>Live chain totals withheld</b><br><span class="muted">Aggregate transaction, wallet and contract counts stay absent until measured.</span></div>
<div><b>Infrastructure geography only</b><br><span class="muted">Node/validator geography needs provenance. Wallet/user location is not inferred.</span></div>
<div><b>Evidence ≠ hypothesis</b><br><span class="muted">Candidate correlations cannot silently become ARVIS evidence.</span></div>
<div><b>Decision authority stays separate</b><br><span class="muted">Radar transports evidence; ARVIS owns deterministic decision policy.</span></div>
</div>
</aside>
</section>

<section class="pipeline" aria-label="Evidence pipeline">
<div class="step"><b>Radar Event</b><small>{{.Snapshot.EventSchema}}</small></div>
<div class="step"><b>Entity Graph</b><small>{{.Snapshot.EntityGraphSchema}}</small></div>
<div class="step"><b>Attack Path</b><small>{{.Snapshot.AttackPathSchema}}</small></div>
<div class="step"><b>Security Evidence</b><small>{{.Snapshot.SecurityEvidenceSchema}}</small></div>
<div class="step"><b>ARVIS</b><small>deterministic verdict authority</small></div>
</section>

<section class="panel table-wrap">
<h2>Network implementation state</h2>
<table class="network-table">
<thead><tr><th>Network</th><th>Family</th><th>Consensus class</th><th>Collector</th><th>Runtime</th><th>Live availability</th><th>Node telemetry</th></tr></thead>
<tbody>
{{range .Snapshot.Networks}}<tr><td>{{.Network.Name}}</td><td>{{.Network.Family}}</td><td>{{.Network.ConsensusFamily}}</td><td>{{.Network.CollectorStatus}}</td><td>{{.CollectorRuntime}}</td><td>{{.LiveAvailability}}</td><td>{{.Network.NodeTelemetryStatus}}</td></tr>{{end}}
</tbody>
</table>
<p class="footer-note">Implementation readiness is not the same as live evidence. A network may have an adapter or probe in code while deployment configuration or current availability remains unchecked.</p>
</section>
</div>
</body>
</html>`))

func globalRadarOperatorPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	view := globalRadarOperatorView{Snapshot: currentGlobalRadarSnapshot(time.Now())}
	if err := globalRadarOperatorTemplate.Execute(w, view); err != nil {
		http.Error(w, "global radar surface unavailable", http.StatusInternalServerError)
	}
}
