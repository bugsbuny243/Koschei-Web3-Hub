package http

import (
	"encoding/json"
	"html/template"
	"net/http"

	"koschei/api/internal/networktarget"
)

type networkProbeView struct {
	States []networkDeploymentState
	Result *networktarget.EVMProbeResult
	Error  string
}

func networkDeploymentCatalogHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"schema_version": networktarget.SchemaVersion,
		"networks":       networkDeploymentCatalog(),
	})
}

func networkProbePage(w http.ResponseWriter, _ *http.Request) {
	renderNetworkProbeResult(w, http.StatusOK, nil, "")
}

func renderNetworkProbeResult(w http.ResponseWriter, status int, result *networktarget.EVMProbeResult, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = networkProbeTemplate.Execute(w, networkProbeView{States: networkDeploymentCatalog(), Result: result, Error: message})
}

var networkProbeTemplate = template.Must(template.New("network-live-probe").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Live EVM evidence | Koschei Web3</title><style>
*{box-sizing:border-box}body{margin:auto;padding:24px;max-width:1040px;background:#080b12;color:#eef2ff;font:16px/1.6 system-ui,sans-serif}a{color:#67e8c2}h1{font-size:2rem}.muted{color:#abb8d1}.panel{padding:24px;border:1px solid #344360;border-radius:16px;margin:20px 0;background:#101725}label{display:block;margin:12px 0}input,select,button{width:100%;min-height:48px;padding:10px;font:inherit;border:1px solid #63718b;border-radius:8px;background:#111b2d;color:#eef2ff}button{margin-top:16px;background:#67e8c2;color:#071713;font-weight:700;cursor:pointer}table{border-collapse:collapse;width:100%}th,td{padding:10px;border-bottom:1px solid #344360;text-align:left}.error{color:#ffb4ac}.ok{color:#67e8c2;overflow-wrap:anywhere}nav{display:flex;gap:18px;flex-wrap:wrap}
</style></head><body><nav><a href="/scan">Investigation console</a><a href="/fabric/networks">Network coverage</a><a href="/fabric">Project status</a></nav>
<h1>Live EVM evidence</h1><p class="muted">This is a narrow read-only collector check. Koschei verifies the configured RPC chain ID before observing contract bytecode. No transaction is sent and no safety verdict is produced.</p>
<section class="panel"><h2>Collector deployment</h2><table><thead><tr><th>Network</th><th>Runtime</th><th>Live availability</th></tr></thead><tbody>{{range .States}}<tr><td>{{.NetworkID}}</td><td>{{.CollectorRuntime}}</td><td>{{.LiveAvailability}}</td></tr>{{end}}</tbody></table></section>
<section class="panel"><h2>Probe EVM address</h2><form method="post" action="/fabric/networks/probe"><label>Network<select name="network" required><option value="">Select EVM network</option><option value="ethereum-mainnet">Ethereum</option><option value="base-mainnet">Base</option><option value="arbitrum-mainnet">Arbitrum</option><option value="optimism-mainnet">Optimism</option></select></label><label>Address<input name="address" required maxlength="42" autocomplete="off" spellcheck="false" placeholder="0x..."></label><button type="submit">Run live read-only probe</button></form>
{{if .Error}}<p class="error">Probe unavailable: {{.Error}}. No evidence was accepted.</p>{{end}}
{{with .Result}}<div class="ok"><h2>{{.Resolution.Network.Name}} live evidence observed</h2><p>Canonical subject: {{.Resolution.CanonicalRef}}</p><p>Chain ID: {{.ChainID}} (expected {{.ExpectedChainID}})</p><p>Contract code: {{.ContractCodeState}}</p>{{if .ContractCodeHash}}<p>Observed code SHA-256: {{.ContractCodeHash}}</p>{{end}}<p>Evidence status: {{.EvidenceStatus}} · live availability: {{.LiveAvailability}}</p><p class="muted">No contract code does not prove the account exists, belongs to a person, or is safe.</p></div>{{end}}
</section></body></html>`))
