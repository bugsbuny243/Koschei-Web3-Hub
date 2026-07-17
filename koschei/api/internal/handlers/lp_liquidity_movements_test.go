package handlers

import (
	"testing"

	"koschei/api/internal/services"
)

func liquidityMovementTestTransaction(lp services.LPControlEvidence, actor string, tokenPre, tokenPost, quotePre, quotePost float64, instruction string) services.SolanaTransactionResult {
	keys := []any{
		map[string]any{"pubkey": actor, "signer": true},
		map[string]any{"pubkey": lp.PoolAddress, "signer": false},
		map[string]any{"pubkey": lp.PoolProgram, "signer": false},
		map[string]any{"pubkey": lp.TokenVault, "signer": false},
		map[string]any{"pubkey": lp.QuoteVault, "signer": false},
	}
	balances := func(token, quote float64) []any {
		return []any{
			map[string]any{"accountIndex": float64(3), "mint": lp.TokenMint, "uiTokenAmount": map[string]any{"uiAmount": token}},
			map[string]any{"accountIndex": float64(4), "mint": lp.QuoteMint, "uiTokenAmount": map[string]any{"uiAmount": quote}},
		}
	}
	return services.SolanaTransactionResult{
		"blockTime": float64(1_752_739_200),
		"meta": map[string]any{
			"err": nil,
			"preTokenBalances": balances(tokenPre, quotePre),
			"postTokenBalances": balances(tokenPost, quotePost),
			"logMessages": []any{"Program log: Instruction: " + instruction},
			"innerInstructions": []any{},
		},
		"transaction": map[string]any{"message": map[string]any{
			"accountKeys": keys,
			"instructions": []any{map[string]any{"parsed": map[string]any{"type": instruction, "info": map[string]any{}}}},
		}},
	}
}

func liquidityMovementTestLP() services.LPControlEvidence {
	return services.LPControlEvidence{
		PoolAddress: "Pool111", PoolProgram: pumpSwapProgram,
		TokenMint: "Mint111", QuoteMint: "Quote111", TokenVault: "TokenVault111", QuoteVault: "QuoteVault111",
	}
}

func TestParseLiquidityMovementDetectsAddFromSameDirectionVaultDeltas(t *testing.T) {
	lp := liquidityMovementTestLP()
	tx := liquidityMovementTestTransaction(lp, "Actor111", 100, 150, 20, 30, "deposit")
	movement, ok := parseLiquidityMovement(lp, services.SolanaSignatureInfo{Signature: "AddSig111", Slot: 1200}, tx)
	if !ok { t.Fatal("add liquidity was not returned") }
	if movement.Kind != "add_liquidity" || movement.TokenDelta != 50 || movement.QuoteDelta != 10 || movement.ActorWallet != "Actor111" {
		t.Fatalf("movement=%#v", movement)
	}
	if movement.EvidenceKey != "liquidity_movement:add_liquidity:AddSig111:1200" { t.Fatalf("key=%q", movement.EvidenceKey) }
}

func TestParseLiquidityMovementDetectsRemoveFromSameDirectionVaultDeltas(t *testing.T) {
	lp := liquidityMovementTestLP()
	tx := liquidityMovementTestTransaction(lp, "Actor222", 150, 100, 30, 20, "withdraw")
	movement, ok := parseLiquidityMovement(lp, services.SolanaSignatureInfo{Signature: "RemoveSig111", Slot: 1201}, tx)
	if !ok || movement.Kind != "remove_liquidity" || movement.TokenDelta != -50 || movement.QuoteDelta != -10 {
		t.Fatalf("movement=%#v ok=%t", movement, ok)
	}
}

func TestParseLiquidityMovementRejectsSwapOpposingVaultDeltas(t *testing.T) {
	lp := liquidityMovementTestLP()
	tx := liquidityMovementTestTransaction(lp, "Trader111", 100, 150, 30, 20, "swap")
	if movement, ok := parseLiquidityMovement(lp, services.SolanaSignatureInfo{Signature: "SwapSig111", Slot: 1202}, tx); ok {
		t.Fatalf("swap was misclassified as liquidity movement: %#v", movement)
	}
}

func TestLiquidityMovementRequiresPoolAndPinnedProgramReferences(t *testing.T) {
	lp := liquidityMovementTestLP()
	tx := liquidityMovementTestTransaction(lp, "Actor111", 100, 150, 20, 30, "deposit")
	message := creatorIntelMap(creatorIntelMap(map[string]any(tx)["transaction"])["message"])
	message["accountKeys"] = []any{map[string]any{"pubkey": "Actor111", "signer": true}, map[string]any{"pubkey": lp.PoolAddress, "signer": false}, map[string]any{"pubkey": lp.TokenVault, "signer": false}, map[string]any{"pubkey": lp.QuoteVault, "signer": false}}
	if movement, ok := parseLiquidityMovement(lp, services.SolanaSignatureInfo{Signature: "MissingProgram", Slot: 1203}, tx); ok {
		t.Fatalf("movement without program reference accepted: %#v", movement)
	}
}
