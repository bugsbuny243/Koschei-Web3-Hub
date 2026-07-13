package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicScanHistoryRequiresValidMint(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/public/scan-history?mint=bad", nil)
	res := httptest.NewRecorder()
	h.PublicScanHistory(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestPublicScanHistoryWithoutDatabaseReturnsEmptyHistory(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/public/scan-history?mint=11111111111111111111111111111111", nil)
	res := httptest.NewRecorder()
	h.PublicScanHistory(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if body := res.Body.String(); body == "" || body == "{}\n" {
		t.Fatalf("unexpected empty response: %q", body)
	}
}
