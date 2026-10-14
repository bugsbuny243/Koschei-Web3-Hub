package services

import (
	"context"
	"encoding/json"
	r "koschei/api/internal/radaradapter"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRadarTransportHashesRawResponseAndDoesNotLeakProviderSecret(t *testing.T) {
	raw := `{"fixture":"upstream"}`
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("X-Fixture") != "operator-secret" {
			t.Error("header missing")
		}
		w.Write([]byte(raw))
	}))
	defer server.Close()
	previous := radarHTTPClient
	radarHTTPClient = server.Client()
	t.Cleanup(func() { radarHTTPClient = previous })
	t.Setenv("SOLANA_RPC_BUDGET_ENABLED", "false")
	t.Setenv("SOLANA_RPC_GOVERNOR_ENABLED", "false")
	cfg := map[string]RadarNetworkProviders{"fixture": {Primary: RadarProvider{URL: server.URL, Headers: map[string]string{"X-Fixture": "operator-secret"}, RequestsPerSecond: 100}}}
	encoded, _ := json.Marshal(cfg)
	t.Setenv("RADAR_PROVIDERS_JSON", string(encoded))
	tr := NewRadarTransport()
	response, e := tr.Do(context.Background(), r.Request{Network: "fixture", Method: "GET", Path: "/"})
	if e != nil || string(response.Raw) != raw || response.Host != "127.0.0.1" {
		t.Fatal(response, e)
	}
	if tr.Count() != 1 {
		t.Fatal("request accounting failed")
	}
	_, e = tr.Do(context.Background(), r.Request{Network: "fixture", Method: "GET", Path: "//evil.example"})
	if e == nil || strings.Contains(e.Error(), "operator-secret") {
		t.Fatal("unsafe path/error")
	}
}
func TestRadarProviderBudgetHonorsCancellationAndDailyCap(t *testing.T) {
	p := RadarProvider{RequestsPerSecond: 1, DailyLimit: 1}
	if e := reserveRadarProvider(context.Background(), "budget.fixture.invalid", p); e != nil {
		t.Fatal(e)
	}
	if e := reserveRadarProvider(context.Background(), "budget.fixture.invalid", p); e == nil {
		t.Fatal("daily limit ignored")
	}
	p.DailyLimit = 10
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if e := reserveRadarProvider(ctx, "budget.fixture.invalid", p); e == nil {
		t.Fatal("deadline ignored")
	}
}
func TestRadarExecutionWithoutProvidersNeverCompletesOrGrades(t *testing.T) {
	t.Setenv("RADAR_PROVIDERS_JSON", "{}")
	out := ExecuteRadarInvestigation(context.Background(), r.Target{Network: "ethereum-mainnet", Kind: "address", Raw: "0x" + strings.Repeat("1", 40)})
	if out.Status != "insufficient_evidence" || out.Verdict != "withhold" || out.GradeIssued || len(out.Evidence) > 0 || len(out.Limitations) == 0 {
		t.Fatalf("false completion %#v", out)
	}
}
