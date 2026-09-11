package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
)

type portablePublicCasePageData struct {
	Case            publicDossierCaseV2
	RegistryBackend string
	RegistryHash    string
	GeneratedAt     string
}

var portablePublicCaseHTML = template.Must(template.New("portable-public-case").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Case.CaseRef}} · Koschei Web3</title>
<meta name="robots" content="index,follow">
<style>
body{font-family:system-ui,-apple-system,sans-serif;background:#0b0c12;color:#eef2ff;margin:0;padding:24px}main{max-width:920px;margin:auto}.card{background:#141722;border:1px solid #2b3142;border-radius:16px;padding:20px;margin:14px 0}.muted{color:#9ba7bd}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:12px}.metric{background:#0f121b;border:1px solid #262c3b;border-radius:12px;padding:14px}.value{font-size:1.6rem;font-weight:700}.hash{word-break:break-all;font-family:ui-monospace,monospace;font-size:.88rem}a{color:#8fd3ff}</style>
</head>
<body><main>
<h1>{{if .Case.Title}}{{.Case.Title}}{{else}}{{.Case.CaseRef}}{{end}}</h1>
<p class="muted">Portable public case projection. This page reports only fields present in the verified public registry snapshot; it does not invent missing dossier detail.</p>
<div class="card">
<p><strong>Case:</strong> {{.Case.CaseRef}}</p>
<p><strong>Network:</strong> {{.Case.Network}}</p>
<p><strong>Target:</strong> {{.Case.TargetDisplay}}</p>
<p><strong>Verdict:</strong> {{.Case.VerdictGrade}} {{.Case.VerdictStatus}}</p>
<p>{{.Case.Summary}}</p>
</div>
<div class="grid">
<div class="metric"><div class="value">{{.Case.EvidenceRows}}</div><div class="muted">Evidence rows</div></div>
<div class="metric"><div class="value">{{.Case.VerifiedRows}}</div><div class="muted">Verified</div></div>
<div class="metric"><div class="value">{{.Case.ObservedRows}}</div><div class="muted">Observed</div></div>
<div class="metric"><div class="value">{{.Case.OpenRows}}</div><div class="muted">Open</div></div>
<div class="metric"><div class="value">{{.Case.BlockedRows}}</div><div class="muted">Blocked</div></div>
</div>
<div class="card">
<p><strong>Immutable bundle hash</strong></p><p class="hash">{{.Case.BundleHash}}</p>
<p><strong>Publication ledger:</strong> {{.Case.PublicationLedgerStatus}}</p>
<p><strong>Published by:</strong> {{.Case.PublishedBy}}</p>
<p><strong>Registry backend:</strong> {{.RegistryBackend}}</p>
{{if .RegistryHash}}<p><strong>Registry snapshot SHA-256</strong></p><p class="hash">{{.RegistryHash}}</p>{{end}}
<p class="muted">Registry generated at {{.GeneratedAt}}</p>
</div>
<p><a href="/api/public/cases">← Public case registry</a></p>
</main></body></html>`))

// PublicCasePortablePage keeps the database-backed integrity path unchanged while
// allowing explicitly configured Drive registry deployments to serve a safe case
// detail projection without requiring application persistence.
func (h *Handler) PublicCasePortablePage(w http.ResponseWriter, r *http.Request) {
	caseRef := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/case/"))
	if !publicDossierCaseRefPattern.MatchString(caseRef) {
		http.NotFound(w, r)
		return
	}

	backend := strings.ToLower(strings.TrimSpace(os.Getenv("KOSCHEI_PUBLIC_REGISTRY_BACKEND")))
	if backend == "" {
		backend = "database"
	}
	switch backend {
	case "database":
		h.PublicCaseOperationalPageV2(w, r)
		return
	case "drive":
		maxRows, err := publicDossierRegistrySnapshotMaxRows()
		if err != nil {
			http.Error(w, "public case registry configuration invalid", http.StatusServiceUnavailable)
			return
		}
		snapshot, object, err := loadPublicDossierRegistrySnapshotFromDrive(r, maxRows)
		if err != nil {
			http.Error(w, "public case registry unavailable", http.StatusServiceUnavailable)
			return
		}
		var item *publicDossierCaseV2
		for i := range snapshot.Cases {
			if snapshot.Cases[i].CaseRef == caseRef {
				item = &snapshot.Cases[i]
				break
			}
		}
		if item == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
		w.Header().Set("X-Robots-Tag", "index, follow")
		w.Header().Set("X-Koschei-Registry-Backend", "google_drive")
		if hash := strings.ToLower(strings.TrimSpace(object.Hash)); hash != "" {
			w.Header().Set("X-Koschei-Registry-SHA256", hash)
		}
		data := portablePublicCasePageData{
			Case:            *item,
			RegistryBackend: "google_drive",
			RegistryHash:    strings.ToLower(strings.TrimSpace(object.Hash)),
			GeneratedAt:     snapshot.GeneratedAt.UTC().Format("2006-01-02 15:04:05 UTC"),
		}
		if err := portablePublicCaseHTML.Execute(w, data); err != nil {
			http.Error(w, "public case render failed", http.StatusInternalServerError)
		}
		return
	default:
		http.Error(w, fmt.Sprintf("unsupported public registry backend: %s", backend), http.StatusServiceUnavailable)
	}
}
