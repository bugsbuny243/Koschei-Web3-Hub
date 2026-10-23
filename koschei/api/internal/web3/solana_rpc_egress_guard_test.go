package web3

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"koschei/api/internal/outboundhttp"
)

func TestCanonicalSolanaRPCRejectsPrivateProductionEndpoint(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	var hits atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var result map[string]any
	err := callSolanaRPC(context.Background(), &http.Client{Timeout: time.Second}, server.URL, "getAccountInfo", []byte("{}"), &result)
	if !errors.Is(err, outboundhttp.ErrUnsafeDestination) {
		t.Fatalf("private production RPC endpoint must be denied before dial: %v", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("unsafe provider unexpectedly handled %d request(s)", hits.Load())
	}
}

func TestCanonicalSolanaRPCRejectsCredentialBearingEndpoint(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	var result map[string]any
	err := callSolanaRPC(context.Background(), &http.Client{Timeout: time.Second}, "https://user:secret@rpc.example", "getAccountInfo", []byte("{}"), &result)
	if !errors.Is(err, outboundhttp.ErrUnsafeDestination) {
		t.Fatalf("embedded URL credentials must be rejected: %v", err)
	}
}
