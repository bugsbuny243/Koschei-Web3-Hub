package services

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"koschei/api/internal/radaradapter"
)

type tronIntegrationTransport func(*http.Request) (*http.Response, error)

func (fn tronIntegrationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

// Exercises the real Tron constructor, both fixed TronGrid RPC requests,
// universal registry, evidence adaptation and observed-only enforcement.
// Network responses are deterministic fixtures; this is not a live RPC test.
func TestTronWalletRegistryEndToEndObservedOnly(t *testing.T) {
	t.Setenv("TRONGRID_API_KEY", "fixture-tron-key")
	const account = "41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	heightCalls, accountCalls := 0, 0
	client := &http.Client{Transport: tronIntegrationTransport(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("TRON-PRO-API-KEY") != "fixture-tron-key" || req.Method != http.MethodPost {
			t.Fatal("TronGrid RPC request missing auth or wrong method")
		}
		var payload string
		switch req.URL.String() {
		case "https://api.trongrid.io/wallet/getnowblock":
			heightCalls++
			payload = `{"block_header":{"raw_data":{"number":12345}}}`
		case "https://api.trongrid.io/wallet/getaccount":
			accountCalls++
			payload = `{"address":"` + account + `","owner_permission":{"id":0,"type":0,"threshold":1,"keys":[{"address":"` + account + `","weight":1}]}}`
		default:
			t.Fatalf("unexpected RPC destination: %s", req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(payload))}, nil
	})}
	adapter, err := radaradapter.NewTronMainnetWalletAdapter(client, account)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewUniversalAdapterRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	collected, err := registry.CollectTronWalletAddress(context.Background(), account)
	if err != nil {
		t.Fatal(err)
	}
	if heightCalls != 1 || accountCalls != 1 || !collected.Executable ||
		collected.Route != UniversalDispatchTronWallet || len(collected.Evidence) != 1 ||
		len(collected.Limitations) != 1 {
		t.Fatalf("unexpected Tron collection: height=%d account=%d result=%+v", heightCalls, accountCalls, collected)
	}
	evidence := collected.Evidence[0]
	if evidence.SubjectID != "tron-mainnet:account:"+account ||
		evidence.ChainFamily != "tron" || evidence.Chain != "tron" ||
		evidence.Status != IntelligenceEvidenceObserved ||
		evidence.Attributes["account_state_height_verified"] != false {
		t.Fatalf("Tron evidence promoted or misbound: %+v", evidence)
	}
}
