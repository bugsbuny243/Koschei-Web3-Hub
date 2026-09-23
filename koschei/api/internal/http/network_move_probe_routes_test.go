package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMoveProbeRequiresConfiguredIdentityEndpoint(t *testing.T) {
	address := "0x" + strings.Repeat("11", 32)
	for _, tc := range []struct {
		network string
		envKey  string
	}{
		{network: "sui-mainnet", envKey: "SUI_GRAPHQL_URL"},
		{network: "aptos-mainnet", envKey: "APTOS_REST_URL"},
	} {
		t.Run(tc.network, func(t *testing.T) {
			t.Setenv(tc.envKey, "")
			request := httptest.NewRequest(
				http.MethodPost,
				"/fabric/networks/probe",
				strings.NewReader(`{"network":"`+tc.network+`","address":"`+address+`"}`),
			)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			networkTargetProbe(response, request)

			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), `"error":"move_identity_configuration_required"`) {
				t.Fatalf("unexpected response: %s", response.Body.String())
			}
			if !strings.Contains(response.Body.String(), `"live_availability":"configuration_required"`) {
				t.Fatalf("configuration state missing: %s", response.Body.String())
			}
		})
	}
}

func TestSuiMoveProbeIntelligenceProjectsVerifiedNetworkIdentityOnly(t *testing.T) {
	address := "0x" + strings.Repeat("22", 32)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["query"] != "{ chainIdentifier }" {
			t.Fatalf("query=%q", body["query"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"chainIdentifier": "4btiuiMPvEENsttpZC7CZ53DruC3MAgfznDbASZ7DR6S",
			},
		})
	}))
	defer server.Close()
	t.Setenv("SUI_GRAPHQL_URL", server.URL)

	request := httptest.NewRequest(
		http.MethodPost,
		"/fabric/networks/probe/intelligence",
		strings.NewReader(`{"network":"sui-mainnet","address":"`+address+`"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	networkTargetProbeWithClient(response, request, server.Client())

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}

	var payload struct {
		Probe struct {
			ChainIdentifier  string `json:"chain_identifier"`
			AnalysisPerformed bool  `json:"analysis_performed"`
		} `json:"probe"`
		Intelligence struct {
			Subject struct {
				ChainFamily string `json:"chain_family"`
				Chain       string `json:"chain"`
				Network     string `json:"network"`
			} `json:"subject"`
			Evidence struct {
				Status     string         `json:"status"`
				Source     string         `json:"source"`
				Confidence float64        `json:"confidence"`
				Attributes map[string]any `json:"attributes"`
			} `json:"evidence"`
		} `json:"intelligence"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Probe.ChainIdentifier == "" || !payload.Probe.AnalysisPerformed {
		t.Fatalf("unexpected Sui probe: %#v", payload.Probe)
	}
	if payload.Intelligence.Subject.ChainFamily != "move" ||
		payload.Intelligence.Subject.Chain != "sui" ||
		payload.Intelligence.Subject.Network != "sui-mainnet" {
		t.Fatalf("unexpected Sui subject: %#v", payload.Intelligence.Subject)
	}
	if payload.Intelligence.Evidence.Status != "verified" ||
		payload.Intelligence.Evidence.Source != "sui_graphql_identity_probe" ||
		payload.Intelligence.Evidence.Confidence != 1 {
		t.Fatalf("unexpected Sui evidence: %#v", payload.Intelligence.Evidence)
	}
	if observed, ok := payload.Intelligence.Evidence.Attributes["address_state_observed"].(bool); !ok || observed {
		t.Fatalf("Sui identity probe overclaimed address state: %#v", payload.Intelligence.Evidence.Attributes)
	}
}

func TestAptosMoveProbeIntelligenceProjectsLedgerIdentityOnly(t *testing.T) {
	address := "0x" + strings.Repeat("33", 32)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method=%s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"chain_id":         1,
			"epoch":            "44",
			"ledger_version":   "5500",
			"ledger_timestamp": "1700000000000000",
			"block_height":     "777",
			"node_role":        "full_node",
			"git_hash":         "abcdef",
		})
	}))
	defer server.Close()
	t.Setenv("APTOS_REST_URL", server.URL)

	request := httptest.NewRequest(
		http.MethodPost,
		"/fabric/networks/probe/intelligence",
		strings.NewReader(`{"network":"aptos-mainnet","address":"`+address+`"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	networkTargetProbeWithClient(response, request, server.Client())

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}

	var payload struct {
		Probe struct {
			ChainID       uint8  `json:"chain_id"`
			LedgerVersion uint64 `json:"ledger_version"`
			BlockHeight   uint64 `json:"block_height"`
		} `json:"probe"`
		Intelligence struct {
			Subject struct {
				ChainFamily string `json:"chain_family"`
				Chain       string `json:"chain"`
				Network     string `json:"network"`
			} `json:"subject"`
			Evidence struct {
				Status     string         `json:"status"`
				Source     string         `json:"source"`
				Confidence float64        `json:"confidence"`
				Attributes map[string]any `json:"attributes"`
			} `json:"evidence"`
		} `json:"intelligence"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Probe.ChainID != 1 || payload.Probe.LedgerVersion != 5500 || payload.Probe.BlockHeight != 777 {
		t.Fatalf("unexpected Aptos probe: %#v", payload.Probe)
	}
	if payload.Intelligence.Subject.ChainFamily != "move" ||
		payload.Intelligence.Subject.Chain != "aptos" ||
		payload.Intelligence.Subject.Network != "aptos-mainnet" {
		t.Fatalf("unexpected Aptos subject: %#v", payload.Intelligence.Subject)
	}
	if payload.Intelligence.Evidence.Status != "verified" ||
		payload.Intelligence.Evidence.Source != "aptos_rest_identity_probe" ||
		payload.Intelligence.Evidence.Confidence != 1 {
		t.Fatalf("unexpected Aptos evidence: %#v", payload.Intelligence.Evidence)
	}
	if observed, ok := payload.Intelligence.Evidence.Attributes["address_state_observed"].(bool); !ok || observed {
		t.Fatalf("Aptos identity probe overclaimed address state: %#v", payload.Intelligence.Evidence.Attributes)
	}
}

func TestMoveDeploymentCatalogDoesNotExposeConfiguredEndpoint(t *testing.T) {
	secretEndpoint := "https://move.example.invalid/private-secret-token"
	t.Setenv("SUI_GRAPHQL_URL", secretEndpoint)
	t.Setenv("APTOS_REST_URL", "")

	response := httptest.NewRecorder()
	MountFabric(http.NotFoundHandler()).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/fabric/networks/deployment", nil),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), secretEndpoint) || strings.Contains(response.Body.String(), "private-secret-token") {
		t.Fatalf("deployment catalog leaked Move endpoint: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"network_id":"sui-mainnet","collector_runtime":"identity_probe_configured"`) {
		t.Fatalf("Sui configured state missing: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"network_id":"aptos-mainnet","collector_runtime":"configuration_required"`) {
		t.Fatalf("Aptos configuration-required state missing: %s", response.Body.String())
	}
}
