package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicTransactionSimulateRequiresTransaction(t *testing.T) {
	h := &Handler{Limiter: NewLimiter()}
	req := httptest.NewRequest(http.MethodPost, "/api/public/transaction-simulate", bytes.NewBufferString(`{"network":"solana-mainnet"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	h.PublicTransactionSimulate(res, req)
	if res.Code != http.StatusBadRequest || !strings.Contains(res.Body.String(), "transaction_required") {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
}

func TestPublicTransactionSimulateRejectsInvalidBase64(t *testing.T) {
	h := &Handler{Limiter: NewLimiter()}
	req := httptest.NewRequest(http.MethodPost, "/api/public/transaction-simulate", bytes.NewBufferString(`{"transaction":"not-base64","encoding":"base64","network":"solana-mainnet"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	h.PublicTransactionSimulate(res, req)
	if res.Code != http.StatusBadRequest || !strings.Contains(res.Body.String(), "invalid_transaction") {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
}
