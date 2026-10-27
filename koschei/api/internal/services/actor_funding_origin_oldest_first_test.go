package services

import "testing"

// Explicit fixtures shaped like Helius getTransactionsForAddress rows.
func fundingFixtureTx(signature string, slot, blockTime int64, failed bool, instructions ...map[string]any) map[string]any {
	items := make([]any, 0, len(instructions))
	for _, instruction := range instructions {
		items = append(items, instruction)
	}
	meta := map[string]any{"err": nil}
	if failed {
		meta["err"] = map[string]any{"InstructionError": []any{0, "fixture"}}
	}
	return map[string]any{
		"slot": float64(slot), "blockTime": float64(blockTime), "meta": meta,
		"transaction": map[string]any{
			"signatures": []any{signature},
			"message": map[string]any{
				"accountKeys":  []any{map[string]any{"pubkey": "ci-fixture-funder", "signer": true}},
				"instructions": items,
			},
		},
	}
}

func fundingFixtureTransfer(source, destination string, lamports int64) map[string]any {
	return map[string]any{"program": "system", "parsed": map[string]any{
		"type": "transfer", "info": map[string]any{"source": source, "destination": destination, "lamports": float64(lamports)},
	}}
}

func TestOldestFirstFundingOriginIsTheFirstTransferIntoTheWallet(t *testing.T) {
	wallet := "ci-fixture-creator"
	rows := []map[string]any{
		fundingFixtureTx("ci-fixture-sig-failed", 100, 1790000000, true, fundingFixtureTransfer("ci-fixture-other", wallet, 9e9)),
		fundingFixtureTx("ci-fixture-sig-out", 101, 1790000010, false, fundingFixtureTransfer(wallet, "ci-fixture-elsewhere", 1e6)),
		fundingFixtureTx("ci-fixture-sig-first", 102, 1790000020, false, fundingFixtureTransfer("ci-fixture-funder", wallet, 1_500_000_000)),
		fundingFixtureTx("ci-fixture-sig-later", 103, 1790000030, false, fundingFixtureTransfer("ci-fixture-later", wallet, 2e9)),
	}
	origin, ok := actorFundingOriginFromOldestTransactions(wallet, rows)
	if !ok {
		t.Fatal("the first inbound transfer was not found")
	}
	if origin.SourceWallet != "ci-fixture-funder" || origin.Signature != "ci-fixture-sig-first" || origin.AmountSOL != 1.5 {
		t.Fatalf("initial funding = %+v, want ci-fixture-funder 1.5 SOL in ci-fixture-sig-first", origin)
	}
	if origin.Status != "initial_funding_observed" || origin.VerificationStatus != "verified" || origin.ResultState != ActorFundingResultVerified {
		t.Fatalf("a signed first transfer read oldest first is the verified initial funding: %+v", origin)
	}
	evidence, ok := ActorFundingOriginEvidence(origin, "solana-mainnet")
	if !ok || evidence.Relation != "initial_funding_in" {
		t.Fatalf("oldest-first funding must persist as initial funding: ok=%v relation=%s", ok, evidence.Relation)
	}
}

func TestOldestFirstFundingOriginReportsNothingWithoutAnInboundTransfer(t *testing.T) {
	wallet := "ci-fixture-creator"
	rows := []map[string]any{fundingFixtureTx("ci-fixture-sig-out", 101, 1790000010, false, fundingFixtureTransfer(wallet, "ci-fixture-elsewhere", 1e6))}
	if origin, ok := actorFundingOriginFromOldestTransactions(wallet, rows); ok {
		t.Fatalf("no inbound transfer must leave the bounded result in place, got %+v", origin)
	}
}
