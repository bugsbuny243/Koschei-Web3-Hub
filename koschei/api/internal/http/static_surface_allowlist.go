package http

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// allowedStandalonePages is intentionally explicit. These are auxiliary
// account/auth/docs/public-proof or not-yet-migrated operational surfaces; they
// are not additional primary product surfaces. Home and Customer Panel remain
// the only canonical product surfaces.
var allowedStandalonePages = []string{
	"account.html",
	"address-poisoning-shield.html",
	"agent-api.html",
	"architecture.html",
	"arvis-chat.html",
	"cases.html",
	"chain-health.html",
	"chains.html",
	"dashboard.html",
	"developers.html",
	"docs-api.html",
	"docs-sdk.html",
	"docs.html",
	"forgot-password.html",
	"impact.html",
	"live.html",
	"login.html",
	"metadata.html",
	"owner-production.html",
	"pay-per-tool.html",
	"pilot.html",
	"pricing.html",
	"privacy.html",
	"refund-policy.html",
	"register.html",
	"reports.html",
	"reset-password.html",
	"solana-security-tools.html",
	"support.html",
	"terms.html",
	"token-2022-scanner.html",
	"transaction-firewall.html",
	"watchlist.html",
	"webhooks.html",
}

var allowedRootStaticAssets = map[string]struct{}{
	"/agent-widget.js":              {},
	"/widget.js":                    {},
	"/full-scan-contract-v1.json":   {},
	"/security-ecosystem.json":      {},
	"/sitemap.xml":                  {},
	"/validation-key.txt":           {},
	"/app/login":                    {},
	"/owner/login":                  {},
}

func registerStaticAllowlisted(mux *http.ServeMux, staticDir string) {
	if staticDir == "" {
		return
	}
	info, err := os.Stat(staticDir)
	if err != nil || !info.IsDir() {
		log.Printf("warning: static directory unavailable at %q: %v", staticDir, err)
		return
	}

	registerStaticAliases(mux, staticDir)
	registerAllowedStandalonePages(mux, staticDir)

	fileServer := http.FileServer(http.Dir(staticDir))
	indexPath := filepath.Join(staticDir, "index.html")
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path == "/" {
			http.ServeFile(w, r, indexPath)
			return
		}
		if !allowedStaticAssetPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		clean := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
		candidate := filepath.Join(staticDir, clean)
		if info, err := os.Stat(candidate); err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func registerAllowedStandalonePages(mux *http.ServeMux, staticDir string) {
	for _, filename := range allowedStandalonePages {
		filename := filename
		fullPath := filepath.Join(staticDir, filename)
		htmlRoute := "/" + filename
		cleanRoute := "/" + strings.TrimSuffix(filename, ".html")

		// Routes already owned by a more specific router keep that authority.
		if !staticAliasOwnsRoute(cleanRoute) {
			registerStaticFileAlias(mux, cleanRoute, fullPath)
		}
		if !staticAliasOwnsRoute(htmlRoute) {
			registerStaticFileAlias(mux, htmlRoute, fullPath)
		}
	}
}

func staticAliasOwnsRoute(route string) bool {
	switch route {
	case "/docs/api", "/docs/sdk",
		"/owner", "/owner.html",
		"/agents", "/agents/", "/agents/pilot", "/agents/pilot/",
		"/agents/admin", "/agents/admin/", "/agents/install", "/agents/install/":
		return true
	default:
		return false
	}
}

func allowedStaticAssetPath(requestPath string) bool {
	if _, ok := allowedRootStaticAssets[requestPath]; ok {
		return true
	}
	if strings.HasPrefix(requestPath, "/css/") {
		return strings.HasSuffix(strings.ToLower(requestPath), ".css")
	}
	if strings.HasPrefix(requestPath, "/js/") {
		lower := strings.ToLower(requestPath)
		return strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".mjs")
	}
	if strings.HasPrefix(requestPath, "/sdk/") {
		lower := strings.ToLower(requestPath)
		return strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".mjs")
	}
	return false
}
