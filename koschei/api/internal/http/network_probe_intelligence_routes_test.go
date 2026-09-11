package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNetworkProbeIntelligenceProjectsVerifiedEVMObservation(t *testing.T) {
	rpc := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			JSONRPC string `json:"jsonrpc"`
			ID      int    `json:"id"`
			Method  string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode rpc request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "eth_chainId":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
		case "eth_getCode":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":"0x60016000"}`))
		default:
			http.Error(w, "unexpected method", http.StatusBadRequest)
		}
	}))
	defer rpc.Close()
	t.Setenv("ETHEREUM_RPC_URL", rpc.URL)

	request := httptest.NewRequest(http.MethodPost, "/fabric/networks/probe/intelligence", strings.NewReader(`{"network":"ethereum-mainnet","address":"0x1111111111111111111111111111111111111111"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	networkTargetProbeWithClient(response, request, rpc.Client())
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}

	var payload struct {
		SchemaVersion string `json:"schema_version"`
		Probe         struct {
			EvidenceStatus string `json:"evidence_status"`
			ChainID        string `json:"chain_id"`
		} `json:"probe"`
		Intelligence struct {
			Subject struct {
				ChainFamily string `json:"chain_family"`
				Network     string `json:"network"`
			} `json:"subject"`
			Evidence struct {
				Status     string  `json:"status"`
				Source     string  `json:"source"`
				Confidence float64 `json:"confidence"`
			} `json:"evidence"`
		} `json:"intelligence"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode intelligence response: %v", err)
	}
	if payload.SchemaVersion != networkProbeIntelligenceSchemaVersion {
		t.Fatalf("schema_version=%q", payload.SchemaVersion)
	}
	if payload.Probe.EvidenceStatus != "observed" || payload.Probe.ChainID != "0x1" {
		t.Fatalf("unexpected probe: %#v", payload.Probe)
	}
	if payload.Intelligence.Subject.ChainFamily != "evm" || payload.Intelligence.Subject.Network != "ethereum-mainnet" {
		t.Fatalf("unexpected subject: %#v", payload.Intelligence.Subject)
	}
	if payload.Intelligence.Evidence.Status != "observed" || payload.Intelligence.Evidence.Source != "evm_rpc_probe" || payload.Intelligence.Evidence.Confidence != 0.8 {
		t.Fatalf("unexpected evidence: %#v", payload.Intelligence.Evidence)
	}
}

func TestExistingNetworkProbeResponseContractStaysFlat(t *testing.T) {
	rpc := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode rpc request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		if request.Method == "eth_chainId" {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
			return
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":"0x"}`))
	}))
	defer rpc.Close()
	t.Setenv("ETHEREUM_RPC_URL", rpc.URL)

	request := httptest.NewRequest(http.MethodPost, "/fabric/networks/probe", strings.NewReader(`{"network":"ethereum-mainnet","address":"0x1111111111111111111111111111111111111111"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	networkTargetProbeWithClient(response, request, rpc.Client())
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if _, exists := payload["intelligence"]; exists {
		t.Fatalf("legacy probe contract changed: %s", response.Body.String())
	}
	if payload["evidence_status"] != "observed" || payload["analysis_performed"] != true {
		t.Fatalf("legacy probe evidence changed: %#v", payload)
	}
}
