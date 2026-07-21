package handlers

import (
	"encoding/base64"
	"encoding/binary"
	"testing"

	"koschei/api/internal/services"
)

func TestFinalizeEvidenceFirstGuardPreservesNoLogWithhold(t *testing.T) {
	assessment := transactionFirewallAssessment{
		Action: "withhold", RiskLevel: "unknown", SimulationOK: true,
		Findings: []transactionFirewallFinding{}, ProgramIDs: []string{}, Logs: []string{},
	}
	got := finalizeEvidenceFirstGuardAssessment(assessment, transactionGuardProgramPolicy{Complete: true}, transactionGuardIntentPolicy{Complete: true})
	if got.Action != "withhold" || got.RiskLevel != "unknown" {
		t.Fatalf("action=%s level=%s", got.Action, got.RiskLevel)
	}
}

func TestFinalizeEvidenceFirstGuardWithholdsIncompleteIntentEvenWithWarningScore(t *testing.T) {
	assessment := transactionFirewallAssessment{
		Action: "allow", RiskLevel: "low", SimulationOK: true,
		Findings: []transactionFirewallFinding{{Code: "guard_account_missing", Severity: "high", Score: 30}},
	}
	got := finalizeEvidenceFirstGuardAssessment(assessment, transactionGuardProgramPolicy{Complete: true}, transactionGuardIntentPolicy{Requested: true, Complete: false})
	if got.Action != "withhold" || got.RiskLevel != "unknown" {
		t.Fatalf("action=%s level=%s", got.Action, got.RiskLevel)
	}
}

func TestFinalizeEvidenceFirstGuardKeepsVerifiedBlockAboveIncompleteEvidence(t *testing.T) {
	assessment := transactionFirewallAssessment{
		Action: "block", RiskLevel: "critical", SimulationOK: false,
		Findings: []transactionFirewallFinding{{Code: "simulation_failed", Severity: "critical", Score: 100}},
	}
	got := finalizeEvidenceFirstGuardAssessment(assessment, transactionGuardProgramPolicy{Complete: false}, transactionGuardIntentPolicy{Complete: false})
	if got.Action != "block" || got.RiskLevel != "critical" {
		t.Fatalf("action=%s level=%s", got.Action, got.RiskLevel)
	}
}

func TestStableTransactionGuardAlertKeyIgnoresRequestTime(t *testing.T) {
	input := transactionGuardV2Request{Transaction: "dGVzdA=="}
	assessment := transactionFirewallAssessment{Action: "block"}
	first := stableTransactionGuardAlertKey(input, assessment)
	second := stableTransactionGuardAlertKey(input, assessment)
	if first == "" || first != second {
		t.Fatalf("unstable key: %q %q", first, second)
	}
	assessment.Action = "withhold"
	if first == stableTransactionGuardAlertKey(input, assessment) {
		t.Fatal("different Guard decisions shared one dedupe key")
	}
}

func TestEvaluateTransactionGuardAccountOwnersAcceptsDeclaredWallet(t *testing.T) {
	address := "33333333333333333333333333333333"
	wallet := "11111111111111111111111111111111"
	specs := []transactionGuardAccount{{Address: address, Mint: guardTestMintA, Role: "output"}}
	pre := []*services.SolanaAccountInfo{guardOwnedAccountInfo(t, guardTestMintA, wallet, 0)}
	post := []*services.SolanaAccountInfo{guardOwnedAccountInfo(t, guardTestMintA, wallet, 80)}
	intent, findings := evaluateTransactionGuardAccounts(specs, []string{address}, []string{address}, pre, post)
	if len(findings) != 0 {
		t.Fatalf("amount findings=%#v", findings)
	}
	ownerFindings := evaluateTransactionGuardAccountOwners(wallet, specs, []string{address}, []string{address}, pre, post, &intent)
	if len(ownerFindings) != 0 || intent.Accounts[0].PolicyStatus != "pass" {
		t.Fatalf("owner findings=%#v intent=%#v", ownerFindings, intent)
	}
}

func TestEvaluateTransactionGuardAccountOwnersBlocksForeignATA(t *testing.T) {
	address := "33333333333333333333333333333333"
	declaredWallet := "11111111111111111111111111111111"
	foreignOwner := guardTestMintB
	specs := []transactionGuardAccount{{Address: address, Mint: guardTestMintA, Role: "output"}}
	pre := []*services.SolanaAccountInfo{guardOwnedAccountInfo(t, guardTestMintA, foreignOwner, 0)}
	post := []*services.SolanaAccountInfo{guardOwnedAccountInfo(t, guardTestMintA, foreignOwner, 80)}
	intent, _ := evaluateTransactionGuardAccounts(specs, []string{address}, []string{address}, pre, post)
	ownerFindings := evaluateTransactionGuardAccountOwners(declaredWallet, specs, []string{address}, []string{address}, pre, post, &intent)
	if !hasGuardFinding(ownerFindings, "guard_account_owner_mismatch") {
		t.Fatalf("owner findings=%#v", ownerFindings)
	}
	if intent.Accounts[0].PolicyStatus != "fail" || intent.Accounts[0].EvidenceStatus != "owner_mismatch" {
		t.Fatalf("intent=%#v", intent)
	}
}

func guardOwnedAccountInfo(t *testing.T, mint, owner string, amount uint64) *services.SolanaAccountInfo {
	t.Helper()
	mintBytes, err := decodeSolanaPublicKey(mint)
	if err != nil {
		t.Fatalf("decode mint: %v", err)
	}
	ownerBytes, err := decodeSolanaPublicKey(owner)
	if err != nil {
		t.Fatalf("decode owner: %v", err)
	}
	data := make([]byte, 165)
	copy(data[:32], mintBytes)
	copy(data[32:64], ownerBytes)
	binary.LittleEndian.PutUint64(data[64:72], amount)
	return &services.SolanaAccountInfo{Owner: guardTestSPLTokenProgramID, Data: []any{base64.StdEncoding.EncodeToString(data), "base64"}}
}
