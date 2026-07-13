package handlers

import (
	"testing"
	"time"

	"koschei/api/internal/services"
)

func TestActorDefenseTokenAccountOwners(t *testing.T) {
	keys := []string{"source-token-account", "destination-token-account"}
	meta := map[string]any{
		"preTokenBalances": []any{
			map[string]any{"accountIndex": float64(0), "owner": "actor-wallet", "mint": "mint-one"},
		},
		"postTokenBalances": []any{
			map[string]any{"accountIndex": float64(1), "owner": "recipient-wallet", "mint": "mint-one"},
		},
	}
	owners := actorDefenseTokenAccountOwners(meta, keys)
	if owners["source-token-account"].Owner != "actor-wallet" {
		t.Fatalf("source owner=%q", owners["source-token-account"].Owner)
	}
	if owners["destination-token-account"].Owner != "recipient-wallet" {
		t.Fatalf("destination owner=%q", owners["destination-token-account"].Owner)
	}
	if owners["destination-token-account"].Mint != "mint-one" {
		t.Fatalf("destination mint=%q", owners["destination-token-account"].Mint)
	}
}

func TestActorDefenseLiquidityRemovalFromParsedInstruction(t *testing.T) {
	message := map[string]any{"instructions": []any{
		map[string]any{"parsed": map[string]any{"type": "removeLiquidity", "info": map[string]any{}}},
	}}
	found, kinds := actorDefenseLiquidityRemoval(message, map[string]any{})
	if !found {
		t.Fatal("expected parsed remove-liquidity observation")
	}
	if len(kinds) != 1 || kinds[0] != "removeliquidity" {
		t.Fatalf("instruction kinds=%v", kinds)
	}
}

func TestActorDefenseLiquidityRemovalFromLogs(t *testing.T) {
	meta := map[string]any{"logMessages": []any{"Program log: remove_liquidity"}}
	found, _ := actorDefenseLiquidityRemoval(map[string]any{}, meta)
	if !found {
		t.Fatal("expected log-backed remove-liquidity observation")
	}
}

func TestActorDefenseTokenAmountCheckedTransfer(t *testing.T) {
	amount := actorDefenseTokenAmount(map[string]any{"tokenAmount": map[string]any{
		"uiAmountString": "42.125", "decimals": float64(6), "amount": "42125000",
	}})
	if amount != 42.125 {
		t.Fatalf("amount=%v", amount)
	}
}

func TestActorDefenseObservedAtUsesTransactionBlockTime(t *testing.T) {
	got := actorDefenseObservedAt(services.SolanaSignatureInfo{}, map[string]any{"blockTime": float64(1700000000)})
	want := time.Unix(1700000000, 0).UTC()
	if !got.Equal(want) {
		t.Fatalf("observed_at=%s want=%s", got, want)
	}
}
