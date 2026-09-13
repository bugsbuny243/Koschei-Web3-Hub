package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
