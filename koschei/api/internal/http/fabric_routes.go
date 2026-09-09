package http

import (
	"encoding/json"
	"html/template"
	"net/http"
)

type fabricCapability struct {
	ID        string `json:"id"`
	Domain    string `json:"domain"`
	Status    string `json:"status"`
	Backend   string `json:"backend"`
	Frontend  string `json:"frontend"`
	Telemetry string `json:"telemetry"`
}

type fabricComponent struct {
	Name         string             `json:"name"`
	Repository   string             `json:"repository"`
	Role         string             `json:"role"`
	Mode         string             `json:"mode"`
	Capabilities []fabricCapability `json:"capabilities"`
}

type fabricSnapshot struct {
	SchemaVersion          string            `json:"schemaVersion"`
	Workspace              string            `json:"workspace"`
	IntegrationMode        string            `json:"integrationMode"`
	PreserveExisting       bool              `json:"preserveExisting"`
	BreakingChangesAllowed bool              `json:"breakingChangesAllowed"`
	BackendFrontendParity  bool              `json:"backendFrontendParity"`
	Components             []fabricComponent `json:"components"`
}

func currentFabricSnapshot() fabricSnapshot {
	return fabricSnapshot{
		SchemaVersion:          "1.0",
		Workspace:              "koschei-unified",
		IntegrationMode:        "federated",
		PreserveExisting:       true,
		BreakingChangesAllowed: false,
		BackendFrontendParity:  true,
		Components: []fabricComponent{
			{Name: "koschei-web3", Repository: "bugsbuny243/Koschei-Web3-Hub", Role: "security-validation-risk-intelligence", Mode: "active", Capabilities: []fabricCapability{
				{ID: "web3-security-core", Domain: "web3", Status: "stable", Backend: "existing", Frontend: "existing", Telemetry: "existing"},
				{ID: "fabric-capability-registry", Domain: "core", Status: "experimental", Backend: "existing", Frontend: "existing", Telemetry: "planned"},
			}},
			{Name: "koschei-lang", Repository: "bugsbuny243/koschei-lang", Role: "programmable-policy-agent-language", Mode: "observe", Capabilities: []fabricCapability{
				{ID: "language-toolchain", Domain: "core", Status: "stable", Backend: "existing", Frontend: "existing", Telemetry: "existing"},
				{ID: "web4-web6-policy-and-agent-profile", Domain: "web4", Status: "experimental", Backend: "adapter", Frontend: "planned", Telemetry: "planned"},
			}},
			{Name: "koschei-sentinel", Repository: "bugsbuny243/koschei-sentinel", Role: "observation-detection-response-and-model-intelligence", Mode: "observe", Capabilities: []fabricCapability{
				{ID: "sentinel-evaluation-and-provenance", Domain: "core", Status: "stable", Backend: "existing", Frontend: "planned", Telemetry: "existing"},
				{ID: "fabric-observation-plane", Domain: "web4", Status: "experimental", Backend: "adapter", Frontend: "planned", Telemetry: "planned"},
			}},
		},
	}
}

func registerFabricRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v2/fabric/capabilities", method(http.MethodGet, fabricCapabilities))
	mux.HandleFunc("/fabric", method(http.MethodGet, fabricOperatorSurface))
}

// MountFabric adds the new federated control-plane routes without modifying the
// existing NewServer route graph. All non-Fabric requests are delegated to base
// unchanged. Fabric responses still pass through the repository's existing
// security-header and CSP transformation machinery.
func MountFabric(base http.Handler) http.Handler {
	if base == nil {
		base = http.NotFoundHandler()
	}
	fabricMux := http.NewServeMux()
	registerFabricRoutes(fabricMux)
	fabric := securityHeaders(fabricMux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fabric", "/api/v2/fabric/capabilities":
			fabric.ServeHTTP(w, r)
		default:
			base.ServeHTTP(w, r)
		}
	})
}

func fabricCapabilities(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(currentFabricSnapshot())
}

var fabricPage = template.Must(template.New("fabric").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Koschei Fabric</title><style>
body{font-family:system-ui,sans-serif;background:#080b12;color:#eef2ff;margin:0;padding:32px;max-width:1180px;margin-inline:auto}h1{font-size:3rem;margin:.2em 0}.muted{color:#a5afc8}.rules{display:flex;gap:10px;flex-wrap:wrap;margin:24px 0}.pill,.card{border:1px solid #2a3554;background:#101725}.pill{border-radius:999px;padding:7px 11px}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(260px,1fr));gap:16px}.card{border-radius:18px;padding:18px}.cap{border-top:1px solid #2a3554;margin-top:12px;padding-top:12px}.cap strong{display:block}
</style></head><body>
<p class="muted">KOSCHEI UNIFIED CONTROL PLANE · EXPERIMENTAL</p><h1>Koschei Fabric</h1>
<p class="muted">Koschei Web3, Koschei Lang and Koschei Sentinel are joined through explicit contracts. Existing production behavior stays intact; new cross-project capabilities start in observe/adapter mode.</p>
<div class="rules"><span class="pill">workspace: {{.Workspace}}</span><span class="pill">mode: {{.IntegrationMode}}</span><span class="pill">preserve existing: {{.PreserveExisting}}</span><span class="pill">backend ↔ frontend parity: {{.BackendFrontendParity}}</span></div>
<div class="grid">{{range .Components}}<section class="card"><p class="muted">{{.Mode}}</p><h2>{{.Name}}</h2><p class="muted">{{.Role}}</p>{{range .Capabilities}}<div class="cap"><strong>{{.ID}}</strong><span class="muted">{{.Domain}} · {{.Status}} · backend {{.Backend}} · frontend {{.Frontend}} · telemetry {{.Telemetry}}</span></div>{{end}}</section>{{end}}</div>
</body></html>`))

func fabricOperatorSurface(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := fabricPage.Execute(w, currentFabricSnapshot()); err != nil {
		http.Error(w, "fabric surface unavailable", http.StatusInternalServerError)
	}
}
