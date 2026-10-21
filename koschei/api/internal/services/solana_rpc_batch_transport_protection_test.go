package services

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"koschei/api/internal/outboundhttp"
)

func TestSolanaRPCBatchRejectsPrivateProductionDestinationsBeforeDial(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("SOLANA_RPC_BUDGET_ENABLED", "false")
	t.Setenv("SOLANA_RPC_BATCH_MODE", "batch")

	var attempted atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempted.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	result, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"test-sig"})
	if !errors.Is(err, outboundhttp.ErrUnsafeDestination) {
		t.Fatalf("private production batch endpoint must fail via outbound boundary: err=%v", err)
	}
	if len(result) != 0 {
		t.Fatalf("blocked private endpoint returned evidence: %#v", result)
	}
	if calls := attempted.Load(); calls != 0 {
		t.Fatalf("private endpoint was reached %d times", calls)
	}
}

func TestSolanaRPCBatchRejectsCredentialBearingProductionURLs(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("SOLANA_RPC_BUDGET_ENABLED", "false")
	t.Setenv("SOLANA_RPC_BATCH_MODE", "batch")

	result, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), "https://user:secret@rpc.example", []string{"test-sig"})
	if !errors.Is(err, outboundhttp.ErrUnsafeDestination) {
		t.Fatalf("URL with embedded credentials must fail before network access: err=%v", err)
	}
	if len(result) != 0 {
		t.Fatalf("unsafe URL returned evidence: %#v", result)
	}
}
