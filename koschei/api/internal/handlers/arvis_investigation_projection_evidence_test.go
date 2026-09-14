package handlers

import (
	"strings"
	"testing"
)

func TestEnrichArvisInvestigationProjectionDeepEvidence(t *testing.T) {
	results := make([]arvisInvestigationResult, 0, 14)
	for index := 1; index <= 14; index++ {
		id := ""
		if index < 10 {
			id = "0" + string(rune('0'+index))
		} else {
			id = string(rune('0'+index/10)) + string(rune('0'+index%10))
		}
		results = append(results, arvisInvestigationResult{ID: id, Title: "test-" + id})
	}
	projection := map[string]any{"results": results}
	report := map[string]any{
		"actor_investigation": map[string]any{
			"wallet": "CreatorWallet",
			"funding_origin": map[string]any{
				"source_wallet": "FundingWallet",
				"destination_wallet": "CreatorWallet",
				"amount_sol": 1.25,
				"signature": "FundingSig",
				"slot": 12345,
				"verification_status": "verified",
			},
		},
		"launch_forensics": map[string]any{
			"profiles": []any{
				map[string]any{"owner_wallet": "CreatorLinkedWallet", "creator_linked": true},
			},
		},
		"holder_cluster": map[string]any{
			"shared_funding_groups": []any{
				map[string]any{"wallets": []any{"HolderA", "HolderB"}, "evidence": []any{"shared-funding-evidence"}},
			},
			"synchronized_wallets": []any{"HolderC"},
			"wallets": []any{
				map[string]any{"wallet": "HolderA", "funding_source": "HolderFunder"},
			},
			"flow": map[string]any{
				"linked_wallets": []any{"HolderD"},
				"circular_wallets": []any{"HolderE"},
				"observations": []any{
					map[string]any{
						"source_wallet": "HolderA",
						"destination": "ExitWallet",
						"mint": "TargetMint",
						"amount": 42.5,
						"direction": "outbound",
						"kind": "external_token_recipient",
						"signature": "FlowSig",
						"from_entity": "KnownSource",
						"to_entity": "KnownExit",
						"from_entity_category": "wallet",
						"to_entity_category": "exchange",
						"program_ids": []any{"DexProgram"},
					},
					map[string]any{
						"source_wallet": "HolderA",
						"destination": "HolderB",
						"mint": "TargetMint",
						"amount": 5.0,
						"direction": "outbound",
						"kind": "holder_to_holder",
						"signature": "InternalSig",
					},
				},
			},
		},
		"transaction_evidence": []any{
			map[string]any{"signature": "TradeOnlySig", "trader": "TraderOnly", "direction": "sell", "source": "ledger"},
		},
	}

	enriched := enrichArvisInvestigationProjection(projection, report)
	got, ok := enriched["results"].([]arvisInvestigationResult)
	if !ok {
		t.Fatalf("results type = %T, want []arvisInvestigationResult", enriched["results"])
	}
	if len(got) != 14 {
		t.Fatalf("result count = %d, want 14", len(got))
	}

	byID := map[string]arvisInvestigationResult{}
	for _, result := range got {
		byID[result.ID] = result
	}

	sybil := byID["05"]
	for _, wallet := range []string{"HolderA", "HolderB", "HolderC", "HolderD", "HolderE", "CreatorLinkedWallet"} {
		if !containsProjectionString(sybil.Wallets, wallet) {
			t.Errorf("sybil wallets missing %q: %#v", wallet, sybil.Wallets)
		}
	}
	if !strings.Contains(strings.ToLower(sybil.Answer), "do not by themselves prove common real-world ownership") {
		t.Errorf("sybil answer lost identity caveat: %q", sybil.Answer)
	}
	if !containsProjectionString(sybil.Transactions, "InternalSig") {
		t.Errorf("sybil transactions missing direct holder relation signature: %#v", sybil.Transactions)
	}

	funding := byID["06"]
	for _, wallet := range []string{"FundingWallet", "CreatorWallet", "HolderFunder", "HolderA"} {
		if !containsProjectionString(funding.Wallets, wallet) {
			t.Errorf("funding wallets missing %q: %#v", wallet, funding.Wallets)
		}
	}
	if !containsProjectionString(funding.Transactions, "FundingSig") {
		t.Errorf("funding transactions missing signature: %#v", funding.Transactions)
	}
	if !containsProjectionString(funding.Counterparties, "FundingWallet") || !containsProjectionString(funding.Counterparties, "HolderFunder") {
		t.Errorf("funding counterparties incomplete: %#v", funding.Counterparties)
	}
	if !projectionSliceContainsSubstring(funding.CurrentFindings, "1.250000000 SOL") {
		t.Errorf("funding findings missing amount: %#v", funding.CurrentFindings)
	}

	moneyFlow := byID["07"]
	if !containsProjectionString(moneyFlow.Wallets, "HolderA") || !containsProjectionString(moneyFlow.Wallets, "ExitWallet") {
		t.Errorf("money-flow wallets incomplete: %#v", moneyFlow.Wallets)
	}
	if !containsProjectionString(moneyFlow.Transactions, "FlowSig") || !containsProjectionString(moneyFlow.Transactions, "TradeOnlySig") {
		t.Errorf("money-flow transactions incomplete: %#v", moneyFlow.Transactions)
	}
	if !projectionSliceContainsSubstring(moneyFlow.CurrentFindings, "HolderA -> ExitWallet") {
		t.Errorf("money-flow findings missing direct source/destination: %#v", moneyFlow.CurrentFindings)
	}
	if containsProjectionString(moneyFlow.Counterparties, "invented-recipient") {
		t.Fatalf("money-flow invented a recipient from transaction-only evidence")
	}
	if !containsProjectionString(moneyFlow.Wallets, "TraderOnly") {
		t.Errorf("transaction-only trader should remain supplemental wallet evidence: %#v", moneyFlow.Wallets)
	}

	infra := byID["13"]
	for _, entity := range []string{"KnownSource", "KnownExit", "wallet", "exchange", "DexProgram", "ledger"} {
		if !containsProjectionString(infra.Entities, entity) {
			t.Errorf("infrastructure entities missing %q: %#v", entity, infra.Entities)
		}
	}
	if !containsProjectionString(infra.Transactions, "FlowSig") || !containsProjectionString(infra.Transactions, "TradeOnlySig") {
		t.Errorf("infrastructure transactions incomplete: %#v", infra.Transactions)
	}
}

func containsProjectionString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func projectionSliceContainsSubstring(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}
