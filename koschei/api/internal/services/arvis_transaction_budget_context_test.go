package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCollectArvisTransactionEvidenceContextBypassesExhaustedBackgroundBudget(t *testing.T) {
	t.Setenv("SOLANA_RPC_BUDGET_ENABLED", "true")
	t.Setenv("SOLANA_RPC_LIMIT_SAVER_ENABLED", "false")
	t.Setenv("SOLANA_RPC_BUDGET_MAX_REQUESTS", "1")
	t.Setenv("SOLANA_RPC_BUDGET_WINDOW_SECONDS", "3600")
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "0")
	t.Setenv("SOLANA_RPC_MAX_429_RETRIES", "0")

	resetSolanaRPCCachesForTest()
	t.Cleanup(resetSolanaRPCCachesForTest)

	if err := reserveSolanaRPCBudget(context.Background(), "background-prime"); err != nil {
		t.Fatalf("prime background budget: %v", err)
	}
	if err := reserveSolanaRPCBudget(context.Background(), "background-blocked"); err == nil {
		t.Fatal("expected shared background RPC budget to be exhausted")
	} else if _, ok := solanaRPCBudgetResetAt(err); !ok {
		t.Fatalf("expected budget exhaustion error, got %v", err)
	}

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("X-Koschei-RPC-Method"); got != "getTransaction" {
			t.Errorf("unexpected RPC method header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"slot":      42,
				"blockTime": 1_700_000_000,
				"transaction": map[string]any{
					"message": map[string]any{
						"accountKeys": []any{
							map[string]any{
								"pubkey":   "11111111111111111111111111111111",
								"signer":   true,
								"writable": true,
							},
						},
						"instructions": []any{},
					},
				},
				"meta": map[string]any{
					"fee":                  5000,
					"computeUnitsConsumed": 1,
					"preBalances":          []any{10000},
					"postBalances":         []any{5000},
					"innerInstructions":    []any{},
					"preTokenBalances":     []any{},
					"postTokenBalances":    []any{},
					"logMessages":          []any{},
				},
			},
		})
	}))
	defer server.Close()
	t.Setenv("SOLANA_RPC_URL", server.URL)

	signature := strings.Repeat("1", 88)
	ctx := WithInteractiveSolanaRPCBudget(context.Background())
	evidence := collectArvisTransactionEvidenceContext(ctx, SecurityRadarRequest{
		Target:  signature,
		Network: "solana-mainnet",
		Mode:    "owner_full_scan",
	}, nil)

	if !evidence.Available {
		t.Fatalf("interactive transaction enrichment should bypass exhausted background budget; errors=%v", evidence.Errors)
	}
	if calls != 1 {
		t.Fatalf("expected one upstream getTransaction call, got %d", calls)
	}
	for _, item := range evidence.Errors {
		if strings.Contains(strings.ToLower(item), "budget exceeded") {
			t.Fatalf("interactive enrichment leaked into background RPC budget: %v", evidence.Errors)
		}
	}
}
