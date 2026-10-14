package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type tronContractTransport func(*http.Request) (*http.Response, error)

func (f tronContractTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestTronWalletEvidenceHTTPContract(t *testing.T) {
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	const address = "41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for _, code := range []int{200, 429, 503} {
		http.DefaultTransport = tronContractTransport(func(req *http.Request) (*http.Response, error) {
			payload := `{"block_header":{"raw_data":{"number":123}}}`
			if req.URL.Path == "/wallet/getaccount" {
				payload = `{"address":"` + address + `","owner_permission":{"type":"Owner","threshold":1,"keys":[{"address":"` + address + `","weight":1}]}}`
			}
			return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(payload))}, nil
		})
		w := httptest.NewRecorder()
		(&Handler{}).TronWalletPermissions(w, httptest.NewRequest("GET", "/?address="+address, nil))
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		expected := 503
		if code == 200 {
			expected = 200
		}
		if w.Code != expected {
			t.Fatalf("status=%d body=%v", w.Code, body)
		}
		for _, key := range []string{"ok", "collector_status", "provider", "provider_latency_ms", "evidence_row_count"} {
			if _, ok := body[key]; !ok {
				t.Errorf("missing %s: %v", key, body)
			}
		}
		if code == 200 && (body["ok"] != true || body["verdict"] != "withhold" || body["grade_issued"] != false || body["account_state_height_verified"] != false || len(body["unmet_evidence_reasons"].([]any)) == 0) {
			t.Fatalf("unsupported outcome: %v", body)
		}
	}
}
