package services

import (
	"testing"
	"time"
)

func TestParseArvisTransactionEvidence(t *testing.T) {
	creator := "11111111111111111111111111111111"
	pool := "22222222222222222222222222222222"
	mintA := "So11111111111111111111111111111111111111112"
	mintB := "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"
	tx := map[string]any{
		"slot":      float64(12345),
		"blockTime": float64(1710000000),
		"transaction": map[string]any{
			"message": map[string]any{
				"accountKeys": []any{
					map[string]any{"pubkey": creator, "signer": true, "writable": true},
					map[string]any{"pubkey": pool, "signer": false, "writable": true},
				},
				"instructions": []any{
					map[string]any{"programId": "ComputeBudget111111111111111111111111111111", "parsed": map[string]any{"type": "setComputeUnitPrice", "info": map[string]any{}}},
					map[string]any{"programId": "675kPX9MHTjS2zt1qfr1NYhd1B9M9QGK6cEcDDCo2t9", "parsed": map[string]any{"type": "initializeMint2", "info": map[string]any{"mint": mintA}}},
				},
			},
		},
		"meta": map[string]any{
			"fee":                 float64(500000),
			"computeUnitsConsumed": float64(600000),
			"preBalances":         []any{float64(2_000_000), float64(0)},
			"postBalances":        []any{float64(1_000_000), float64(995000)},
			"preTokenBalances": []any{
				map[string]any{"mint": mintA, "uiTokenAmount": map[string]any{"uiAmount": float64(100)}},
				map[string]any{"mint": mintB, "uiTokenAmount": map[string]any{"uiAmount": float64(50)}},
			},
			"postTokenBalances": []any{
				map[string]any{"mint": mintA, "uiTokenAmount": map[string]any{"uiAmount": float64(70)}},
				map[string]any{"mint": mintB, "uiTokenAmount": map[string]any{"uiAmount": float64(80)}},
			},
			"innerInstructions": []any{map[string]any{"instructions": []any{map[string]any{"programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"}}}},
			"logMessages":       []any{"Program log: raydium initialize2"},
		},
	}

	var evidence arvisTransactionEvidence
	evidence.TokenBalanceChanges = map[string]float64{}
	evidence.LamportDeltas = map[string]int64{}
	parseArvisTransactionMap(tx, &evidence)

	if evidence.CreatorCandidate != creator {
		t.Fatalf("expected creator candidate %s, got %s", creator, evidence.CreatorCandidate)
	}
	if !evidence.RaydiumRelated || !evidence.ComputeBudgetRelated || !evidence.InitializeMint {
		t.Fatalf("expected Raydium, compute-budget and initialize evidence: %#v", evidence)
	}
	if len(evidence.FundingAccounts) == 0 || evidence.FundingAccounts[0] != creator {
		t.Fatalf("expected creator funding delta, got %#v", evidence.FundingAccounts)
	}
	if evidence.TokenBalanceChanges[mintA] != -30 || evidence.TokenBalanceChanges[mintB] != 30 {
		t.Fatalf("unexpected token deltas: %#v", evidence.TokenBalanceChanges)
	}
}

func TestTransactionArmsRequireParsedEvidence(t *testing.T) {
	req := SecurityRadarRequest{Target: "target", Network: "solana-mainnet"}
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	missing := arvisTransactionEvidence{}
	if arm := buildLiquidityMovementTransactionArm(req, missing, generatedAt); arm.Signed {
		t.Fatal("liquidity arm must remain unsigned without parsed transaction evidence")
	}
	if arm := buildCreatorLinkTransactionArm(req, missing, generatedAt); arm.Signed {
		t.Fatal("creator arm must remain unsigned without parsed transaction evidence")
	}
	if arm := buildFundingClusterTransactionArm(req, missing, generatedAt); arm.Signed {
		t.Fatal("funding arm must remain unsigned without parsed transaction evidence")
	}
}
