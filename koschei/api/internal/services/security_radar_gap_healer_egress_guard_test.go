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

func TestSecurityRadarGapHealerRejectsPrivateProductionEndpoint(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SOLANA_RPC_BUDGET_ENABLED", "false")
	var hits atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	healer := &securityRadarGapHealer{RPCURL: server.URL}
	rows, err := healer.fetchSignaturePage(context.Background(), "test-program", "", 2)
	if !errors.Is(err, outboundhttp.ErrUnsafeDestination) {
		t.Fatalf("private gap-healer RPC endpoint must be denied before dial: %v", err)
	}
	if len(rows) != 0 || hits.Load() != 0 {
		t.Fatalf("unsafe endpoint provided data or was contacted: rows=%v hits=%d", rows, hits.Load())
	}
}
