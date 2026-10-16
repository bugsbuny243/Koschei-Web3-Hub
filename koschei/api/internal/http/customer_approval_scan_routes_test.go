package http

import (
	"encoding/json"
	"fmt"
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

func TestCustomerApprovalScanInvalidEVMAddressReturns400WithoutRPC(t *testing.T) {
	validToken := "0x1111111111111111111111111111111111111111"
	validOwner := "0x2222222222222222222222222222222222222222"
	validSpender := "0x3333333333333333333333333333333333333333"
	for _, tc := range []struct {
		name, token, owner, spender, code string
	}{
		{"invalid_token", "0x1234", validOwner, validSpender, "evm_allowance_token_invalid"},
		{"nonhex_owner", validToken, "0x" + strings.Repeat("z", 40), validSpender, "evm_allowance_owner_invalid"},
		{"short_spender", validToken, validOwner, "0x1234", "evm_allowance_spender_invalid"},
		{"embedded_space", validToken, validOwner, "0x" + strings.Repeat("1", 20) + " " + strings.Repeat("2", 19), "evm_allowance_spender_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			registerCustomerScanRoutes(mux)
			body := fmt.Sprintf(`{"network":"ethereum-mainnet","token":"%s","owner":"%s","spender":"%s"}`, tc.token, tc.owner, tc.spender)
			request := httptest.NewRequest(http.MethodPost, "/api/scan/approval", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, request)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("malformed user input should fail before RPC, got %d: %s", rec.Code, rec.Body.String())
			}
			var response struct {
				Error             string `json:"error"`
				EvidenceStatus    string `json:"evidence_status"`
				AnalysisPerformed bool   `json:"analysis_performed"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if response.Error != tc.code || response.EvidenceStatus != "unverified" || response.AnalysisPerformed {
				t.Fatalf("invalid address promoted to RPC or verified evidence: %+v", response)
			}
		})
	}
}
