package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteProviderSafeJSONRedactsNestedProviderURL(t *testing.T) {
	const value = "testvalue123"
	recorder := httptest.NewRecorder()
	writeProviderSafeJSON(recorder, http.StatusOK, map[string]any{
		"actor": map[string]any{
			"limitations": []string{
				`Post "https://mainnet.helius-rpc.com/?api-key=` + value + `": provider cooling down`,
			},
		},
	})

	body := recorder.Body.String()
	if strings.Contains(body, value) {
		t.Fatalf("provider credential leaked in JSON response: %s", body)
	}
	if !strings.Contains(body, "api-key=[redacted]") {
		t.Fatalf("redaction marker missing from JSON response: %s", body)
	}
}

func TestWriteProviderSafeJSONDoesNotTruncateLargePayload(t *testing.T) {
	const value = "testvalue123"
	recorder := httptest.NewRecorder()
	writeProviderSafeJSON(recorder, http.StatusOK, map[string]any{
		"summary":        strings.Repeat("x", 600),
		"provider_error": "https://rpc.example.test/?api_key=" + value + "&network=mainnet",
	})

	body := recorder.Body.String()
	if len(body) <= 240 {
		t.Fatalf("provider-safe JSON was truncated: length=%d body=%q", len(body), body)
	}
	if strings.Contains(body, value) {
		t.Fatalf("provider credential leaked in JSON response: %s", body)
	}
	if !strings.Contains(body, "api_key=[redacted]") {
		t.Fatalf("redaction marker missing from JSON response: %s", body)
	}
	if !json.Valid([]byte(body)) {
		t.Fatalf("provider-safe response must remain valid JSON: %s", body)
	}
}
