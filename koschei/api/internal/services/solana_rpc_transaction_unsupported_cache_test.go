package services

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSolanaTransactionVersionFailureIsCachedPerSignature(t *testing.T) {
	configureSolanaTransactionSingleflightTest(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32015,"message":"Transaction version (1) is not supported by the requesting client"}}`)
	}))
	defer server.Close()
	for i := 0; i < 2; i++ {
		tx, err := SolanaGetTransactionJSONParsed(context.Background(), server.URL, "unsupported-v1")
		if tx != nil || err == nil || !strings.Contains(err.Error(), "-32015") {
			t.Fatalf("unsupported version must fail closed: tx=%v err=%v", tx, err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("same signature burned %d RPC calls, want 1", got)
	}
	// Do not suppress an independent signature.
	if _, err := SolanaGetTransactionJSONParsed(context.Background(), server.URL, "another-version-1"); err == nil {
		t.Fatal("other unsupported signature must remain an error")
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("different signatures must be checked independently; calls=%d", got)
	}
}

func TestSolanaTransactionVersionCacheDoesNotHideTransientErrorsOrSuccess(t *testing.T) {
	configureSolanaTransactionSingleflightTest(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32005,"message":"node is behind"}}`)
			return
		}
		writeTransactionResult(w, "eventually-readable")
	}))
	defer server.Close()
	if _, err := SolanaGetTransactionJSONParsed(context.Background(), server.URL, "temporary-failure"); err == nil {
		t.Fatal("first call should fail")
	}
	got, err := SolanaGetTransactionJSONParsed(context.Background(), server.URL, "temporary-failure")
	if err != nil || got["signature"] != "eventually-readable" || calls.Load() != 2 {
		t.Fatalf("temporary failure must be retried: result=%v err=%v calls=%d", got, err, calls.Load())
	}
}

func TestSolanaUnsupportedVersionCacheIsBounded(t *testing.T) {
	configureSolanaTransactionSingleflightTest(t)
	for i := 0; i < solanaUnsupportedTransactionMaxEntries+50; i++ {
		cacheSolanaUnsupportedTransactionVersion(fmt.Sprintf("hash-%d", i))
	}
	solanaUnsupportedTransactionVersions.Lock()
	n := len(solanaUnsupportedTransactionVersions.items)
	solanaUnsupportedTransactionVersions.Unlock()
	if n > solanaUnsupportedTransactionMaxEntries {
		t.Fatalf("unsupported transaction cache unbounded: %d", n)
	}
}
