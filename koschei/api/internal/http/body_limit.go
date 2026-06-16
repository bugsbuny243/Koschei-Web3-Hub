package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"koschei/api/internal/services"
)

func bodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !methodMayHaveBody(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		limit := maxBodyBytesForPath(r.URL.Path)
		if r.ContentLength > limit {
			services.WriteSecurityAuditEvent(r.Context(), getSecurityAuditDB(), services.SecurityAuditEvent{EventType: "request_body_too_large", ActorType: "request", IP: securityClientIP(r), UserAgent: r.UserAgent(), Path: r.URL.Path, Severity: "warning", Metadata: map[string]any{"limit": limit}})
			writeSecurityJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request_body_too_large"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}

func methodMayHaveBody(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func maxBodyBytesForPath(path string) int64 {
	path = strings.TrimSpace(path)
	switch path {
	case "/api/paddle/webhook", "/api/v1/paddle/webhook", "/api/shopier/webhook":
		return 512 << 10
	}
	if strings.Contains(path, "/api/ai/") || strings.Contains(path, "/api/artifacts/") || strings.Contains(path, "/api/v1/build/") {
		return 2 << 20
	}
	return 1 << 20
}

func writeSecurityJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
