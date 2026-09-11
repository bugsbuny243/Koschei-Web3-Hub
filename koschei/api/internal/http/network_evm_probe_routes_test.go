package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEVMProbeFailsClosedWhenRPCIsNotConfigured(t *testing.T) {
	for _, key := range []string{"ETHEREUM_RPC_URL", "BASE_RPC_URL", "ARBITRUM_RPC_URL", "OPTIMISM_RPC_URL"} {
		t.Setenv(key, "")
	}
	request := httptest.NewRequest(http.MethodPost, "/fabric/networks/probe", strings.NewReader(`{"network":"ethereum-mainnet","address":"0x1111111111111111111111111111111111111111"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	MountFabric(http.NotFoundHandler()).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("probe status=%d body=%s", response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["live_availability"] != "configuration_required" || payload["analysis_performed"] != false || payload["evidence_status"] != "unknown" {
		t.Fatalf("probe fabricated readiness: %#v", payload)
	}
}

func TestEVMDeploymentSurfaceDoesNotExposeRPCURL(t *testing.T) {
	secretEndpoint := "https://rpc.example.invalid/private-secret-token"
	t.Setenv("ETHEREUM_RPC_URL", secretEndpoint)
	for _, key := range []string{"BASE_RPC_URL", "ARBITRUM_RPC_URL", "OPTIMISM_RPC_URL"} {
		t.Setenv(key, "")
	}
	mounted := MountFabric(http.NotFoundHandler())

	catalog := httptest.NewRecorder()
	mounted.ServeHTTP(catalog, httptest.NewRequest(http.MethodGet, "/fabric/networks/deployment", nil))
	if catalog.Code != http.StatusOK || strings.Contains(catalog.Body.String(), secretEndpoint) || strings.Contains(catalog.Body.String(), "private-secret-token") {
		t.Fatalf("deployment surface leaked RPC configuration: %d %s", catalog.Code, catalog.Body.String())
	}
	if !strings.Contains(catalog.Body.String(), `"collector_runtime":"rpc_configured"`) || !strings.Contains(catalog.Body.String(), `"collector_runtime":"configuration_required"`) {
		t.Fatalf("deployment state missing configured/unconfigured truth: %s", catalog.Body.String())
	}

	page := httptest.NewRecorder()
	mounted.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/fabric/networks/live", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Live EVM evidence") || strings.Contains(page.Body.String(), secretEndpoint) {
		t.Fatalf("live collector page contract failed: %d %s", page.Code, page.Body.String())
	}
}

func TestEVMProbeDoesNotAcceptNonEVMNetwork(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/fabric/networks/probe", strings.NewReader(`{"network":"solana-mainnet","address":"11111111111111111111111111111111"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	MountFabric(http.NotFoundHandler()).ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "live_probe_not_supported_for_network") {
		t.Fatalf("non-EVM probe status=%d body=%s", response.Code, response.Body.String())
	}
}
