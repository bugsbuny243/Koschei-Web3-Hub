package handlers

import (
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"
)

type apiAccessPolicyDecision struct {
	PolicyID           string
	OrganizationID     string
	Decision           string
	ReasonCode         string
	ReasonDetail       string
	RateMultiplier     float64
	EvidenceConfidence float64
}

func normalizeAPIAccessDecision(decision apiAccessPolicyDecision) apiAccessPolicyDecision {
	// A probabilistic label alone must never produce a hard denial.
	if (decision.Decision == "deny" || decision.Decision == "temporary_hold") && decision.EvidenceConfidence < 0.90 {
		decision.Decision = "enterprise_review"
		decision.ReasonCode = "insufficient_evidence_for_hard_restriction"
	}
	if decision.RateMultiplier <= 0 || decision.RateMultiplier > 1 {
		decision.RateMultiplier = 1
	}
	return decision
}

func (h *Handler) enforceAPIAccessPolicy(w http.ResponseWriter, r *http.Request, principal apiPrincipal) bool {
	decision, found, err := h.resolveAPIAccessPolicy(r, principal)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error":   "access_policy_unavailable",
			"message": "Kurumsal erişim politikası doğrulanamadı.",
		})
		return false
	}
	if !found || decision.Decision == "allow" {
		return true
	}
	decision = normalizeAPIAccessDecision(decision)
	h.recordAPIAccessDecision(r, principal, decision)

	switch decision.Decision {
	case "throttle":
		limit := principal.RateLimitPerMinute
		if limit <= 0 {
			limit = 60
		}
		limit = int(float64(limit) * decision.RateMultiplier)
		if limit < 1 {
			limit = 1
		}
		if h.Limiter != nil && !h.Limiter.allow("entity-policy:"+principal.KeyID+":"+r.URL.Path, limit, time.Minute) {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":         "policy_rate_limit_exceeded",
				"reason_code":   decision.ReasonCode,
				"retry_after_s": 60,
			})
			return false
		}
		return true
	case "enterprise_review":
		writeJSON(w, http.StatusForbidden, map[string]any{
			"error":       "enterprise_review_required",
			"reason_code": decision.ReasonCode,
			"message":     "Bu erişim için kurumsal inceleme gerekiyor.",
		})
		return false
	case "temporary_hold":
		writeJSON(w, http.StatusLocked, map[string]any{
			"error":       "temporary_access_hold",
			"reason_code": decision.ReasonCode,
			"message":     "Erişim güvenlik incelemesi tamamlanana kadar geçici olarak durduruldu.",
		})
		return false
	case "deny":
		writeJSON(w, http.StatusForbidden, map[string]any{
			"error":       "api_access_denied",
			"reason_code": decision.ReasonCode,
		})
		return false
	default:
		return true
	}
}

func (h *Handler) resolveAPIAccessPolicy(r *http.Request, principal apiPrincipal) (apiAccessPolicyDecision, bool, error) {
	var out apiAccessPolicyDecision
	var organizationID sql.NullString
	err := h.DB.QueryRowContext(r.Context(), `
		SELECT
			p.id::text,
			p.organization_id::text,
			p.decision,
			p.reason_code,
			p.reason_detail,
			p.rate_limit_multiplier::float8,
			p.evidence_confidence::float8
		FROM api_access_policies p
		LEFT JOIN api_key_organizations ako ON ako.api_key_id=$1::uuid
		WHERE p.active=true
		  AND p.starts_at <= now()
		  AND (p.expires_at IS NULL OR p.expires_at > now())
		  AND (
			p.api_key_id=$1::uuid
			OR (p.organization_id IS NOT NULL AND p.organization_id=ako.organization_id)
			OR (p.auth_subject IS NOT NULL AND p.auth_subject=$2)
		  )
		ORDER BY
			CASE p.decision
				WHEN 'deny' THEN 5
				WHEN 'temporary_hold' THEN 4
				WHEN 'enterprise_review' THEN 3
				WHEN 'throttle' THEN 2
				ELSE 1
			END DESC,
			CASE
				WHEN p.api_key_id IS NOT NULL THEN 3
				WHEN p.organization_id IS NOT NULL THEN 2
				ELSE 1
			END DESC,
			p.created_at DESC
		LIMIT 1`, principal.KeyID, principal.AuthSubject).
		Scan(&out.PolicyID, &organizationID, &out.Decision, &out.ReasonCode, &out.ReasonDetail, &out.RateMultiplier, &out.EvidenceConfidence)
	if err == sql.ErrNoRows {
		return apiAccessPolicyDecision{}, false, nil
	}
	if err != nil {
		return apiAccessPolicyDecision{}, false, err
	}
	if organizationID.Valid {
		out.OrganizationID = organizationID.String
	}
	return out, true, nil
}

func (h *Handler) recordAPIAccessDecision(r *http.Request, principal apiPrincipal, decision apiAccessPolicyDecision) {
	metadata, _ := json.Marshal(map[string]any{
		"reason_detail":       decision.ReasonDetail,
		"rate_multiplier":     decision.RateMultiplier,
		"evidence_confidence": decision.EvidenceConfidence,
	})
	var organization any
	if decision.OrganizationID != "" {
		organization = decision.OrganizationID
	}
	_, _ = h.DB.ExecContext(r.Context(), `
		INSERT INTO api_access_decisions
		(api_key_id,organization_id,auth_subject,endpoint,decision,reason_code,policy_id,request_ip,user_agent,metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb)`,
		principal.KeyID,
		organization,
		principal.AuthSubject,
		r.URL.Path,
		decision.Decision,
		decision.ReasonCode,
		decision.PolicyID,
		apiPolicyClientIP(r),
		r.UserAgent(),
		string(metadata),
	)
}

func apiPolicyClientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
