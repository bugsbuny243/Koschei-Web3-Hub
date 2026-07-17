package handlers

import (
	"strings"
	"testing"
)

func acceptanceFixture(completed, producing int) map[string]any {
	report := dossierTestReport()
	report["investigation_coverage"] = map[string]any{
		"status":                  "partial_investigation",
		"capability_total":        14,
		"attempted":               14,
		"completed":               completed,
		"evidence_producing":      producing,
		"completed_without_match": 2,
		"not_applicable":          2,
		"evidence_pending":        14 - completed - 2,
		"source_unavailable":      0,
		"insufficient_evidence":   0,
		"critical_gaps":           []string{},
	}
	report["full_scan_live_evidence"] = map[string]any{
		"status": "complete",
		"mint":   "Mint111",
		"wallet_coverage": []any{
			map[string]any{"wallet": "Creator111", "role": "creator_source_observed", "status": "complete", "transactions_parsed": 4},
			map[string]any{"wallet": "Owner111", "role": "risk_bearing_holder", "status": "complete_no_relevant_token_delta", "transactions_parsed": 3},
		},
		"transactions": []any{
			map[string]any{
				"signature": "LiveSig111", "slot": 12345, "wallet": "Creator111",
				"role": "creator_source_observed", "direction": "sell", "token_delta": -25.0,
				"evidence_key": "LiveSig111:Creator111:sell",
			},
		},
	}
	return report
}

func TestProductionAcceptanceProfiles(t *testing.T) {
	cases := []struct {
		name      string
		profile   string
		completed int
		producing int
	}{
		{name: "pump bonding curve", profile: "bonding_curve", completed: 8, producing: 8},
		{name: "dex traded", profile: "dex_traded", completed: 10, producing: 10},
		{name: "high concentration", profile: "high_concentration", completed: 10, producing: 10},
		{name: "low concentration", profile: "low_concentration", completed: 10, producing: 10},
		{name: "creator sell", profile: "creator_sell", completed: 10, producing: 10},
		{name: "lp movement", profile: "lp_movement", completed: 10, producing: 10},
		{name: "new token", profile: "new_token", completed: 8, producing: 8},
		{name: "old token", profile: "old_token", completed: 10, producing: 10},
		{name: "low activity", profile: "low_activity", completed: 8, producing: 8},
		{name: "high activity", profile: "high_activity", completed: 10, producing: 10},
	}
	if len(cases) != 10 {
		t.Fatalf("acceptance archetypes=%d", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := evaluateInvestigationAcceptance(acceptanceFixture(tc.completed, tc.producing), "Mint111", tc.profile)
			if result.Status != "pass" {
				t.Fatalf("status=%s blockers=%#v warnings=%#v metrics=%#v", result.Status, result.Blockers, result.Warnings, result.Metrics)
			}
			if !result.CallerParity.Passed {
				t.Fatalf("parity=%#v", result.CallerParity)
			}
			if result.Metrics.LiveTransactions != 1 || result.Metrics.CompletedWalletWindows != 2 {
				t.Fatalf("live metrics=%#v", result.Metrics)
			}
		})
	}
}

func TestProductionAcceptanceRejectsTargetMismatch(t *testing.T) {
	result := evaluateInvestigationAcceptance(acceptanceFixture(10, 10), "DifferentMint", "dex_traded")
	assertAcceptanceBlocker(t, result, "target_mismatch")
}

func TestProductionAcceptanceRejectsPopulatedSignalWithoutReference(t *testing.T) {
	report := acceptanceFixture(10, 10)
	refs := dossierMap(report["evidence_references"])
	refs["concentration"] = map[string]any{"wallets": []string{}, "accounts": []string{}, "signatures": []string{}, "slots": []int64{}, "evidence_keys": []string{}}
	result := evaluateInvestigationAcceptance(report, "Mint111", "dex_traded")
	assertAcceptanceBlocker(t, result, "populated_signal_missing_reference")
}

func TestProductionAcceptanceRejectsIncompleteLiveTransaction(t *testing.T) {
	report := acceptanceFixture(10, 10)
	live := dossierMap(report["full_scan_live_evidence"])
	transactions := dossierSlice(live["transactions"])
	delete(dossierMap(transactions[0]), "slot")
	result := evaluateInvestigationAcceptance(report, "Mint111", "dex_traded")
	assertAcceptanceBlocker(t, result, "live_transaction_incomplete")
}

func TestProductionAcceptanceRejectsLiveTargetMismatch(t *testing.T) {
	report := acceptanceFixture(10, 10)
	dossierMap(report["full_scan_live_evidence"])["mint"] = "OtherMint"
	result := evaluateInvestigationAcceptance(report, "Mint111", "dex_traded")
	assertAcceptanceBlocker(t, result, "live_evidence_target_mismatch")
}

func TestProductionAcceptanceReturnsPartialForCoverageFloor(t *testing.T) {
	result := evaluateInvestigationAcceptance(acceptanceFixture(9, 9), "Mint111", "dex_traded")
	if result.Status != "partial" || len(result.Blockers) != 0 || len(result.Warnings) == 0 {
		t.Fatalf("result=%#v", result)
	}
}

func TestProductionAcceptanceParityHashIsStable(t *testing.T) {
	report := acceptanceFixture(10, 10)
	first := investigationCallerParity(report)
	second := investigationCallerParity(report)
	if !first.Passed || !second.Passed {
		t.Fatalf("parity failed: %#v %#v", first, second)
	}
	for _, caller := range []string{"owner", "public", "api"} {
		if first.Hashes[caller] == "" || first.Hashes[caller] != second.Hashes[caller] {
			t.Fatalf("caller=%s first=%q second=%q", caller, first.Hashes[caller], second.Hashes[caller])
		}
	}
}

func assertAcceptanceBlocker(t *testing.T, result InvestigationAcceptanceResult, code string) {
	t.Helper()
	if result.Status != "fail" {
		t.Fatalf("status=%s blockers=%#v", result.Status, result.Blockers)
	}
	for _, item := range result.Blockers {
		if strings.EqualFold(item.Code, code) {
			return
		}
	}
	t.Fatalf("blocker %q missing from %#v", code, result.Blockers)
}
