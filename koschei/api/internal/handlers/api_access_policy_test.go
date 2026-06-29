package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestNormalizeAPIAccessDecisionDowngradesLowConfidenceDeny(t *testing.T) {
	got := normalizeAPIAccessDecision(apiAccessPolicyDecision{
		Decision:           "deny",
		ReasonCode:         "wallet_label",
		EvidenceConfidence: 0.70,
		RateMultiplier:     0.5,
	})
	if got.Decision != "enterprise_review" {
		t.Fatalf("decision = %q, want enterprise_review", got.Decision)
	}
	if got.ReasonCode != "insufficient_evidence_for_hard_restriction" {
		t.Fatalf("reason = %q", got.ReasonCode)
	}
}

func TestNormalizeAPIAccessDecisionKeepsVerifiedHold(t *testing.T) {
	got := normalizeAPIAccessDecision(apiAccessPolicyDecision{
		Decision:           "temporary_hold",
		ReasonCode:         "compromised_key",
		EvidenceConfidence: 0.98,
		RateMultiplier:     1,
	})
	if got.Decision != "temporary_hold" {
		t.Fatalf("decision = %q, want temporary_hold", got.Decision)
	}
}

func TestNormalizeAPIAccessDecisionClampsMultiplier(t *testing.T) {
	got := normalizeAPIAccessDecision(apiAccessPolicyDecision{Decision: "throttle", RateMultiplier: 2})
	if got.RateMultiplier != 1 {
		t.Fatalf("multiplier = %v, want 1", got.RateMultiplier)
	}
}

func TestAPIPolicyClientIPUsesFirstForwardedAddress(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/usage", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	if got := apiPolicyClientIP(r); got != "203.0.113.9" {
		t.Fatalf("ip = %q", got)
	}
}
