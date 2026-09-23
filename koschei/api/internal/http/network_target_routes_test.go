package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"koschei/api/internal/networktarget"
)

func targetRequest(body, contentType string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/fabric/networks/resolve", strings.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()
	MountFabric(http.NotFoundHandler()).ServeHTTP(response, request)
	return response
}

func TestNetworkTargetResolutionDoesNotRunAnalysis(t *testing.T) {
	response := targetRequest(`{"network":"ethereum-mainnet","address":"0x1111111111111111111111111111111111111111"}`, "application/json")
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", response.Code, response.Body.String())
	}
	var result networktarget.Resolution
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.AnalysisPerformed || result.EvidenceStatus != "unknown" || result.LiveAvailability != "not_checked" {
		t.Fatal("resolution fabricated live analysis")
	}
	if result.Network.CollectorStatus != "probe_ready" {
		t.Fatalf("implementation status=%q want probe_ready", result.Network.CollectorStatus)
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("network target responses must preserve transport protections")
	}
}

func TestNetworkTargetRejectsOversizeUnknownFieldsAndTrailingJSON(t *testing.T) {
	for _, body := range []string{
		`{"network":"ethereum-mainnet","address":"x","signature":"invented"}`,
		`{"network":"ethereum-mainnet","address":"x"} {}`,
		`{"network":"solana-mainnet","network":"ethereum-mainnet","address":"x"}`,
		`{"network":"solana-mainnet","address":"` + strings.Repeat("1", 2000) + `"}`,
	} {
		if response := targetRequest(body, "application/json"); response.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status %d", response.Code)
		}
	}
}

func TestNetworkCoverageFormAndCatalogAreVisible(t *testing.T) {
	mounted := MountFabric(http.NotFoundHandler())
	page := httptest.NewRecorder()
	mounted.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/fabric/networks", nil))
	for _, text := range []string{"Network coverage", "Select a network", "Ethereum", "Bitcoin", "Probe implemented; deployment and live health not checked"} {
		if !strings.Contains(page.Body.String(), text) {
			t.Fatalf("coverage page missing %q", text)
		}
	}
	if strings.Contains(page.Body.String(), "Bitcoin address parsing and evidence collection remain pending") {
		t.Fatal("coverage page still claims implemented Bitcoin parsing/probe is pending")
	}
	values := url.Values{"network": {"solana-mainnet"}, "address": {strings.Repeat("1", 32)}}
	response := targetRequest(values.Encode(), "application/x-www-form-urlencoded")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Solana address format recognized") {
		t.Fatalf("coverage form failed: %d %s", response.Code, response.Body.String())
	}
	values.Add("network", "ethereum-mainnet")
	if response := targetRequest(values.Encode(), "application/x-www-form-urlencoded"); response.Code != http.StatusBadRequest {
		t.Fatal("duplicate form network must be rejected")
	}
	catalog := httptest.NewRecorder()
	mounted.ServeHTTP(catalog, httptest.NewRequest(http.MethodGet, "/fabric/networks/catalog", nil))
	var payload map[string]any
	if err := json.Unmarshal(catalog.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	telemetry, ok := payload["telemetry"].(map[string]any)
	if !ok || payload["live_availability"] != "not_checked" {
		t.Fatal("coverage telemetry or availability boundary missing")
	}
	for _, key := range []string{
		"evm_transaction_probe_requests",
		"evm_transaction_probe_success",
		"evm_transaction_probe_not_found",
		"evm_transaction_probe_failed",
		"evm_transaction_probe_pending",
		"evm_transaction_probe_unknown",
		"evm_transaction_probe_latency_ms_total",
	} {
		if _, exists := telemetry[key]; !exists {
			t.Fatalf("operator telemetry missing %q: %#v", key, telemetry)
		}
	}
}

func TestNetworkResolutionHasNoImplicitNetworkAndNoGETMutation(t *testing.T) {
	response := targetRequest(`{"address":"0x1111111111111111111111111111111111111111"}`, "application/json")
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "network_required") {
		t.Fatal("address was allowed to choose its own network")
	}
	get := httptest.NewRecorder()
	MountFabric(http.NotFoundHandler()).ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/fabric/networks/resolve", nil))
	if get.Code != http.StatusMethodNotAllowed {
		t.Fatal("resolver must keep target addresses out of GET URLs")
	}
}


func TestGlobalRadarCapabilityCatalogIsExplicitAndFailClosed(t *testing.T) {
	mounted := MountFabric(http.NotFoundHandler())
	response := httptest.NewRecorder()
	mounted.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/fabric/networks/radar", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("global radar capability response must preserve transport protections")
	}

	var payload struct {
		SchemaVersion    string                          `json:"schema_version"`
		Scope            string                          `json:"scope"`
		LiveAvailability string                          `json:"live_availability"`
		Networks         []networktarget.RadarCapability `json:"networks"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.SchemaVersion != networktarget.GlobalRadarCapabilitySchemaVersion ||
		payload.Scope != "global-multi-chain-radar" ||
		payload.LiveAvailability != "not_checked" {
		t.Fatalf("unexpected radar capability boundary: %#v", payload)
	}
	if len(payload.Networks) != len(networktarget.Catalog()) {
		t.Fatalf("radar capability count=%d catalog=%d", len(payload.Networks), len(networktarget.Catalog()))
	}
	for _, capability := range payload.Networks {
		if capability.Family == "move" && capability.TransactionObservation != "not_connected" {
			t.Fatalf("%s move transaction dispatcher was overclaimed: %#v", capability.NetworkID, capability)
		}
	}
}
