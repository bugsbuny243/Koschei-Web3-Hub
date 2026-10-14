package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRadarHTTPRejectsMalformedAndWithholdsMissingProviders(t *testing.T) {
	t.Setenv("RADAR_PROVIDERS_JSON", "{}")
	for _, tc := range []struct {
		body string
		code int
	}{
		{`{"network":"ethereum-mainnet","kind":"wallet","target":"bad"}`, 400},
		{`{"network":"xrpl-mainnet","kind":"wallet","target":"rUnknown"}`, 501},
		{`{"network":"ethereum-mainnet","kind":"wallet","target":"0x1111111111111111111111111111111111111111"}`, 503},
		{`{"target":"x","provider":"https://evil.invalid"}`, 400},
		{`{"target":"x"} {}`, 400},
	} {
		w := httptest.NewRecorder()
		(&Handler{}).RadarInvestigate(w, httptest.NewRequest("POST", "/api/owner/radar/investigate", strings.NewReader(tc.body)))
		if w.Code != tc.code {
			t.Fatalf("got %d want %d: %s", w.Code, tc.code, w.Body.String())
		}
		if tc.code == 503 {
			var p map[string]any
			json.Unmarshal(w.Body.Bytes(), &p)
			if p["verdict"] != "withhold" || p["grade_issued"] != false {
				t.Fatal(p)
			}
		}
	}
}
