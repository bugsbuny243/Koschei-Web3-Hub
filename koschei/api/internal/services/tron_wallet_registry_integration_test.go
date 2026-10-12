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
	requests := 0
	client := &http.Client{Transport: tronRegistryTransport(func(req *http.Request) (*http.Response, error) {
		requests++
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
	plan, err = registry.PrepareTronWalletPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Executable || plan.Route != UniversalDispatchTronWallet || !plan.EvidenceOnly || plan.VerdictAuthority != UniversalVerdictEvidenceOnly {
		t.Fatalf("Tron runtime plan must require collection: %+v", plan)
	}
	executable, err := registry.Collect(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if !executable.Executable || executable.Route != UniversalDispatchTronWallet || len(executable.Evidence) != 1 ||
		executable.Evidence[0].Status != IntelligenceEvidenceObserved {
		t.Fatalf("Tron evidence-only route failed: %+v", executable)
	}
	bounded, err := registry.CollectTronWalletPermissions(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if !bounded.Executable || len(bounded.Evidence) != 1 || bounded.Evidence[0].Status != IntelligenceEvidenceObserved {
		t.Fatalf("bounded Tron collection failed: %+v", bounded)
	}
	if bounded.Evidence[0].Network != "tron-mainnet" ||
		bounded.Evidence[0].SubjectID != plan.Subject.ID ||
		bounded.Evidence[0].Provenance != "radaradapter_read_only" ||
		bounded.Evidence[0].Attributes["account_state_height_verified"] != false {
		t.Fatalf("Tron bounded evidence lost provenance restrictions: %+v", bounded.Evidence[0])
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := registry.CollectTronWalletPermissions(canceled, plan); err != context.Canceled {
		t.Fatalf("canceled Tron collection should stop before execution: %v", err)
	}
	byAddress, err := registry.CollectTronWalletAddress(context.Background(), account)
	if err != nil {
		t.Fatal(err)
	}
	if !byAddress.Executable || len(byAddress.Evidence) != 1 ||
		byAddress.Evidence[0].Status != IntelligenceEvidenceObserved ||
		byAddress.Evidence[0].SubjectID != "tron-mainnet:account:"+account {
		t.Fatalf("Tron address bridge did not return subject-bound observed evidence: %+v", byAddress)
	}
	if _, err := registry.CollectTronWalletAddress(context.Background(), "not-a-tron-address"); err == nil {
		t.Fatal("Tron address bridge accepted invalid account syntax")
	}
	plan.VerdictAuthority = UniversalVerdictExistingRulesOnly
	beforeUnauthorized := requests
	if _, err := registry.Collect(context.Background(), plan); err == nil {
		t.Fatal("Tron route accepted grade authority")
	}
	if requests != beforeUnauthorized {
		t.Fatalf("unauthorized Tron plan contacted RPC: before=%d after=%d", beforeUnauthorized, requests)
	}
	plan.VerdictAuthority = UniversalVerdictEvidenceOnly
	plan.EvidenceOnly = false
	beforeUnauthorized = requests
	if _, err := registry.Collect(context.Background(), plan); err == nil {
		t.Fatal("Tron route accepted non-evidence-only plan")
	}
	if requests != beforeUnauthorized {
		t.Fatalf("non-evidence-only Tron plan contacted RPC: before=%d after=%d", beforeUnauthorized, requests)
	}
	plan.EvidenceOnly = true
	wrongNetwork := plan
	wrongNetwork.Subject.Network = "ethereum-mainnet"
	if _, err := registry.PrepareTronWalletPlan(wrongNetwork); err == nil {
		t.Fatal("Tron runtime plan accepted a foreign network")
	}
	wrongKind := plan
	wrongKind.Subject.Kind = "contract"
	if _, err := registry.PrepareTronWalletPlan(wrongKind); err == nil {
		t.Fatal("Tron runtime plan accepted a contract target")
	}
	duplicate, err := NewUniversalAdapterRegistry(adapter, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.HasRuntimeCollector(radaradapter.Target{Network: "tron-mainnet", Kind: "account", Raw: account}) {
		t.Fatal("ambiguous Tron collectors must not be advertised as available")
	}
	if _, err := duplicate.PrepareTronWalletPlan(plan); err == nil {
		t.Fatal("ambiguous Tron collectors must fail closed during plan preparation")
	}
	if _, err := (*UniversalAdapterRegistry)(nil).PrepareTronWalletPlan(plan); err == nil {
		t.Fatal("nil Tron registry must not prepare an executable plan")
	}
	if executable.Evidence[0].Status != IntelligenceEvidenceObserved {
		t.Fatal("Tron evidence must remain observed")
	}
	if executable.Executable && len(executable.Evidence) == 0 {
		t.Fatal("Tron route cannot claim execution without evidence")
	}
	if !registry.HasRuntimeCollector(radaradapter.Target{Network: "tron-mainnet", Kind: "account", Raw: account}) {
		t.Fatal("installed Tron collector not discoverable")
	}
}
