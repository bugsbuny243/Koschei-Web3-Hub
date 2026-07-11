package http

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"koschei/api/internal/services"
)

type sensitiveLimitRule struct {
	Limit  int
	Window time.Duration
}

type sensitiveBucket struct {
	Count     int
	ResetTime time.Time
}

var sensitiveLimiter = struct {
	mu      sync.Mutex
	buckets map[string]sensitiveBucket
}{buckets: map[string]sensitiveBucket{}}

func sensitiveRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rule, ok := sensitiveRuleForPath(r.URL.Path)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		key := securityClientIP(r) + "|" + r.URL.Path
		if !allowSensitiveRequest(key, rule) {
			services.WriteSecurityAuditEvent(r.Context(), getSecurityAuditDB(), services.SecurityAuditEvent{EventType: "rate_limit_exceeded", ActorType: "request", IP: securityClientIP(r), UserAgent: r.UserAgent(), Path: r.URL.Path, Severity: "warning", Metadata: map[string]any{"limit": rule.Limit, "window_seconds": int(rule.Window.Seconds())}})
			writeSecurityJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limit_exceeded", "message": "Too many requests. Please try again later."})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sensitiveRuleForPath(path string) (sensitiveLimitRule, bool) {
	path = strings.TrimSpace(path)
	switch path {
	case "/api/owner/login":
		return sensitiveLimitRule{Limit: 5, Window: 15 * time.Minute}, true
	case "/api/auth/login", "/api/auth/register":
		return sensitiveLimitRule{Limit: 10, Window: 10 * time.Minute}, true
	case "/api/arvis/preflight":
		return sensitiveLimitRule{Limit: 30, Window: time.Minute}, true
	case "/api/token/scan":
		return sensitiveLimitRule{Limit: 10, Window: time.Minute}, true
	case "/api/v1/radar/check", "/api/v1/unified/analyze":
		return sensitiveLimitRule{Limit: 30, Window: time.Minute}, true
	case "/api/v1/risk/badge", "/api/v1/security/risk-badge":
		return sensitiveLimitRule{Limit: 20, Window: time.Minute}, true
	case "/api/paddle/webhook", "/api/v1/paddle/webhook", "/api/shopier/webhook":
		return sensitiveLimitRule{Limit: 120, Window: time.Minute}, true
	case "/api/paddle/checkout":
		return sensitiveLimitRule{Limit: 20, Window: 5 * time.Minute}, true
	default:
		return sensitiveLimitRule{}, false
	}
}

func allowSensitiveRequest(key string, rule sensitiveLimitRule) bool {
	now := time.Now()
	sensitiveLimiter.mu.Lock()
	defer sensitiveLimiter.mu.Unlock()
	bucket := sensitiveLimiter.buckets[key]
	if bucket.ResetTime.IsZero() || now.After(bucket.ResetTime) {
		sensitiveLimiter.buckets[key] = sensitiveBucket{Count: 1, ResetTime: now.Add(rule.Window)}
		cleanupSensitiveBucketsLocked(now)
		return true
	}
	if bucket.Count >= rule.Limit {
		return false
	}
	bucket.Count++
	sensitiveLimiter.buckets[key] = bucket
	return true
}

func cleanupSensitiveBucketsLocked(now time.Time) {
	if len(sensitiveLimiter.buckets) < 1024 {
		return
	}
	for key, bucket := range sensitiveLimiter.buckets {
		if now.After(bucket.ResetTime) {
			delete(sensitiveLimiter.buckets, key)
		}
	}
}
