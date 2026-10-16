package handlers

import (
	"strings"
	"testing"
)

func TestAttackPathUnavailableDecodeDoesNotClaimNoAttackObserved(t *testing.T) {
	result := buildTransactionGuardAttackPaths(
		"",
		transactionFirewallAssessment{},
		transactionGuardDecodedTransaction{},
		transactionGuardCPIFlowAnalysis{},
		transactionGuardAuthoritySurfaceAnalysis{},
	)
	if result.Complete {
		t.Fatal("unavailable decoded transaction must never be complete")
	}
	if result.Status != "incomplete_evidence" {
		t.Fatalf("unexpected status: %q", result.Status)
	}
	if result.PathCount != 0 {
		t.Fatalf("fabricated attack paths: %d", result.PathCount)
	}
	found := false
	for _, limitation := range result.Limitations {
		if strings.Contains(limitation, "Decoded transaction instructions were unavailable") {
			found = true
		}
	}
	if !found {
		t.Fatal("missing explicit decoding limitation")
	}
}

func TestAttackPathSecondaryFindingCannotUpgradeMissingDecode(t *testing.T) {
	decoded := transactionGuardDecodedTransaction{
		Available: false,
		Complete:  true, // Deliberately inconsistent upstream flags.
		AutomaticBalance: transactionGuardAutomaticBalanceAnalysis{
			Requested: false, Complete: true,
		},
	}
	assessment := transactionFirewallAssessment{Findings: []transactionFirewallFinding{{
		Code: "signed_ui_intent_policy_mismatch", Severity: "critical",
		Title: "Signed UI intent mismatch", Evidence: "declared program mismatch",
	}}}
	result := buildTransactionGuardAttackPaths(
		"", assessment, decoded,
		transactionGuardCPIFlowAnalysis{Required: false, Complete: true},
		transactionGuardAuthoritySurfaceAnalysis{Required: false, Complete: true},
	)
	if result.Complete {
		t.Fatal("secondary findings must not upgrade missing transaction decode to complete")
	}
	if !result.Available {
		t.Fatal("secondary findings must remain visible despite incomplete transaction decoding")
	}
	if result.Status != "attack_path_observed" || result.PathCount != 1 {
		t.Fatalf("expected bounded observed finding: %+v", result)
	}
	if result.Paths[0].Confidence != "medium" {
		t.Fatalf("unavailable decoded transaction cannot claim high confidence: %+v", result.Paths[0])
	}
	limited := false
	for _, limitation := range result.Limitations {
		if strings.Contains(limitation, "Decoded transaction instructions were unavailable") {
			limited = true
		}
	}
	if !limited {
		t.Fatal("incomplete decoding must remain visible alongside suspicious findings")
	}
}
