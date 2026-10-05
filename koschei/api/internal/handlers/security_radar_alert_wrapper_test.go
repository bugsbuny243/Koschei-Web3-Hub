package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityRadarCheckWithAlertsRejectsOversizedBody(t *testing.T) {
	h := &Handler{}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/radar/check", bytes.NewReader(bytes.Repeat([]byte{'x'}, maxSecurityRadarAlertBody+1)))
	response := httptest.NewRecorder()
	h.SecurityRadarCheckWithAlerts(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestARVISAlertDedupeKeyIsTenantScoped(t *testing.T) {
	first := arvisAlertDedupeKey("customer-a", "signature-1")
	second := arvisAlertDedupeKey("customer-b", "signature-1")
	if first == second {
		t.Fatalf("tenant-scoped keys collided: %q", first)
	}
	if !strings.Contains(first, "customer-a") || !strings.Contains(second, "customer-b") {
		t.Fatalf("tenant scopes are missing: %q %q", first, second)
	}
}

func TestARVISAlertPayloadRemainsScoreFree(t *testing.T) {
	payload := arvisAlertPayload("mint", "D", "high", "avoid", "signature", "radar-v1")
	if _, exists := payload["risk_index"]; exists {
		t.Fatalf("numeric final score leaked into Radar alert: %#v", payload)
	}
	if payload["grade"] != "D" || payload["signature"] != "signature" || payload["rule_version"] != "radar-v1" {
		t.Fatalf("evidence identity missing: %#v", payload)
	}
}

func TestARVISTelegramMessageUsesStandaloneWeb3Identity(t *testing.T) {
	message, resultID := arvisCustomerTelegramMessage("MintABC", map[string]any{
		"status":            "ready",
		"has_live_evidence": true,
		"final_verdict": map[string]any{
			"risk_level":     "high",
			"grade":          "D",
			"verdict":        "coordinated launch risk",
			"recommendation": "manual review",
			"signature":      "signed-result-123",
		},
	})
	for _, want := range []string{"Koschei Web3 · ARVIS", "MintABC", "Risk: HIGH", "Grade: D", "coordinated launch risk", "live evidence verified"} {
		if !strings.Contains(message, want) {
			t.Fatalf("telegram result missing %q: %s", want, message)
		}
	}
	for _, forbidden := range []string{"Sentinel", "Lang", "unified Professional"} {
		if strings.Contains(message, forbidden) {
			t.Fatalf("standalone Web3 Telegram result leaked bundled product %q: %s", forbidden, message)
		}
	}
	if resultID != "signed-result-123" {
		t.Fatalf("result id=%q", resultID)
	}
}

func TestARVISTelegramMessageIncludesBoundedContextWithoutMakingClaims(t *testing.T) {
	message, _ := arvisCustomerTelegramMessage("MintContext", map[string]any{
		"status":            "ready",
		"has_live_evidence": true,
		"final_verdict": map[string]any{
			"risk_level": "high",
			"grade":      "D",
			"signature":  "context-result-1",
		},
		"investigation_report": map[string]any{
			"trade_ledger_aggregates": map[string]any{
				"market_behavior_status": "bounded_pattern_observed",
			},
			"actor_coordination_intelligence": map[string]any{
				"status": "coordination_patterns_observed",
			},
			"public_promotion_intelligence": map[string]any{
				"status": "cross_asset_public_promotion_overlap_observed",
			},
		},
	})
	for _, want := range []string{
		"suspicious bounded timing pattern observed",
		"requires corroboration",
		"Actor coordination: evidence-backed correlation pattern(s) observed",
		"not an identity or wrongdoing claim",
		"Public promotion: cross-asset public-source overlap observed",
		"context only",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("Telegram bounded context missing %q: %s", want, message)
		}
	}
	for _, forbidden := range []string{"wash trading proven", "manipulation proven", "criminal group", "same operator confirmed"} {
		if strings.Contains(strings.ToLower(message), forbidden) {
			t.Fatalf("Telegram message made unsupported claim %q: %s", forbidden, message)
		}
	}
}

func TestARVISTelegramMessageDoesNotCallPendingEvidenceSafe(t *testing.T) {
	message, resultID := arvisCustomerTelegramMessage("MintPending", map[string]any{
		"status":            "evidence_pending",
		"has_live_evidence": false,
		"final_verdict": map[string]any{
			"risk_level":     "unknown",
			"recommendation": "collect_more_evidence",
			"signed":         false,
		},
	})
	if resultID != "" {
		t.Fatalf("unsigned result unexpectedly got signature id=%q", resultID)
	}
	for _, want := range []string{"EVIDENCE_PENDING", "Risk: UNKNOWN", "incomplete / pending", "not treated as SAFE"} {
		if !strings.Contains(message, want) {
			t.Fatalf("pending Telegram result missing %q: %s", want, message)
		}
	}
}
