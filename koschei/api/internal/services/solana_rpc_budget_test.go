package services

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSolanaRPCBudgetAllowsConfiguredWindow(t *testing.T) {
	resetSolanaRPCCachesForTest()
	t.Setenv("SOLANA_RPC_BUDGET_ENABLED", "true")
	t.Setenv("SOLANA_RPC_BUDGET_WINDOW_SECONDS", "60")
	t.Setenv("SOLANA_RPC_BUDGET_MAX_REQUESTS", "2")
	if err := reserveSolanaRPCBudget(context.Background(), "getSignaturesForAddress"); err != nil {
		t.Fatalf("first budget reserve failed: %v", err)
	}
	if err := reserveSolanaRPCBudget(context.Background(), "getTransaction"); err != nil {
		t.Fatalf("second budget reserve failed: %v", err)
	}
	err := reserveSolanaRPCBudget(context.Background(), "getAccountInfo")
	if err == nil || !strings.Contains(err.Error(), "budget exceeded") {
		t.Fatalf("expected budget exceeded, got %v", err)
	}
}

func TestSolanaRPCBudgetDisabledByDefault(t *testing.T) {
	resetSolanaRPCCachesForTest()
	t.Setenv("SOLANA_RPC_BUDGET_ENABLED", "")
	t.Setenv("SOLANA_RPC_BUDGET_MAX_REQUESTS", "0")
	if err := reserveSolanaRPCBudget(context.Background(), "getSignaturesForAddress"); err != nil {
		t.Fatalf("disabled budget should not block: %v", err)
	}
}

func TestSolanaRPC429CooldownHasFloorAndCeiling(t *testing.T) {
	t.Setenv("SOLANA_RPC_429_COOLDOWN_SECONDS", "120")
	if got := solanaRPC429Cooldown(); got != 120*time.Second {
		t.Fatalf("cooldown = %s, want 120s", got)
	}
	t.Setenv("SOLANA_RPC_429_COOLDOWN_SECONDS", "5")
	if got := solanaRPC429Cooldown(); got != 30*time.Second {
		t.Fatalf("cooldown floor = %s, want 30s", got)
	}
	t.Setenv("SOLANA_RPC_429_COOLDOWN_SECONDS", "99999")
	if got := solanaRPC429Cooldown(); got != time.Hour {
		t.Fatalf("cooldown ceiling = %s, want 1h", got)
	}
}
