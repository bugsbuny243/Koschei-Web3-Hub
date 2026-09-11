package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNetworkDeploymentCatalogReportsBitcoinConfigurationRequired(t *testing.T) {
	t.Setenv("BITCOIN_ESPLORA_URL", "")
	for _, state := range networkDeploymentCatalog() {
		if state.NetworkID != "bitcoin-mainnet" {
			continue
		}
		if state.CollectorRuntime != "configuration_required" {
			t.Fatalf("bitcoin collector_runtime=%q", state.CollectorRuntime)
		}
		if state.LiveAvailability != "configuration_required" {
			t.Fatalf("bitcoin live_availability=%q", state.LiveAvailability)
		}
		return
	}
	t.Fatal("bitcoin-mainnet missing from deployment catalog")
}

func TestNetworkDeploymentCatalogDoesNotExposeBitcoinEndpoint(t *testing.T) {
	secretEndpoint := "https://bitcoin.example.invalid/private-token"
	t.Setenv("BITCOIN_ESPLORA_URL", secretEndpoint)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/fabric/networks/deployment", nil)
	networkDeploymentCatalogHandler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), secretEndpoint) || strings.Contains(recorder.Body.String(), "private-token") {
		t.Fatalf("deployment response leaked Bitcoin endpoint: %s", recorder.Body.String())
	}
	var payload struct {
		Networks []networkDeploymentState `json:"networks"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode deployment response: %v", err)
	}
	for _, state := range payload.Networks {
		if state.NetworkID == "bitcoin-mainnet" && state.CollectorRuntime != "esplora_configured" {
			t.Fatalf("bitcoin collector_runtime=%q", state.CollectorRuntime)
		}
	}
}
