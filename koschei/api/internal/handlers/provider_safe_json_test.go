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

func TestWriteProviderSafeJSONPreservesLargeValidJSON(t *testing.T) {
	const value = "testvalue123"
	recorder := httptest.NewRecorder()
	writeProviderSafeJSON(recorder, http.StatusOK, map[string]any{
		"ok":    true,
		"large": strings.Repeat("evidence-", 80),
		"nested": map[string]any{
			"error": `Post "https://mainnet.helius-rpc.com/?api-key=` + value + `": provider cooling down`,
		},
	})

	if recorder.Body.Len() <= 240 {
		t.Fatalf("large response was unexpectedly truncated: %d bytes", recorder.Body.Len())
	}
	var decoded map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("large provider-safe response must remain valid JSON: %v body=%q", err, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), value) {
		t.Fatalf("provider credential leaked in large JSON response: %s", recorder.Body.String())
	}
}
