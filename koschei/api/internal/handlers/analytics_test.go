package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnalyticsEventAcceptsValidEventWithoutDatabase(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/analytics/event", strings.NewReader(`{
		"event_name":"login_success",
		"email":"USER@example.com",
		"path":"/login",
		"metadata":{"method":"password"}
	}`))
	res := httptest.NewRecorder()

	h.AnalyticsEvent(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", res.Code)
	}
	if strings.TrimSpace(res.Body.String()) != `{"ok":true}` {
		t.Fatalf("unexpected response body: %s", res.Body.String())
	}
}

func TestAnalyticsEventIgnoresMalformedPayload(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/analytics/event", strings.NewReader(`{"event_name":`))
	res := httptest.NewRecorder()

	h.AnalyticsEvent(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", res.Code)
	}
	if strings.TrimSpace(res.Body.String()) != `{"ok":true}` {
		t.Fatalf("unexpected response body: %s", res.Body.String())
	}
}
