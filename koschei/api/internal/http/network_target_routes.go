package http

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"sync/atomic"

	"koschei/api/internal/networktarget"
)

var networkTargetRequests atomic.Uint64
var networkTargetRejected atomic.Uint64

type networkTargetRequest struct {
	Network string `json:"network"`
	Address string `json:"address"`
}

func decodeNetworkTargetRequest(reader io.Reader) (networkTargetRequest, error) {
	var result networkTargetRequest
	decoder := json.NewDecoder(reader)
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return result, fmt.Errorf("invalid_target_request")
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] || (key != "network" && key != "address") {
			return result, fmt.Errorf("invalid_target_request")
		}
		seen[key] = true
		var value string
		if decoder.Decode(&value) != nil {
			return result, fmt.Errorf("invalid_target_request")
		}
		if key == "network" {
			result.Network = value
		} else {
			result.Address = value
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return result, fmt.Errorf("invalid_target_request")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return result, fmt.Errorf("invalid_target_request")
	}
	return result, nil
}

func registerNetworkTargetRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/fabric/networks", method(http.MethodGet, networkCoveragePage))
	mux.HandleFunc("/fabric/networks/catalog", method(http.MethodGet, networkCoverageCatalog))
	mux.HandleFunc("/fabric/networks/resolve", method(http.MethodPost, networkTargetResolve))
}

func networkCoverageCatalog(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"schema_version": networktarget.SchemaVersion,
		"scope_target": "all-blockchain-networks-and-address-types",
		"networks": networktarget.Catalog(),
		"live_availability": "not_checked",
		"telemetry": map[string]any{
			"scope": "process_local",
			"resolution_requests": networkTargetRequests.Load(),
			"rejected_requests": networkTargetRejected.Load(),
			"addresses_recorded": false,
		},
	})
}

type networkCoverageView struct {
	Networks   []networktarget.Network
	Resolution *networktarget.Resolution
	Error      string
}

func renderNetworkCoverage(w http.ResponseWriter, status int, result *networktarget.Resolution, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = networkCoverageTemplate.Execute(w, networkCoverageView{
		Networks: networktarget.Catalog(), Resolution: result, Error: message,
	})
}

func networkCoveragePage(w http.ResponseWriter, _ *http.Request) {
	renderNetworkCoverage(w, http.StatusOK, nil, "")
}

func networkTargetResolve(w http.ResponseWriter, r *http.Request) {
	networkTargetRequests.Add(1)
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	defer r.Body.Close()
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	isForm := mediaType == "application/x-www-form-urlencoded"
	reject := func(status int, message string) {
		networkTargetRejected.Add(1)
		if isForm {
			renderNetworkCoverage(w, status, nil, message)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": message, "analysis_performed": false, "evidence_status": "unknown",
		})
	}
	var request networkTargetRequest
	if isForm {
		if r.ParseForm() != nil || len(r.PostForm) != 2 || len(r.PostForm["network"]) != 1 || len(r.PostForm["address"]) != 1 {
			reject(http.StatusBadRequest, "invalid_target_request")
			return
		}
		request.Network = r.PostForm.Get("network")
		request.Address = r.PostForm.Get("address")
	} else {
		if mediaType != "application/json" {
			reject(http.StatusUnsupportedMediaType, "json_or_form_required")
			return
		}
		var err error
		request, err = decodeNetworkTargetRequest(r.Body)
		if err != nil {
			reject(http.StatusBadRequest, "invalid_target_request")
			return
		}
	}
	result, err := networktarget.Resolve(request.Network, request.Address)
	if err != nil {
		reject(http.StatusUnprocessableEntity, err.Error())
		return
	}
	if isForm {
		renderNetworkCoverage(w, http.StatusOK, &result, "")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(result)
}

var networkCoverageTemplate = template.Must(template.New("network-coverage").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Network coverage | Koschei Web3</title><style>
*{box-sizing:border-box}body{margin:auto;padding:24px;max-width:1040px;background:#080b12;color:#eef2ff;font:16px/1.6 system-ui,sans-serif}a{color:#67e8c2}h1{font-size:2rem;margin:16px 0}h2{font-size:1.3rem}.muted{color:#abb8d1}.panel{padding:24px;border:1px solid #344360;border-radius:16px;margin:20px 0;background:#101725}label{display:block;margin:12px 0}input,select,button{width:100%;min-height:48px;padding:10px;font:inherit;border:1px solid #63718b;border-radius:8px;background:#111b2d;color:#eef2ff}button{margin-top:16px;background:#67e8c2;color:#071713;font-weight:700;cursor:pointer}.table{overflow-x:auto}table{border-collapse:collapse;width:100%;text-align:left}th,td{padding:12px;border-bottom:1px solid #344360;font-size:1rem}.result{overflow-wrap:anywhere}.error{color:#ffb4ac}nav{display:flex;gap:20px;flex-wrap:wrap}
</style></head><body><nav><a href="/scan">Investigation console</a><a href="/fabric">Project status</a></nav>
<h1>Network coverage</h1><p>Koschei Web3 is expanding across blockchain networks and address types. Check the selected network before starting an investigation.</p>
<section class="panel"><h2>Check an address and network</h2><p class="muted">This checks address format and network identity. It does not collect on-chain evidence or produce a security verdict.</p>
<form method="post" action="/fabric/networks/resolve"><label>Network<select name="network" required><option value="">Select a network</option>{{range .Networks}}<option value="{{.ID}}">{{.Name}}</option>{{end}}</select></label><label>Address<input name="address" required maxlength="128" autocomplete="off" spellcheck="false"></label><button type="submit">Check network and address</button></form>
{{if .Error}}<p class="error" role="alert">Address could not be checked: {{.Error}}. No analysis was run.</p>{{end}}
{{with .Resolution}}<div class="result" role="status"><h2>{{.Network.Name}} address format recognized</h2><p>{{.CanonicalRef}}</p><p>Collector: {{.Network.CollectorStatus}}. Live availability has not been checked. No analysis has been performed; evidence remains unknown.</p>{{if eq .Network.Family "evm"}}<p>Hex format was checked. Address checksum, account existence, account type and ownership were not verified.</p>{{end}}</div>{{end}}</section>
<section class="panel table"><h2>Implementation coverage</h2><table><thead><tr><th>Network</th><th>Address format</th><th>Collector</th></tr></thead><tbody>{{range .Networks}}<tr><td>{{.Name}}</td><td>{{.AddressFormat}}</td><td>{{if eq .CollectorStatus "existing"}}Existing collector; live health not checked{{else}}Not connected{{end}}</td></tr>{{end}}</tbody></table><p class="muted">A shared address format does not prove an account exists on another network. Unconnected networks never fall back to the Solana collector. Bitcoin address parsing and evidence collection remain pending.</p></section>
</body></html>`))
