package services

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"koschei/api/internal/radaradapter"
)

type tronRegistryTransport func(*http.Request) (*http.Response, error)

func (transport tronRegistryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return transport(req)
}

func TestTronWalletRegistryCollectsObservedEvidenceWithoutRoutePromotion(t *testing.T) {
	const account = "41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	client := &http.Client{Transport: tronRegistryTransport(func(req *http.Request) (*http.Response, error) {
		var payload string
		switch req.URL.Path {
		case "/wallet/getnowblock":
			payload = `{"block_header":{"raw_data":{"number":12345}}}`
		case "/wallet/getaccount":
			payload = `{"address":"` + account + `","owner_permission":{"id":0,"type":0,"threshold":1,"keys":[{"address":"` + account + `","weight":1}]}}`
		default:
			t.Fatalf("unexpected Tron RPC path: %s", req.URL.Path)
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
	plan := UniversalInvestigationPlan{
		Subject: IntelligenceSubject{ID: "tron-fixture", CanonicalRef: "tron-mainnet:account:" + account,
			Network: "tron-mainnet", Kind: "account", Raw: account},
		Route: UniversalDispatchDeclaredOnly,
	}
	result, err := registry.Collect(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.Executable || result.Route != UniversalDispatchDeclaredOnly || len(result.Evidence) != 1 {
		t.Fatalf("Tron evidence incorrectly promoted or lost: %+v", result)
	}
	if len(result.Limitations) < 2 {
		t.Fatalf("missing unanchored-height and route limitations: %+v", result)
	}
	if !registry.HasRuntimeCollector(radaradapter.Target{Network: "tron-mainnet", Kind: "account", Raw: account}) {
		t.Fatal("installed Tron collector not discoverable")
	}
}
