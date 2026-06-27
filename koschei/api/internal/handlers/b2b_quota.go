package handlers

import (
	"net/http"
	"time"
)

// CheckB2BQuota enforces API-key rate limits and monthly usage quotas.
// It is provider-agnostic and intentionally independent from payment systems.
func (h *Handler) CheckB2BQuota(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := apiPrincipalFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		limit := p.RateLimitPerMinute
		if limit <= 0 {
			limit = 100
		}
		if h.Limiter != nil && !h.Limiter.allow("b2b:"+p.KeyID+":"+r.URL.Path, limit, time.Minute) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limit_exceeded"})
			return
		}
		quota := p.MonthlyLimit
		if quota <= 0 {
			quota = 1000
		}
		var used int
		monthStart := time.Now().UTC().Format("2006-01-") + "01"
		_ = h.DB.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(GREATEST(credits_reserved, credits_charged)),0) FROM api_usage_events WHERE api_key_id=$1 AND created_at >= $2::date`, p.KeyID, monthStart).Scan(&used)
		if used >= quota {
			writeJSON(w, http.StatusPaymentRequired, map[string]any{"error": "monthly_quota_exceeded", "monthly_quota": quota, "used": used})
			return
		}
		next(w, r)
	}
}

func (h *Handler) B2BUsage(w http.ResponseWriter, r *http.Request) {
	p, ok := apiPrincipalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	monthStart := time.Now().UTC().Format("2006-01-") + "01"
	var used int
	_ = h.DB.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(credits_charged),0) FROM api_usage_events WHERE api_key_id=$1 AND created_at >= $2::date`, p.KeyID, monthStart).Scan(&used)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                    true,
		"api_key_id":            p.KeyID,
		"email":                 p.Email,
		"rate_limit_per_minute": p.RateLimitPerMinute,
		"monthly_quota":         p.MonthlyLimit,
		"monthly_used":          used,
		"monthly_remaining":     maxInt(p.MonthlyLimit-used, 0),
	})
}

// Kept as a generic plan normalizer because owner operations use this name.
func normalizePlanTier(planTier string) string {
	return normalizePackageID(planTier)
}
