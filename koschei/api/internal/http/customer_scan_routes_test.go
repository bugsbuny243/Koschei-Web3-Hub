package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"koschei/api/internal/services"
)

func TestCustomerScanEndpointRequiresNetworkForEVMAddress(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "scan.html"), []byte("scan"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewServer(nil, "", "", "", staticDir))
	defer srv.Close()

	requestBody := `{"target":"0x1111111111111111111111111111111111111111"}`
	resp, err := http.Post(srv.URL+"/api/scan", "application/json", strings.NewReader(requestBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var envelope customerScanEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != customerScanSchemaVersion {
		t.Fatalf("schema=%q", envelope.SchemaVersion)
	}
	if envelope.Result.Status != services.CustomerScanStatusNeedsContext || !envelope.Result.Target.RequiresNetwork {
		t.Fatalf("result=%+v", envelope.Result)
	}
	if envelope.Result.EvidenceStatus != services.Web3TrustEvidenceUnverified || envelope.Result.Trust.Observed || envelope.Result.Trust.Verified || envelope.Result.Trust.Finalized {
		t.Fatalf("network request manufactured evidence: %+v", envelope.Result)
	}
}

func TestCustomerScanEndpointRejectsUnknownJSONFields(t *testing.T) {
	mux := http.NewServeMux()
	registerCustomerScanRoutes(mux)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scan", strings.NewReader(`{"target":"0x1111111111111111111111111111111111111111","network":"ethereum-mainnet","unsafe":true}`))
	request.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestCustomerScanEndpointDoesNotClaimSolanaLiveEvidence(t *testing.T) {
	mux := http.NewServeMux()
	registerCustomerScanRoutes(mux)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scan", strings.NewReader(`{"target":"11111111111111111111111111111111","network":"solana-mainnet"}`))
	request.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotImplemented {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope customerScanEnvelope
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Result.Trust.Observed || envelope.Result.Trust.Verified || envelope.Result.Trust.Finalized {
		t.Fatalf("Solana syntax-only classification manufactured live evidence: %+v", envelope.Result)
	}
	found := false
	for _, reason := range envelope.Result.Reasons {
		if reason == "SOLANA_CUSTOMER_SCAN_NOT_CONNECTED" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing explicit Solana connection boundary: %+v", envelope.Result.Reasons)
	}
}

func TestProductionMuxExposesFabricNetworkCatalog(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "scan.html"), []byte("scan"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewServer(nil, "", "", "", staticDir))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/fabric/networks/catalog")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}
