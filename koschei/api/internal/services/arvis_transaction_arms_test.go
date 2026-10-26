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
					map[string]any{"programId": defaultPumpProgramID, "parsed": map[string]any{"type": "buy", "info": map[string]any{"mint": mintB}}},
				},
			},
		},
		"meta": map[string]any{
			"fee":                  float64(500000),
			"computeUnitsConsumed": float64(600000),
			"preBalances":          []any{float64(2_000_000), float64(0)},
			"postBalances":         []any{float64(1_000_000), float64(995000)},
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
	if !evidence.RaydiumRelated || !evidence.PumpRelated || !evidence.ComputeBudgetRelated || !evidence.InitializeMint {
		t.Fatalf("expected Pump, Raydium, compute-budget and initialize evidence: %#v", evidence)
	}
	if !containsString(evidence.ProgramIDs, defaultPumpProgramID) {
		t.Fatalf("verified Pump program missing from parsed programs: %#v", evidence.ProgramIDs)
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
	if arm := buildPumpTransactionArm(req, missing, generatedAt); SecurityRadarVerdictHasVerifiedEvidence(arm) {
		t.Fatal("Pump arm must remain unsigned without parsed program evidence")
	}
	if arm := buildRaydiumTransactionArm(req, missing, generatedAt); SecurityRadarVerdictHasVerifiedEvidence(arm) {
		t.Fatal("Raydium arm must remain unsigned without parsed program evidence")
	}
	if arm := buildLiquidityMovementTransactionArm(req, missing, generatedAt); SecurityRadarVerdictHasVerifiedEvidence(arm) {
		t.Fatal("liquidity arm must remain unsigned without parsed transaction evidence")
	}
	if arm := buildCreatorLinkTransactionArm(req, missing, generatedAt); SecurityRadarVerdictHasVerifiedEvidence(arm) {
		t.Fatal("creator arm must remain unsigned without parsed transaction evidence")
	}
	if arm := buildFundingClusterTransactionArm(req, missing, generatedAt); SecurityRadarVerdictHasVerifiedEvidence(arm) {
		t.Fatal("funding arm must remain unsigned without parsed transaction evidence")
	}
	if arm := buildTransactionIntentProgramArm(req, missing, generatedAt); SecurityRadarVerdictHasVerifiedEvidence(arm) {
		t.Fatal("transaction intent must remain unsigned without parsed transaction evidence")
	}
}

func TestTransactionIntentProgramArmClassifiesParsedIntent(t *testing.T) {
	req := SecurityRadarRequest{Target: "target", Network: "solana-mainnet"}
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	tx := arvisTransactionEvidence{
		Available: true, Signature: "sig-intent", Slot: 123, BlockTime: 456,
		Signers:             []string{"Signer111111111111111111111111111111111"},
		ProgramIDs:          []string{"ComputeBudget111111111111111111111111111111", "675kPX9MHTjS2zt1qfr1NYhd1B9M9QGK6cEcDDCo2t9"},
		InstructionTypes:    []string{"swapBaseIn", "transferChecked"},
		TokenMints:          []string{"So11111111111111111111111111111111111111112", "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"},
		TokenBalanceChanges: map[string]float64{"So11111111111111111111111111111111111111112": -1.5, "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB": 250},
		LamportDeltas:       map[string]int64{"Signer111111111111111111111111111111111": -5000},
		WritableCount:       4, InnerInstructionCount: 2, RaydiumRelated: true, ComputeBudgetRelated: true,
	}
	arm := buildTransactionIntentProgramArm(req, tx, generatedAt)
	if !SecurityRadarVerdictHasVerifiedEvidence(arm) {
		t.Fatalf("expected verified transaction intent evidence: %#v", arm)
	}
	if arm.ModuleID != ModuleProgramRelationScan {
		t.Fatalf("intent must strengthen Program Relation without adding a 15th arm: %s", arm.ModuleID)
	}
	if arm.RiskIndex != 0 || arm.Grade != "-" {
		t.Fatalf("intent arm issued score/grade: %#v", arm)
	}
	if got, _ := arm.Signals["transaction_intent"].(string); got != "liquidity_or_swap_interaction" {
		t.Fatalf("unexpected transaction intent %q", got)
	}
	if reasons, _ := arm.Signals["transaction_intent_reasons"].([]string); len(reasons) == 0 {
		t.Fatalf("missing transaction intent reasons: %#v", arm.Signals)
	}
	if note, _ := arm.Signals["scope_note"].(string); note == "" {
		t.Fatalf("missing evidence-only scope note: %#v", arm.Signals)
	}
}

// A v1 transaction (SIMD-0385) has every account inline, no
// addressTableLookups, and its priority fee and compute-unit limit in
// message.transactionConfig instead of Compute Budget instructions. Reading it
// as "no priority settings" would understate MEV exposure.
func TestParseArvisTransactionEvidenceReadsV1TransactionConfig(t *testing.T) {
	trader := "11111111111111111111111111111111"
	pool := "22222222222222222222222222222222"
	mint := "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"
	tx := map[string]any{
		"slot":      float64(461256126),
		"blockTime": float64(1793000000),
		"version":   float64(1),
		"transaction": map[string]any{
			"message": map[string]any{
				"accountKeys": []any{
					map[string]any{"pubkey": trader, "signer": true, "writable": true, "source": "transaction"},
					map[string]any{"pubkey": pool, "signer": false, "writable": true, "source": "transaction"},
				},
				"instructions": []any{
					map[string]any{"programId": defaultPumpProgramID, "parsed": map[string]any{"type": "buy", "info": map[string]any{"mint": mint}}},
				},
				"transactionConfig": map[string]any{
					"priorityFee": float64(250000), "computeUnitLimit": float64(200000),
					"loadedAccountsDataSizeLimit": float64(65536), "heapSize": float64(32768),
				},
			},
		},
		"meta": map[string]any{
			"fee":                  float64(255000),
			"computeUnitsConsumed": float64(180000),
			"preBalances":          []any{float64(2_000_000), float64(0)},
			"postBalances":         []any{float64(1_745_000), float64(0)},
			"loadedAddresses":      map[string]any{"readonly": []any{}, "writable": []any{}},
			"logMessages":          []any{"Program log: Instruction: Buy"},
		},
	}
	var evidence arvisTransactionEvidence
	evidence.TokenBalanceChanges = map[string]float64{}
	evidence.LamportDeltas = map[string]int64{}
	parseArvisTransactionMap(tx, &evidence)

	if evidence.TransactionVersion != "1" || !evidence.TransactionConfigPriority || evidence.ComputeBudgetRelated {
		t.Fatalf("v1 transactionConfig not read as priority settings: version=%q config=%t computeBudget=%t",
			evidence.TransactionVersion, evidence.TransactionConfigPriority, evidence.ComputeBudgetRelated)
	}
	if len(evidence.AccountKeys) != 2 || len(evidence.Signers) != 1 || evidence.Signers[0] != trader {
		t.Fatalf("inline v1 account keys not parsed: %#v", evidence.AccountKeys)
	}
	evidence.Available = true
	evidence.Signature = "v1-fixture-signature"
	arm := buildTransactionMEVArm(SecurityRadarRequest{Target: mint, Network: "solana-mainnet"}, evidence, time.Now().UTC().Format(time.RFC3339))
	if arm.Signals["transaction_config_priority"] != true || arm.Signals["transaction_version"] != "1" {
		t.Fatalf("MEV arm withheld v1 priority evidence: %#v", arm.Signals)
	}
	if kind, _ := classifyTransactionIntent(evidence); kind == "" {
		t.Fatal("v1 transaction intent was not classified")
	}

	legacy := map[string]any{"version": "legacy", "transaction": map[string]any{"message": map[string]any{}}, "meta": map[string]any{}}
	var legacyEvidence arvisTransactionEvidence
	parseArvisTransactionMap(legacy, &legacyEvidence)
	if legacyEvidence.TransactionVersion != "legacy" || legacyEvidence.TransactionConfigPriority {
		t.Fatalf("legacy transaction misread: %#v", legacyEvidence)
	}
}
