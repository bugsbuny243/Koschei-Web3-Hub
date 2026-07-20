package handlers

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"

	"koschei/api/internal/services"
)

const guardTestSPLTokenProgramID = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"

func TestTransactionGuardRequestRejectsInvalidProgramIdentity(t *testing.T) {
	var input transactionGuardV2Request
	err := json.Unmarshal([]byte(`{"transaction":"dGVzdA==","expected_programs":["not-a-solana-program"]}`), &input)
	if err == nil {
		t.Fatal("invalid expected program was accepted")
	}
}

func TestTransactionGuardRequestAcceptsValidIdentityPolicy(t *testing.T) {
	var input transactionGuardV2Request
	err := json.Unmarshal([]byte(`{"transaction":"dGVzdA==","wallet":"11111111111111111111111111111111","expected_programs":["ComputeBudget111111111111111111111111111111"],"accounts":[{"address":"33333333333333333333333333333333","mint":"44444444444444444444444444444444","role":"observe"}]}`), &input)
	if err != nil {
		t.Fatalf("valid identity policy was rejected: %v", err)
	}
}

func TestTransactionGuardAccountDeltaLabelsMintAsDeclared(t *testing.T) {
	encoded, err := json.Marshal(transactionGuardAccountDelta{Address: "33333333333333333333333333333333", Mint: "44444444444444444444444444444444", Role: "observe", PolicyStatus: "pass", EvidenceStatus: "verified_rpc_simulation"})
	if err != nil {
		t.Fatalf("marshal account delta: %v", err)
	}
	text := string(encoded)
	if !strings.Contains(text, `"declared_mint":"44444444444444444444444444444444"`) || !strings.Contains(text, `"mint_verified":false`) {
		t.Fatalf("mint evidence labels are missing: %s", text)
	}
	if strings.Contains(text, `"mint":"`) {
		t.Fatalf("unverified mint was serialized as verified field: %s", text)
	}
}

func TestEvaluateTransactionGuardProgramsDetectsBlockedAndUnexpected(t *testing.T) {
	blocked := "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
	unexpected := "JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4"
	expected := "22222222222222222222222222222222"
	policy, findings := evaluateTransactionGuardPrograms(
		[]string{blocked, unexpected},
		[]string{expected},
		nil,
		[]string{blocked},
	)
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
	pre := []*services.SolanaAccountInfo{guardAccountInfo(1000), guardAccountInfo(0)}
	post := []*services.SolanaAccountInfo{guardAccountInfo(800), guardAccountInfo(80)}
	policy, findings := evaluateTransactionGuardAccounts(specs, []string{inputAddress, outputAddress}, []string{inputAddress, outputAddress}, pre, post)
	if !policy.Complete {
		t.Fatal("RPC evidence is complete even when a policy fails")
	}
	if len(policy.Accounts) != 2 {
		t.Fatalf("account result count = %d", len(policy.Accounts))
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

func TestFinalizeGuardAssessmentWithholdsIncompleteEvidence(t *testing.T) {
	assessment := transactionFirewallAssessment{SimulationOK: true, Findings: []transactionFirewallFinding{}, ProgramIDs: []string{}, Logs: []string{}}
	got := finalizeGuardAssessment(assessment, transactionGuardProgramPolicy{Complete: true}, transactionGuardIntentPolicy{Requested: true, Complete: false})
	if got.Action != "withhold" || got.RiskLevel != "unknown" {
		t.Fatalf("action=%s level=%s", got.Action, got.RiskLevel)
	}
}

func guardAccountInfo(amount uint64) *services.SolanaAccountInfo {
	data := make([]byte, 165)
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
