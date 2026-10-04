package http

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// Both probe routes share an application budget. These canonical /api/ bucket
// names reuse the existing PostgreSQL schema even though the public transport
// lives under /fabric. Without PostgreSQL the bounded memory store is used.
func fabricProbeProtection(config fabricConfig, next http.HandlerFunc) http.HandlerFunc {
	inflight := make(chan struct{}, 4)
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), sharedSensitiveLimitTimeout)
		defer cancel()
		for _, budget := range []struct {
			key   string
			route string
			rule  sensitiveLimitRule
		}{
			{securityClientIP(r), "/api/fabric/probe/client", sensitiveLimitRule{Limit: 10, Window: time.Minute}},
			{"fabric-application", "/api/fabric/probe/application", sensitiveLimitRule{Limit: 120, Window: time.Minute}},
		} {
			decision, err := consumeSharedSensitiveLimit(ctx, config.db, sensitiveBucketKeyHash(budget.key, budget.route), budget.route, budget.rule)
			if err != nil {
				w.Header().Set("Retry-After", "1")
				writeSecurityJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "probe_protection_unavailable"})
				return
			}
			if !decision.Allowed {
				w.Header().Set("Retry-After", strconv.FormatInt(maxSensitiveResetSeconds(decision.ResetAfterSeconds), 10))
				writeSecurityJSON(w, http.StatusTooManyRequests, map[string]string{"error": "probe_budget_exceeded"})
				return
			}
		}
		select {
		case inflight <- struct{}{}:
			defer func() { <-inflight }()
			next(w, r)
		default:
			w.Header().Set("Retry-After", "1")
			writeSecurityJSON(w, http.StatusTooManyRequests, map[string]string{"error": "probe_concurrency_exceeded"})
		}
	}
}
