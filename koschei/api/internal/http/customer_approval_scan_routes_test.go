package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/services"
)

func TestCustomerApprovalScanRejectsMissingContext(t *testing.T) {
	mux := http.NewServeMux()
	registerCustomerScanRoutes(mux)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scan/approval", strings.NewReader(`{"network":"ethereum-mainnet","token":"0x0000000000000000000000000000000000000001"}`))
	request.Header.Set("Content-Type", "application/json")

	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "approval_scan_context_required") {
		t.Fatalf("expected missing context error, got %s", recorder.Body.String())
	}
}

func TestCustomerApprovalScanRejectsUnsupportedNetwork(t *testing.T) {
	mux := http.NewServeMux()
	registerCustomerScanRoutes(mux)
	body := `{"network":"not-a-chain","token":"0x0000000000000000000000000000000000000001","owner":"0x0000000000000000000000000000000000000002","spender":"0x0000000000000000000000000000000000000003"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scan/approval", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "evm_allowance_unsupported_network") {
		t.Fatalf("expected unsupported network error, got %s", recorder.Body.String())
	}
}

func TestCustomerApprovalScanRequiresJSON(t *testing.T) {
	mux := http.NewServeMux()
	registerCustomerScanRoutes(mux)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scan/approval", strings.NewReader("network=ethereum-mainnet"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestCustomerApprovalScanRejectsGET(t *testing.T) {
	mux := http.NewServeMux()
	registerCustomerScanRoutes(mux)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/scan/approval", nil)

	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestValidateCustomerApprovalObservationAcceptsBoundObservedTuple(t *testing.T) {
	request := customerApprovalScanRequest{
		Network: "ethereum-mainnet",
		Token:   "0x0000000000000000000000000000000000000001",
		Owner:   "0x0000000000000000000000000000000000000002",
		Spender: "0x0000000000000000000000000000000000000003",
	}
	allowance := networktarget.EVMAllowanceProbeResult{
		Network:          request.Network,
		ChainID:          "0x1",
		ExpectedChainID:  "0x1",
		Token:            request.Token,
		Owner:            request.Owner,
		Spender:          request.Spender,
		Amount:           "0",
		EvidenceStatus:   "observed",
		LiveAvailability: "checked",
	}
	if err := validateCustomerApprovalObservation(request, allowance); err != nil {
		t.Fatalf("valid bound observation rejected: %v", err)
	}
}

func TestValidateCustomerApprovalObservationRejectsCrossNetworkSubstitution(t *testing.T) {
	request := customerApprovalScanRequest{
		Network: "ethereum-mainnet",
		Token:   "0x0000000000000000000000000000000000000001",
		Owner:   "0x0000000000000000000000000000000000000002",
		Spender: "0x0000000000000000000000000000000000000003",
	}
	allowance := networktarget.EVMAllowanceProbeResult{
		Network:          "base-mainnet",
		ChainID:          "0x2105",
		ExpectedChainID:  "0x2105",
		Token:            request.Token,
		Owner:            request.Owner,
		Spender:          request.Spender,
		Amount:           "1",
		EvidenceStatus:   "observed",
		LiveAvailability: "checked",
	}
	if err := validateCustomerApprovalObservation(request, allowance); err == nil {
		t.Fatal("expected cross-network allowance evidence to be rejected")
	}
}

func TestValidateCustomerApprovalObservationRejectsSpenderSubstitutionAndIncompleteEvidence(t *testing.T) {
	request := customerApprovalScanRequest{
		Network: "ethereum-mainnet",
		Token:   "0x0000000000000000000000000000000000000001",
		Owner:   "0x0000000000000000000000000000000000000002",
		Spender: "0x0000000000000000000000000000000000000003",
	}
	base := networktarget.EVMAllowanceProbeResult{
		Network:          request.Network,
		ChainID:          "0x1",
		ExpectedChainID:  "0x1",
		Token:            request.Token,
		Owner:            request.Owner,
		Spender:          "0x0000000000000000000000000000000000000004",
		Amount:           "1",
		EvidenceStatus:   "observed",
		LiveAvailability: "checked",
	}
	if err := validateCustomerApprovalObservation(request, base); err == nil {
		t.Fatal("expected substituted spender to be rejected")
	}
	base.Spender = request.Spender
	base.EvidenceStatus = "unverified"
	if err := validateCustomerApprovalObservation(request, base); err == nil {
		t.Fatal("expected incomplete approval evidence to be rejected")
	}
}

func TestCustomerApprovalResultWriterPinsNoStoreSchemaAndObservedTrust(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeCustomerApprovalScanResult(recorder, http.StatusOK, customerApprovalScanResult{
		Network:          "ethereum-mainnet",
		Token:            "0x0000000000000000000000000000000000000001",
		Owner:            "0x0000000000000000000000000000000000000002",
		Spender:          "0x0000000000000000000000000000000000000003",
		Amount:           "0",
		EvidenceStatus:   services.Web3TrustEvidenceObserved,
		LiveAvailability: "checked",
		Trust:            services.Web3TrustVector{Observed: true},
		Reasons:          []string{"CURRENT_ALLOWANCE_OBSERVED"},
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q", got)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type=%q", got)
	}
	var envelope customerApprovalScanEnvelope
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != customerApprovalScanSchemaVersion {
		t.Fatalf("schema=%q", envelope.SchemaVersion)
	}
	if envelope.Result.Amount != "0" || !envelope.Result.Trust.Observed || envelope.Result.EvidenceStatus != services.Web3TrustEvidenceObserved {
		t.Fatalf("unexpected observed envelope: %+v", envelope.Result)
	}
}

func TestCustomerApprovalErrorWriterCannotManufactureEvidence(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeCustomerApprovalScanError(recorder, http.StatusBadGateway, "evm_allowance_probe_unavailable")
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q", got)
	}
	var body map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["schema_version"] != customerApprovalScanSchemaVersion || body["analysis_performed"] != false || body["evidence_status"] != services.Web3TrustEvidenceUnverified {
		t.Fatalf("unexpected error envelope: %#v", body)
	}
	if body["error"] != "evm_allowance_probe_unavailable" {
		t.Fatalf("error=%v", body["error"])
	}
}
