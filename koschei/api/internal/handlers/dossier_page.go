package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
)

type dossierPageData struct {
	Bundle dossierBundle
	TR bool
	Sections []dossierPageSection
}

type dossierPageSection struct {
	Title string
	Content string
}

func (h *Handler) DossierPage(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DB == nil { http.NotFound(w, r); return }
	caseRef := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/dossier/"))
	if caseRef == "" || strings.Contains(caseRef, "/") { http.NotFound(w, r); return }
	var raw []byte
	if h.DB.QueryRowContext(r.Context(), `SELECT canonical_bundle FROM dossier_exports WHERE case_ref=$1`, caseRef).Scan(&raw) != nil { http.NotFound(w, r); return }
	var bundle dossierBundle
	if json.Unmarshal(raw, &bundle) != nil { http.Error(w, "export unavailable", http.StatusServiceUnavailable); return }
	card := dossierMap(bundle.VerdictCard)
	data := dossierPageData{Bundle: bundle, TR: strings.EqualFold(r.URL.Query().Get("lang"), "tr"), Sections: []dossierPageSection{
		{Title:"1 · Target and signed result",Content:dossierPretty(map[string]any{"token":bundle.Token,"verdict":bundle.Verdict})},
		{Title:"2 · Signal coverage",Content:dossierPretty(card["signal_rows"])},
		{Title:"3 · Threat pathways",Content:dossierPretty(bundle.ThreatAnticipation)},
		{Title:"4 · Evidence arms",Content:dossierPretty(bundle.EvidenceArms)},
		{Title:"5 · Transaction evidence",Content:dossierPretty(bundle.TransactionEvidence)},
		{Title:"6 · Actor observations",Content:dossierPretty(bundle.ActorDossier)},
		{Title:"7 · Independent verification",Content:dossierPretty(bundle.Verification)},
		{Title:"8 · Limitations",Content:dossierPretty(bundle.Limitations)},
	}}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=300")
	_ = dossierHTML.Execute(w, data)
}

var dossierHTML = template.Must(template.New("dossier").Parse(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Koschei {{.Bundle.CaseRef}}</title><style>@page{size:A4;margin:16mm}body{font:12px/1.5 Arial;color:#111;margin:0}h1{font-size:24px}h2{font-size:16px;border-bottom:2px solid;padding-bottom:4px}header,section{margin-bottom:18px;break-inside:avoid}.meta{display:grid;grid-template-columns:1fr 1fr;gap:8px}.box,pre{border:1px solid #888;padding:9px}.mono,pre{font-family:monospace;overflow-wrap:anywhere}pre{white-space:pre-wrap;font-size:9px}footer{position:fixed;bottom:0;font-size:9px;color:#555}@media(max-width:640px){.meta{grid-template-columns:1fr}}</style></head><body><header><h1>{{if .TR}}Koschei Teknik Kanıt Çıktısı{{else}}Koschei Technical Evidence Export{{end}}</h1><div class="meta"><div class="box mono">{{.Bundle.CaseRef}}</div><div class="box mono">{{.Bundle.BundleHash}}</div></div></header>{{range .Sections}}<section><h2>{{.Title}}</h2><pre>{{.Content}}</pre></section>{{end}}<footer>{{.Bundle.CaseRef}} · Koschei evidence-first export</footer></body></html>`))
