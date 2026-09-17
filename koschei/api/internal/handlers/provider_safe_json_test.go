package handlers

import (
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
