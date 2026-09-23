package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"koschei/api/internal/radarevent"
	"koschei/api/internal/securityevidence"
)

func TestGlobalRadarSuiNetworkEventProducesVerifiedEvent(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"chainIdentifier":"4btiuiMPvEENsttpZC7CZ53DruC3MAgfznDbASZ7DR6S"}}`))
	}))
	defer server.Close()
	t.Setenv("SUI_GRAPHQL_URL", server.URL)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/fabric/radar/network-event",
		strings.NewReader(`{"network":"sui-mainnet"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	globalRadarNetworkEventWithClient(response, request, server.Client())

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var envelope globalRadarProbeEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.RadarEvent.NetworkID != "sui-mainnet" ||
		envelope.RadarEvent.Kind != radarevent.KindNetworkHealth ||
		envelope.RadarEvent.State != securityevidence.StateVerified {
		t.Fatalf("unexpected radar event: %#v", envelope.RadarEvent)
	}
	if err := envelope.RadarEvent.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestGlobalRadarAptosNetworkEventProducesVerifiedEvent(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method=%s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"chain_id":1,"epoch":"42","ledger_version":"1234","ledger_timestamp":"1780000000000000","block_height":"1000","node_role":"full_node","git_hash":"abc123"}`))
	}))
	defer server.Close()
	t.Setenv("APTOS_REST_URL", server.URL)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/fabric/radar/network-event",
		strings.NewReader(`{"network":"aptos-mainnet"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	globalRadarNetworkEventWithClient(response, request, server.Client())

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var envelope globalRadarProbeEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.RadarEvent.NetworkID != "aptos-mainnet" ||
		envelope.RadarEvent.Kind != radarevent.KindNetworkHealth ||
		envelope.RadarEvent.State != securityevidence.StateVerified {
		t.Fatalf("unexpected radar event: %#v", envelope.RadarEvent)
	}
	if err := envelope.RadarEvent.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestGlobalRadarMoveNetworkEventFailsClosedWithoutConfiguration(t *testing.T) {
	t.Setenv("SUI_GRAPHQL_URL", "")
	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/fabric/radar/network-event",
		strings.NewReader(`{"network":"sui-mainnet"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	MountFabric(http.NotFoundHandler()).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable ||
		!strings.Contains(response.Body.String(), "move_identity_configuration_required") {
		t.Fatalf("unexpected fail-closed response: %d %s", response.Code, response.Body.String())
	}
}

func TestGlobalRadarNetworkEventRejectsUnknownFieldsAndGET(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/fabric/radar/network-event",
		strings.NewReader(`{"network":"sui-mainnet","address":"invented"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	MountFabric(http.NotFoundHandler()).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status=%d body=%s", response.Code, response.Body.String())
	}

	get := httptest.NewRecorder()
	MountFabric(http.NotFoundHandler()).ServeHTTP(
		get,
		httptest.NewRequest(http.MethodGet, "/fabric/radar/network-event", nil),
	)
	if get.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status=%d body=%s", get.Code, get.Body.String())
	}
}


func TestMoveDeploymentCatalogReportsConfigurationTruthfully(t *testing.T) {
	t.Setenv("SUI_GRAPHQL_URL", "")
	t.Setenv("APTOS_REST_URL", "")
	states := networkDeploymentCatalog()
	byID := map[string]networkDeploymentState{}
	for _, state := range states {
		byID[state.NetworkID] = state
	}
	for _, networkID := range []string{"sui-mainnet", "aptos-mainnet"} {
		state, ok := byID[networkID]
		if !ok {
			t.Fatalf("deployment catalog missing %s", networkID)
		}
		if state.CollectorRuntime != "configuration_required" ||
			state.LiveAvailability != "configuration_required" {
			t.Fatalf("%s state=%#v", networkID, state)
		}
	}

	t.Setenv("SUI_GRAPHQL_URL", "https://sui.example/graphql")
	t.Setenv("APTOS_REST_URL", "https://aptos.example/v1")
	states = networkDeploymentCatalog()
	byID = map[string]networkDeploymentState{}
	for _, state := range states {
		byID[state.NetworkID] = state
	}
	if byID["sui-mainnet"].CollectorRuntime != "graphql_configured" {
		t.Fatalf("sui state=%#v", byID["sui-mainnet"])
	}
	if byID["aptos-mainnet"].CollectorRuntime != "rest_configured" {
		t.Fatalf("aptos state=%#v", byID["aptos-mainnet"])
	}
}
