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
	var requestPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"result":{"risk":"unknown"}}`))
	}))
	defer srv.Close()

	client := &Client{
		HTTP:             srv.Client(),
		BlockaidKey:      "test-only",
		GoPlusKey:        "test-only",
		BubblemapsKey:    "test-only",
		blockaidOrigin:   srv.URL,
		goplusOrigin:     srv.URL,
		bubblemapsOrigin: srv.URL,
	}
	solanaMint := "So11111111111111111111111111111111111111112"
	evmAddress := "0x1234567890123456789012345678901234567890"
	cases := []Input{
		{Provider: "blockaid", Operation: "token", Chain: "ethereum", Address: evmAddress},
		{Provider: "blockaid", Operation: "address", Chain: "ethereum", Address: evmAddress},
		{Provider: "goplus", Operation: "token", Chain: "solana", Address: solanaMint},
		{Provider: "goplus", Operation: "approval", Chain: "ethereum", Address: evmAddress},
		{Provider: "goplus", Operation: "phishing", Chain: "ethereum", URL: "https://example.org/fake"},
		{Provider: "bubblemaps", Operation: "holders", Chain: "sol", Address: solanaMint},
		{Provider: "bubblemaps", Operation: "map", Chain: "sol", Address: solanaMint},
		{Provider: "bubblemaps", Operation: "subgraph", Chain: "sol", Address: solanaMint, Addresses: []string{solanaMint}},
	}
	for _, input := range cases {
		t.Run(input.Provider+"_"+input.Operation, func(t *testing.T) {
			out, err := client.Query(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if out.Signed || out.RiskLevel != "unknown" || out.Verification != "external_observation_only" {
				t.Fatalf("untrusted provider was granted canonical verdict authority: %+v", out)
			}
			if requestPath == "" || strings.Contains(requestPath, "../") {
				t.Fatalf("unsafe upstream path %q", requestPath)
			}
		})
	}
}

func TestExternalProviderRequestsFailClosed(t *testing.T) {
	client := NewFromEnv()
	cases := []Input{
		{Provider: "blockaid", Operation: "token", Chain: "ethereum", Address: "../../internal"},
		{Provider: "goplus", Operation: "token", Chain: "solana", Address: "bad/path"},
		{Provider: "goplus", Operation: "phishing", Chain: "ethereum", URL: "file:///etc/passwd"},
		{Provider: "bubblemaps", Operation: "subgraph", Chain: "sol", Address: "mint", Addresses: nil},
		{Provider: "blockaid", Operation: "transaction_bulk", Chain: "ethereum", Payload: json.RawMessage(`{"data":[],"chain":"ethereum","account_address":"0x1"}`)},
		{Provider: "unknown", Operation: "token", Chain: "eth", Address: "0x1234567890123456789012345678901234567890"},
	}
	for _, input := range cases {
		if _, err := client.Query(context.Background(), input); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid upstream request accepted: %+v err=%v", input, err)
		}
	}
}

func TestSlitherImportNeverSignsRisk(t *testing.T) {
	client := NewFromEnv()
	input := Input{
		Provider:  "slither",
		Operation: "findings",
		Payload:   json.RawMessage(`{"success":true,"results":{"detectors":[{"check":"reentrancy-eth","impact":"High","confidence":"High","description":"External call"}]}}`),
	}
	result, err := client.Query(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Signed || result.RiskLevel != "unknown" || !strings.Contains(string(result.Data), "reentrancy-eth") {
		t.Fatalf("static analysis observation promoted into a signed verdict: %+v", result)
	}
	input.Payload = json.RawMessage(`{"success":false,"results":{"detectors":[]}}`)
	if _, err := client.Query(context.Background(), input); !errors.Is(err, ErrInvalid) {
		t.Fatalf("failed Slither scan accepted: %v", err)
	}
}
