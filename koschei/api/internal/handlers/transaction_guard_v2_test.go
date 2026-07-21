package handlers

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"koschei/api/internal/services"
)

const (
	guardTestSPLTokenProgramID = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	guardTestMintA             = "So11111111111111111111111111111111111111112"
	guardTestMintB             = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
)

func TestTransactionGuardRequestRejectsInvalidProgramIdentity(t *testing.T) {
	var input transactionGuardV2Request
	err := json.Unmarshal([]byte(`{"transaction":"dGVzdA==","expected_programs":["not-a-solana-program"]}`), &input)
	if err == nil {
		t.Fatal("invalid expected program was accepted")
	}
}

func TestTransactionGuardRequestAcceptsValidIdentityPolicy(t *testing.T) {
	var input transactionGuardV2Request
	err := json.Unmarshal([]byte(`{"transaction":"dGVzdA==","wallet":"11111111111111111111111111111111","expected_programs":["ComputeBudget111111111111111111111111111111"],"accounts":[{"address":"33333333333333333333333333333333","mint":"So11111111111111111111111111111111111111112","role":"observe"}]}`), &input)
	if err != nil {
		t.Fatalf("valid identity policy was rejected: %v", err)
	}
}

func TestTransactionGuardAccountDeltaLabelsVerifiedMint(t *testing.T) {
	encoded, err := json.Marshal(transactionGuardAccountDelta{Address: "33333333333333333333333333333333", Mint: guardTestMintA, MintVerified: true, Role: "observe", PolicyStatus: "pass", EvidenceStatus: "verified_rpc_simulation"})
	if err != nil {
		t.Fatalf("marshal account delta: %v", err)
	}
	text := string(encoded)
	if !strings.Contains(text, `"declared_mint":"`+guardTestMintA+`"`) || !strings.Contains(text, `"mint_verified":true`) {
		t.Fatalf("mint evidence labels are missing: %s", text)
	}
	if strings.Contains(text, `"mint":"`) {
		t.Fatalf("declared mint was serialized through an ambiguous field: %s", text)
	}
}

func TestAssessTransactionGuardSimulationUsesFullProgramLogSurface(t *testing.T) {
	blocked := "JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4"
	var simulation services.SolanaSimulationResult
	for i := 0; i < maxFirewallLogs; i++ {
		simulation.Value.Logs = append(simulation.Value.Logs, fmt.Sprintf("Program log: filler %d", i))
	}
	simulation.Value.Logs = append(simulation.Value.Logs, "Program "+blocked+" invoke [1]")
	assessment := assessTransactionGuardSimulation(simulation)
	if len(assessment.Logs) != maxFirewallLogs {
		t.Fatalf("response logs=%d want=%d", len(assessment.Logs), maxFirewallLogs)
	}
	if !containsString(assessment.ProgramIDs, blocked) {
		t.Fatalf("full log program surface was truncated: %#v", assessment.ProgramIDs)
	}
}

func TestEvaluateTransactionGuardProgramsDetectsBlockedAndUnexpected(t *testing.T) {
	blocked := "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
	unexpected := "JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4"
	expected := "22222222222222222222222222222222"
	policy, findings := evaluateTransactionGuardPrograms([]string{blocked, unexpected}, []string{expected}, nil, []string{blocked})
	if len(policy.BlockedInvoked) != 1 || policy.BlockedInvoked[0] != blocked {
		t.Fatalf("blocked invoked = %#v", policy.BlockedInvoked)
	}
	if len(policy.Unexpected) != 1 || policy.Unexpected[0] != unexpected {
		t.Fatalf("unexpected = %#v", policy.Unexpected)
	}
	if !hasGuardFinding(findings, "blocked_program_invoked") || !hasGuardFinding(findings, "unexpected_program") {
		t.Fatalf("findings = %#v", findings)
	}
	if policy.Complete {
		t.Fatal("policy should not be complete when blocked or unexpected programs are invoked")
	}
}

func TestEvaluateTransactionGuardAccountsEnforcesSpendReceiveAndSlippage(t *testing.T) {
	inputAddress := "33333333333333333333333333333333"
	outputAddress := "44444444444444444444444444444444"
	specs := []transactionGuardAccount{
		{Address: inputAddress, Role: "input", MaximumSpendRaw: "100"},
		{Address: outputAddress, Role: "output", MinimumReceiveRaw: "100", QuotedReceiveRaw: "100", MaxSlippageBPS: 500},
	}
	pre := []*services.SolanaAccountInfo{guardAccountInfo(t, guardTestMintA, 1000), guardAccountInfo(t, guardTestMintB, 0)}
	post := []*services.SolanaAccountInfo{guardAccountInfo(t, guardTestMintA, 800), guardAccountInfo(t, guardTestMintB, 80)}
	policy, findings := evaluateTransactionGuardAccounts(specs, []string{inputAddress, outputAddress}, []string{inputAddress, outputAddress}, pre, post)
	if !policy.Complete {
		t.Fatal("RPC evidence is complete even when a policy fails")
	}
	if policy.Accounts[0].SpentRaw != "200" || policy.Accounts[0].PolicyStatus != "fail" {
		t.Fatalf("input result = %#v", policy.Accounts[0])
	}
	if policy.Accounts[1].ReceivedRaw != "80" || policy.Accounts[1].SlippageBPS == nil || *policy.Accounts[1].SlippageBPS != 2000 {
		t.Fatalf("output result = %#v", policy.Accounts[1])
	}
	for _, code := range []string{"maximum_spend_exceeded", "minimum_receive_not_met", "slippage_limit_exceeded"} {
		if !hasGuardFinding(findings, code) {
			t.Fatalf("missing finding %s in %#v", code, findings)
		}
	}
}

func TestEvaluateTransactionGuardAccountsVerifiesDeclaredMint(t *testing.T) {
	address := "33333333333333333333333333333333"
	specs := []transactionGuardAccount{{Address: address, Mint: guardTestMintA, Role: "output", MinimumReceiveRaw: "50"}}
	pre := []*services.SolanaAccountInfo{guardAccountInfo(t, guardTestMintA, 0)}
	post := []*services.SolanaAccountInfo{guardAccountInfo(t, guardTestMintA, 80)}
	policy, findings := evaluateTransactionGuardAccounts(specs, []string{address}, []string{address}, pre, post)
	if !policy.Complete || len(findings) != 0 || !policy.Accounts[0].MintVerified {
		t.Fatalf("verified mint policy=%#v findings=%#v", policy, findings)
	}
}

func TestEvaluateTransactionGuardAccountsBlocksMintMismatch(t *testing.T) {
	address := "33333333333333333333333333333333"
	specs := []transactionGuardAccount{{Address: address, Mint: guardTestMintB, Role: "output", MinimumReceiveRaw: "50"}}
	pre := []*services.SolanaAccountInfo{guardAccountInfo(t, guardTestMintA, 0)}
	post := []*services.SolanaAccountInfo{guardAccountInfo(t, guardTestMintA, 80)}
	policy, findings := evaluateTransactionGuardAccounts(specs, []string{address}, []string{address}, pre, post)
	if !hasGuardFinding(findings, "guard_account_mint_mismatch") || policy.Accounts[0].PolicyStatus != "fail" || policy.Accounts[0].MintVerified {
		t.Fatalf("mint mismatch policy=%#v findings=%#v", policy, findings)
	}
}

func TestEvaluateTransactionGuardAccountsHandlesCreatedOutputATA(t *testing.T) {
	address := "33333333333333333333333333333333"
	specs := []transactionGuardAccount{{Address: address, Mint: guardTestMintA, Role: "output", MinimumReceiveRaw: "50"}}
	policy, findings := evaluateTransactionGuardAccounts(specs, []string{address}, []string{address}, []*services.SolanaAccountInfo{nil}, []*services.SolanaAccountInfo{guardAccountInfo(t, guardTestMintA, 80)})
	if !policy.Complete || len(findings) != 0 || policy.Accounts[0].ReceivedRaw != "80" || policy.Accounts[0].EvidenceStatus != "verified_rpc_simulation_created_account" || !policy.Accounts[0].MintVerified {
		t.Fatalf("created output policy=%#v findings=%#v", policy, findings)
	}
}

func TestEvaluateTransactionGuardAccountsHandlesClosedInputAccount(t *testing.T) {
	address := "33333333333333333333333333333333"
	specs := []transactionGuardAccount{{Address: address, Mint: guardTestMintA, Role: "input", MaximumSpendRaw: "100"}}
	policy, findings := evaluateTransactionGuardAccounts(specs, []string{address}, []string{address}, []*services.SolanaAccountInfo{guardAccountInfo(t, guardTestMintA, 80)}, []*services.SolanaAccountInfo{nil})
	if !policy.Complete || len(findings) != 0 || policy.Accounts[0].SpentRaw != "80" || policy.Accounts[0].EvidenceStatus != "verified_rpc_simulation_closed_account" || !policy.Accounts[0].MintVerified {
		t.Fatalf("closed input policy=%#v findings=%#v", policy, findings)
	}
}

func TestEvaluateTransactionGuardAccountsWithholdsMissingObserveSide(t *testing.T) {
	address := "33333333333333333333333333333333"
	policy, findings := evaluateTransactionGuardAccounts([]transactionGuardAccount{{Address: address, Role: "observe"}}, []string{address}, []string{address}, []*services.SolanaAccountInfo{nil}, []*services.SolanaAccountInfo{guardAccountInfo(t, guardTestMintA, 80)})
	if policy.Complete || !hasGuardFinding(findings, "guard_account_decode_failed") {
		t.Fatalf("missing observe side policy=%#v findings=%#v", policy, findings)
	}
}

func TestFinalizeGuardAssessmentPreservesDeterministicSimulationBlock(t *testing.T) {
	var simulation services.SolanaSimulationResult
	simulation.Value.Err = map[string]any{"InstructionError": []any{0, "Custom"}}
	simulation.Value.Logs = []string{"Program 11111111111111111111111111111111 failed: custom program error"}
	assessment := assessTransactionGuardSimulation(simulation)
	got := finalizeGuardAssessment(assessment, transactionGuardProgramPolicy{Complete: true}, transactionGuardIntentPolicy{Complete: true})
	if got.Action != "block" || got.RiskLevel != "critical" || guardHTTPStatus(got) != 200 {
		t.Fatalf("action=%s level=%s status=%d", got.Action, got.RiskLevel, guardHTTPStatus(got))
	}
}

func TestFinalizeGuardAssessmentWithholdsProviderFailure(t *testing.T) {
	got := finalizeGuardAssessment(unavailableGuardAssessment(fmt.Errorf("timeout")), transactionGuardProgramPolicy{Complete: false}, transactionGuardIntentPolicy{Complete: false})
	if got.Action != "withhold" || got.RiskLevel != "unknown" || guardHTTPStatus(got) != 503 {
		t.Fatalf("action=%s level=%s status=%d", got.Action, got.RiskLevel, guardHTTPStatus(got))
	}
}

func TestFinalizeGuardAssessmentWithholdsIncompleteEvidence(t *testing.T) {
	assessment := transactionFirewallAssessment{SimulationOK: true, Findings: []transactionFirewallFinding{}, ProgramIDs: []string{}, Logs: []string{}}
	got := finalizeGuardAssessment(assessment, transactionGuardProgramPolicy{Complete: true}, transactionGuardIntentPolicy{Requested: true, Complete: false})
	if got.Action != "withhold" || got.RiskLevel != "unknown" {
		t.Fatalf("action=%s level=%s", got.Action, got.RiskLevel)
	}
}

func guardAccountInfo(t *testing.T, mint string, amount uint64) *services.SolanaAccountInfo {
	t.Helper()
	mintBytes, err := decodeSolanaPublicKey(mint)
	if err != nil {
		t.Fatalf("decode mint %s: %v", mint, err)
	}
	data := make([]byte, 165)
	copy(data[:32], mintBytes)
	binary.LittleEndian.PutUint64(data[64:72], amount)
	return &services.SolanaAccountInfo{Owner: guardTestSPLTokenProgramID, Data: []any{base64.StdEncoding.EncodeToString(data), "base64"}}
}

func hasGuardFinding(findings []transactionFirewallFinding, code string) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}
