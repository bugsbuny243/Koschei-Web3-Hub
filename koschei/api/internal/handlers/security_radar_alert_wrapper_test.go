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
