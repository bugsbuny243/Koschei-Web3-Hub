package providerintel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExternalProvidersReturnObservationsNotSignedVerdicts(t *testing.T) {
	var gotPath, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		gotKey = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"result":{"risk":"unknown"}}`))
	}))
	defer srv.Close()
	c := &Client{
		HTTP: srv.Client(),
		BlockaidKey: "test-only", GoPlusKey: "test-only", BubblemapsKey: "test-only",
		blockaidOrigin: srv.URL, goplusOrigin: srv.URL, bubblemapsOrigin: srv.URL,
	}
	ctx := context.Background()
	tests := []struct {
		input Input
		method, path string
	}{
		{Input{Provider: "blockaid", Operation: "token", Chain: "ethereum", Address: "0x1234567890123456789012345678901234567890"}, "POST", "/v1/token/scan"},
		{Input{Provider: "blockaid", Operation: "address", Chain: "ethereum", Address: "0x1234567890123456789012345678901234567890"}, "POST", "/v1/address/scan"},
		{Input{Provider: "goplus", Operation: "token", Chain: "solana", Address: "So11111111111111111111111111111111111111112"}, "GET", "/api/v1/solana/token_security"},
		{Input{Provider: "goplus", Operation: "approval", Chain: "ethereum", Address: "0x1234567890123456789012345678901234567890"}, "GET", "/api/v1/approval_security/1"},
		{Input{Provider: "goplus", Operation: "phishing", Chain: "ethereum", URL: "https://example.org/fake"}, "GET", "/api/v1/phishing_site"},
		{Input{Provider: "bubblemaps", Operation: "holders", Chain: "sol", Address: "So11111111111111111111111111111111111111112"}, "GET", "/v0/tokens/holders/sol/"},
		{Input{Provider: "bubblemaps", Operation: "map", Chain: "sol", Address: "So11111111111111111111111111111111111111112"}, "GET", "/v0/tokens/map/sol/"},
		{Input{Provider: "bubblemaps", Operation: "subgraph", Chain: "sol", Address: "So11111111111111111111111111111111111111112", Addresses: []string{"So11111111111111111111111111111111111111112"}}, "POST", "/v0/relationships/subgraph"},
	}
	for _, tc := range tests {
		t.Run(tc.input.Provider+"_"+tc.input.Operation, func(t *testing.T) {
			out, err := c.Query(ctx, tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if out.Signed || out.RiskLevel != "unknown" || out.Verification != "external_observation_only" {
				t.Fatalf("untrusted third party gained ARVIS verdict authority: %+v", out)
			}
			if gotKey == "" && tc.input.Provider != "goplus" {
				t.Fatal("expected server-side provider credential")
			}
			if !strings.HasPrefix(gotPath, tc.path) {
				t.Fatalf("wrong upstream path: %q want %q", gotPath, tc.path)
			}
		})
	}
}

func TestExternalProviderBadRequestsFailClosed(t *testing.T) {
	c := NewFromEnv()
	for _, tc := range []Input{
		{Provider: "blockaid", Operation: "token", Chain: "ethereum", Address: "../../internal"},
		{Provider: "goplus", Operation: "token", Chain: "solana", Address: "bad/path"},
		{Provider: "goplus", Operation: "phishing", Chain: "ethereum", URL: "file:///etc/passwd"},
		{Provider: "bubblemaps", Operation: "subgraph", Chain: "sol", Address: "mint", Addresses: []string{}},
		{Provider: "blockaid", Operation: "transaction_bulk", Chain: "ethereum", Payload: json.RawMessage(`{"data":[],"chain":"ethereum","account_address":"0x1"}`)},
		{Provider: "unknown", Operation: "token", Chain: "eth", Address: "0x1234567890123456789012345678901234567890"},
	} {
		if _, err := c.Query(context.Background(), tc); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid third-party request was not refused: %+v err=%v", tc, err)
		}
	}
}

func TestSlitherImportNeverSignsRiskAndRejectsBrokenScan(t *testing.T) {
	c := NewFromEnv()
	input := Input{
		Provider: "slither", Operation: "findings",
		Payload: json.RawMessage(`{"success":true,"results":{"detectors":[{"check":"reentrancy-eth","impact":"High","confidence":"High","description":"External call"}]}}`),
	}
	got, err := c.Query(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Signed || got.RiskLevel != "unknown" || !strings.Contains(string(got.Data), "reentrancy-eth") {
		t.Fatalf("Slither observation incorrectly promoted: %+v", got)
	}
	input.Payload = json.RawMessage(`{"success":false,"results":{"detectors":[]}}`)
	if _, err := c.Query(context.Background(), input); !errors.Is(err, ErrInvalid) {
		t.Fatalf("failed static analysis accepted: %v", err)
	}
}
