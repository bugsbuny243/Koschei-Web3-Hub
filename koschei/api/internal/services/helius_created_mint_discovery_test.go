package services

import (
	"net/url"
	"testing"
)

func TestExtractHeliusCreatedMintCandidatesPrefersPumpMintAndDoesNotClaimSigner(t *testing.T) {
	actor := "Actor111"
	rows := extractHeliusCreatedMintCandidates([]heliusEnhancedTypedTransaction{
		{
			Signature: "Sig111",
			Slot:      123,
			Timestamp: 1700000000,
			Type:      "CREATE",
			Source:    "PUMP_FUN",
			FeePayer:  actor,
			TokenTransfers: []heliusTokenTransfer{
				{Mint: canonicalWrappedSOLMint},
				{Mint: canonicalUSDCMint},
				{Mint: "Mint111pump"},
			},
			Instructions: []heliusInstruction{
				{ProgramID: "ComputeBudget111111111111111111111111111111"},
				{ProgramID: canonicalPumpFunProgramID},
			},
		},
	}, actor)
	if len(rows) != 1 {
		t.Fatalf("expected one candidate, got %#v", rows)
	}
	if rows[0].Mint != "Mint111pump" {
		t.Fatalf("quote asset was selected instead of created mint: %#v", rows[0])
	}
	if rows[0].Program != canonicalPumpFunProgramID {
		t.Fatalf("canonical Pump program was not selected: %#v", rows[0])
	}
	if rows[0].ActorSigned {
		t.Fatalf("fee-payer discovery must not claim signer proof: %#v", rows[0])
	}
	if rows[0].VerificationStatus != "discovery_candidate" {
		t.Fatalf("candidate was promoted before canonical RPC verification: %#v", rows[0])
	}
}

func TestExtractHeliusCreatedMintCandidatesRejectsSwapAndActorMismatch(t *testing.T) {
	transactions := []heliusEnhancedTypedTransaction{
		{
			Signature:      "SwapSig111",
			Type:           "SWAP",
			Source:         "PUMP_AMM",
			FeePayer:       "Actor111",
			TokenTransfers: []heliusTokenTransfer{{Mint: "Mint111pump"}},
		},
		{
			Signature:      "CreateSig222",
			Type:           "CREATE",
			Source:         "PUMP_FUN",
			FeePayer:       "OtherActor222",
			TokenTransfers: []heliusTokenTransfer{{Mint: "Mint222pump"}},
		},
	}
	if rows := extractHeliusCreatedMintCandidates(transactions, "Actor111"); len(rows) != 0 {
		t.Fatalf("swap or mismatched fee payer produced discovery candidates: %#v", rows)
	}
}

func TestHeliusEnhancedTypedTransactionsURLUsesDocumentedCursor(t *testing.T) {
	raw := heliusEnhancedTypedTransactionsURL("key-111", "Actor111", "SigBefore111", 50)
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse request URL: %v", err)
	}
	query := parsed.Query()
	if query.Get("before-signature") != "SigBefore111" {
		t.Fatalf("documented cursor missing: %s", parsed.RawQuery)
	}
	if query.Get("before") != "" {
		t.Fatalf("legacy/unknown cursor parameter must not be sent: %s", parsed.RawQuery)
	}
	if query.Get("limit") != "50" || query.Get("api-key") != "key-111" {
		t.Fatalf("request parameters missing: %s", parsed.RawQuery)
	}
}
