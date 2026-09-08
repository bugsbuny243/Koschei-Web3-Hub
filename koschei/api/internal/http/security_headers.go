package http

import (
	"net/http"
	"net/url"
	"os"
	"strings"
)

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
		w.Header().Set("Content-Security-Policy", koscheiBaseCSP())
		if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}
		if sensitiveStaticProbePath(r.URL.Path) {
			w.Header().Set("Cache-Control", "no-store")
			http.NotFound(w, r)
			return
		}
		secured := newCSPHTMLResponseWriter(w, r)
		next.ServeHTTP(secured, r)
		secured.finish()
	})
}

// sensitiveStaticProbePath rejects common repository, environment, credential and
// server-configuration paths before they can reach the public FileServer/SPA
// fallback. We intentionally return an ordinary 404 so the public surface does
// not disclose whether a similarly named private file exists on the host.
//
// /.well-known is deliberately not treated as a blanket dotfile: standards such
// as ACME and security.txt may legitimately use that namespace.
func sensitiveStaticProbePath(rawPath string) bool {
	for _, segment := range strings.Split(strings.ReplaceAll(strings.ToLower(rawPath), "\\", "/"), "/") {
		segment = strings.TrimSpace(segment)
		if segment == "" || segment == "." || segment == ".." || segment == ".well-known" {
			continue
		}
		if segment == ".env" || strings.HasPrefix(segment, ".env.") || strings.HasPrefix(segment, ".env-") {
			return true
		}
		switch segment {
		case ".git", ".svn", ".hg", ".ssh", ".aws", ".docker", ".kube",
			".npmrc", ".yarnrc", ".pypirc", ".netrc", ".htaccess", ".htpasswd",
			".ds_store", "id_rsa", "id_ed25519", "credentials.json", "service-account.json":
			return true
		}
	}
	return false
}

func allowedCORSOrigin(origin string, allowed map[string]struct{}) string {
	canonical := canonicalCORSOrigin(origin, true)
	if canonical == "" {
		return ""
	}
	if _, ok := allowed[canonical]; ok {
		return canonical
	}
	return ""
}

func canonicalCORSOrigin(origin string, allowLoopbackHTTP bool) string {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		return ""
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return ""
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	host := strings.ToLower(strings.TrimSpace(u.Host))
	switch scheme {
	case "https":
		return "https://" + host
	case "http":
		if allowLoopbackHTTP && isLoopbackCORSHost(u.Hostname()) {
			return "http://" + host
		}
	}
	return ""
}

func isLoopbackCORSHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "127.0.0.1" || host == "::1"
}
