package http

import (
	"encoding/json"
	"html/template"
	"net/http"
)

type fabricCapability struct {
	ID            string   `json:"id"`
	Domain        string   `json:"domain"`
	Status        string   `json:"status"`
	EvidenceState string   `json:"evidenceState"`
	WorkPackages  []string `json:"workPackages,omitempty"`
	Backend       string   `json:"backend"`
	Frontend      string   `json:"frontend"`
	Telemetry     string   `json:"telemetry"`
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
	SecurityPackage        string            `json:"securityPackage"`
	AcceptanceCaseCount    int               `json:"acceptanceCaseCount"`
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
		SecurityPackage:        "Koschei-Web3-Web6-Guvenlik-Calisma-Paketi-2026-09-08.md",
		AcceptanceCaseCount:    14,
		Components: []fabricComponent{
			{Name: "koschei-web3", Repository: "bugsbuny243/Koschei-Web3-Hub", Role: "security-validation-risk-intelligence", Mode: "active", Capabilities: []fabricCapability{
				{ID: "web3-security-core", Domain: "web3", Status: "stable", EvidenceState: "blocked", WorkPackages: []string{"CORE-01", "CORE-02", "CORE-03", "CORE-04", "SIGN-01"}, Backend: "existing", Frontend: "existing", Telemetry: "existing"},
				{ID: "fabric-capability-registry", Domain: "core", Status: "experimental", EvidenceState: "partial", WorkPackages: []string{"CORE-04", "OPS-01"}, Backend: "existing", Frontend: "existing", Telemetry: "planned"},
			}},
			{Name: "koschei-lang", Repository: "bugsbuny243/koschei-lang", Role: "programmable-policy-agent-language", Mode: "observe", Capabilities: []fabricCapability{
				{ID: "language-toolchain", Domain: "core", Status: "experimental", EvidenceState: "blocked", WorkPackages: []string{"LANG-01", "LANG-02", "LANG-03", "LANG-04", "SUPPLY-02"}, Backend: "existing", Frontend: "existing", Telemetry: "existing"},
				{ID: "web4-policy-agent-profile", Domain: "web4", Status: "experimental", EvidenceState: "research", WorkPackages: []string{"AGENT-02", "AGENT-04", "LANG-03", "LANG-04"}, Backend: "adapter", Frontend: "planned", Telemetry: "planned"},
				{ID: "web5-identity-data-profile", Domain: "web5", Status: "planned", EvidenceState: "unverified", WorkPackages: []string{"ID-01", "ID-02", "ID-03", "ID-04", "DATA-04"}, Backend: "planned", Frontend: "planned", Telemetry: "planned"},
			}},
			{Name: "koschei-sentinel", Repository: "bugsbuny243/koschei-sentinel", Role: "observation-detection-response-and-model-intelligence", Mode: "observe", Capabilities: []fabricCapability{
				{ID: "sentinel-evaluation-and-provenance", Domain: "core", Status: "experimental", EvidenceState: "partial", WorkPackages: []string{"MODEL-01", "MODEL-03", "MODEL-04", "OPS-04"}, Backend: "existing", Frontend: "planned", Telemetry: "existing"},
				{ID: "fabric-observation-plane", Domain: "web4", Status: "experimental", EvidenceState: "research", WorkPackages: []string{"AGENT-01", "MODEL-02", "OPS-01", "OPS-04"}, Backend: "adapter", Frontend: "planned", Telemetry: "planned"},
				{ID: "pq-network-intelligence", Domain: "web6", Status: "experimental", EvidenceState: "research", WorkPackages: []string{"PQ-01", "PQ-02", "PQ-03", "PQ-04"}, Backend: "existing", Frontend: "planned", Telemetry: "existing"},
			}},
		},
	}
}

func registerFabricRoutes(mux *http.ServeMux) {
	// Fabric is still experimental. Keep its capability contract outside /api/*
	// until it is deliberately promoted into the production OpenAPI contract.
	mux.HandleFunc("/fabric/capabilities", method(http.MethodGet, fabricCapabilities))
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
		case "/fabric", "/fabric/capabilities":
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
body{font-family:system-ui,sans-serif;background:#080b12;color:#eef2ff;margin:0;padding:32px;max-width:1180px;margin-inline:auto}h1{font-size:3rem;margin:.2em 0}.muted{color:#a5afc8}.rules{display:flex;gap:10px;flex-wrap:wrap;margin:24px 0}.pill,.card{border:1px solid #2a3554;background:#101725}.pill{border-radius:999px;padding:7px 11px}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(260px,1fr));gap:16px}.card{border-radius:18px;padding:18px}.cap{border-top:1px solid #2a3554;margin-top:12px;padding-top:12px}.cap strong{display:block}.evidence{font-weight:700}.work{display:block;margin-top:5px;font-size:.8rem}
</style></head><body>
<p class="muted">KOSCHEI UNIFIED CONTROL PLANE · EXPERIMENTAL</p><h1>Koschei Fabric</h1>
<p class="muted">Koschei Web3, Koschei Lang and Koschei Sentinel are joined through explicit contracts. Existing production behavior stays intact; evidence maturity is shown separately so research or partial work is never presented as completed security acceptance.</p>
<div class="rules"><span class="pill">workspace: {{.Workspace}}</span><span class="pill">mode: {{.IntegrationMode}}</span><span class="pill">preserve existing: {{.PreserveExisting}}</span><span class="pill">acceptance cases: {{.AcceptanceCaseCount}}</span></div>
<div class="grid">{{range .Components}}<section class="card"><p class="muted">{{.Mode}}</p><h2>{{.Name}}</h2><p class="muted">{{.Role}}</p>{{range .Capabilities}}<div class="cap"><strong>{{.ID}}</strong><span class="muted">{{.Domain}} · lifecycle {{.Status}} · evidence <span class="evidence">{{.EvidenceState}}</span> · backend {{.Backend}} · frontend {{.Frontend}} · telemetry {{.Telemetry}}</span><span class="muted work">{{range .WorkPackages}}{{.}} {{end}}</span></div>{{end}}</section>{{end}}</div>
</body></html>`))

func fabricOperatorSurface(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := fabricPage.Execute(w, currentFabricSnapshot()); err != nil {
		http.Error(w, "fabric surface unavailable", http.StatusInternalServerError)
	}
}
