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

func TestToncenterKeyFallbackConfiguresTonPrimaryOnlyInHeader(t *testing.T) {
	const key = "fixture-toncenter-key-not-real"
	t.Setenv("RADAR_PROVIDERS_JSON", "")
	t.Setenv("TONCENTER_API_KEY", " "+key+" ")
	cfg := RadarProviderConfig()
	ton, ok := cfg["ton-mainnet"]
	if !ok {
		t.Fatal("TONCENTER_API_KEY must configure ton-mainnet when no explicit entry exists")
	}
	if ton.Primary.URL != toncenterMainnetURL || ton.Primary.Headers["X-API-Key"] != key {
		t.Fatalf("unexpected fallback provider: url=%q header_set=%v", ton.Primary.URL, ton.Primary.Headers["X-API-Key"] == key)
	}
	if strings.Contains(ton.Primary.URL, key) {
		t.Fatal("toncenter key must not be placed in the provider URL")
	}
	if ton.Secondary.URL != "" || len(cfg) != 1 {
		t.Fatalf("fallback must add only the ton-mainnet primary role, got %d networks", len(cfg))
	}

	found := false
	for _, row := range RadarCapabilityRuntime() {
		capability, _ := row["capability"].(r.Capability)
		if capability.Network == "ton-mainnet" {
			found = true
			if row["runtime_status"] != "primary_configured" || row["live_verified"] != false {
				t.Fatalf("unexpected TON runtime status: %#v", row)
			}
		}
	}
	if !found {
		t.Fatal("TON capability missing from runtime report")
	}
	encoded, err := json.Marshal(RadarCapabilityRuntime())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), key) {
		t.Fatal("capability runtime output leaked the toncenter key")
	}
}

func TestToncenterKeyFallbackNeverOverridesExplicitTonEntry(t *testing.T) {
	t.Setenv("TONCENTER_API_KEY", "fixture-toncenter-key-not-real")
	t.Setenv("RADAR_PROVIDERS_JSON", `{"ton-mainnet":{"primary":{"url":"https://ton-operator.example.invalid"}}}`)
	ton := RadarProviderConfig()["ton-mainnet"]
	if ton.Primary.URL != "https://ton-operator.example.invalid" || len(ton.Primary.Headers) != 0 {
		t.Fatalf("explicit ton-mainnet entry must win, got %q", ton.Primary.URL)
	}
}

func TestMalformedRadarProvidersFailClosedEvenWithToncenterKey(t *testing.T) {
	t.Setenv("TONCENTER_API_KEY", "fixture-toncenter-key-not-real")
	for _, raw := range []string{"{not-json", `["ton-mainnet"]`} {
		t.Setenv("RADAR_PROVIDERS_JSON", raw)
		if cfg := RadarProviderConfig(); len(cfg) != 0 {
			t.Fatalf("malformed RADAR_PROVIDERS_JSON %q must yield no providers, got %d", raw, len(cfg))
		}
	}
	t.Setenv("RADAR_PROVIDERS_JSON", "null")
	if _, ok := RadarProviderConfig()["ton-mainnet"]; !ok {
		t.Fatal("a JSON null provider map is empty, so the toncenter fallback should still apply")
	}
}

func TestNoToncenterKeyLeavesTonUnconfigured(t *testing.T) {
	t.Setenv("TONCENTER_API_KEY", "")
	t.Setenv("RADAR_PROVIDERS_JSON", "")
	if cfg := RadarProviderConfig(); len(cfg) != 0 {
		t.Fatalf("no providers expected without configuration, got %d", len(cfg))
	}
}
