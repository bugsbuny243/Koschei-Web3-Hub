package services

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestDeriveSolanaAssociatedTokenAddressIsDeterministicAndOffCurve(t *testing.T) {
	owner := encodeSolanaBase58(bytes.Repeat([]byte{7}, 32))
	mint := encodeSolanaBase58(bytes.Repeat([]byte{9}, 32))

	first, bump, err := DeriveSolanaAssociatedTokenAddress(owner, mint, SolanaSPLTokenProgramID)
	if err != nil {
		t.Fatal(err)
	}
	second, secondBump, err := DeriveSolanaAssociatedTokenAddress(owner, mint, SolanaSPLTokenProgramID)
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first != second || bump != secondBump {
		t.Fatalf("ATA derivation is not deterministic: first=%q/%d second=%q/%d", first, bump, second, secondBump)
	}
	decoded, err := decodeSolanaBase58Pubkey(first)
	if err != nil {
		t.Fatalf("decode derived ATA: %v", err)
	}
	if solanaCompressedEd25519Point(decoded) {
		t.Fatalf("derived ATA must be off-curve: %s", first)
	}

	token2022, _, err := DeriveSolanaAssociatedTokenAddress(owner, mint, SolanaToken2022ProgramID)
	if err != nil {
		t.Fatal(err)
	}
	if token2022 == first {
		t.Fatalf("token program must participate in ATA seeds: %s", first)
	}
}

func TestSolanaAssociatedTokenAddressCandidatesUseMintProgramWhenKnown(t *testing.T) {
	owner := encodeSolanaBase58(bytes.Repeat([]byte{11}, 32))
	mint := encodeSolanaBase58(bytes.Repeat([]byte{12}, 32))

	known, err := SolanaAssociatedTokenAddressCandidates(owner, mint, SolanaToken2022ProgramID)
	if err != nil {
		t.Fatal(err)
	}
	if len(known) != 1 || known[0].TokenProgram != SolanaToken2022ProgramID {
		t.Fatalf("known token program candidates=%#v", known)
	}

	unknown, err := SolanaAssociatedTokenAddressCandidates(owner, mint, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(unknown) != 2 || unknown[0].Address == unknown[1].Address {
		t.Fatalf("unknown token program candidates=%#v", unknown)
	}
}

type actorRecipientATAFallbackTransport struct {
	creator            string
	mint               string
	derivedATA         string
	destinationWallet  string
	destinationAccount string
	signature          string
	blockTime          int64
}

func (t *actorRecipientATAFallbackTransport) Transaction(_ context.Context, signature string) (map[string]any, error) {
	if signature != t.signature {
		return map[string]any{}, nil
	}
	return map[string]any{
		"slot":      float64(9911),
		"blockTime": float64(t.blockTime),
		"transaction": map[string]any{
			"message": map[string]any{
				"accountKeys": []any{
					map[string]any{"pubkey": t.creator, "signer": true},
					map[string]any{"pubkey": t.derivedATA, "signer": false},
					map[string]any{"pubkey": t.destinationAccount, "signer": false},
				},
				"instructions": []any{
					map[string]any{
						"program": "spl-token",
						"parsed": map[string]any{
							"type": "transferChecked",
							"info": map[string]any{
								"source":      t.derivedATA,
								"destination": t.destinationAccount,
								"authority":   t.creator,
								"mint":        t.mint,
								"tokenAmount": map[string]any{
									"amount":         "2500000",
									"decimals":       float64(6),
									"uiAmount":       float64(2.5),
									"uiAmountString": "2.5",
								},
							},
						},
					},
				},
			},
		},
		"meta": map[string]any{
			"err": nil,
			"preTokenBalances": []any{
				map[string]any{"accountIndex": float64(1), "mint": t.mint, "owner": t.creator},
				map[string]any{"accountIndex": float64(2), "mint": t.mint, "owner": t.destinationWallet},
			},
			"postTokenBalances": []any{
				map[string]any{"accountIndex": float64(1), "mint": t.mint, "owner": t.creator},
				map[string]any{"accountIndex": float64(2), "mint": t.mint, "owner": t.destinationWallet},
			},
		},
	}, nil
}

func (t *actorRecipientATAFallbackTransport) TokenAccountsByOwnerForMint(_ context.Context, owner, _ string) (SolanaOwnedTokenAccountsResult, error) {
	return SolanaOwnedTokenAccountsResult{Value: []SolanaOwnedTokenAccount{}}, nil
}

func (t *actorRecipientATAFallbackTransport) SignaturesForAddressPage(_ context.Context, address string, options SolanaSignaturePageOptions) ([]SolanaSignatureInfo, error) {
	if address != t.derivedATA || options.Before != "" {
		return []SolanaSignatureInfo{}, nil
	}
	blockTime := t.blockTime
	return []SolanaSignatureInfo{{
		Signature: t.signature,
		Slot:      9911,
		BlockTime: &blockTime,
	}}, nil
}

func (t *actorRecipientATAFallbackTransport) TokenSupply(context.Context, string) (SolanaTokenSupplyResult, error) {
	return SolanaTokenSupplyResult{Value: SolanaTokenAmount{Amount: "0"}}, nil
}

func (t *actorRecipientATAFallbackTransport) LargestTokenAccounts(context.Context, string) (SolanaLargestAccountsResult, error) {
	return SolanaLargestAccountsResult{Value: []SolanaLargestTokenAccount{}}, nil
}

func (t *actorRecipientATAFallbackTransport) MultipleAccounts(_ context.Context, addresses []string) (SolanaMultipleAccountInfoResult, error) {
	out := SolanaMultipleAccountInfoResult{Value: make([]*SolanaAccountInfo, len(addresses))}
	for index, address := range addresses {
		if address == t.mint {
			out.Value[index] = &SolanaAccountInfo{Owner: SolanaSPLTokenProgramID}
		}
	}
	return out, nil
}

func TestActorRecipientDerivedATAFallbackFindsHistoryWithoutPromotingInitialRecipient(t *testing.T) {
	creator := encodeSolanaBase58(bytes.Repeat([]byte{21}, 32))
	mint := encodeSolanaBase58(bytes.Repeat([]byte{22}, 32))
	destinationWallet := encodeSolanaBase58(bytes.Repeat([]byte{23}, 32))
	destinationAccount := encodeSolanaBase58(bytes.Repeat([]byte{24}, 32))
	derivedATA, _, err := DeriveSolanaAssociatedTokenAddress(creator, mint, SolanaSPLTokenProgramID)
	if err != nil {
		t.Fatal(err)
	}
	transport := &actorRecipientATAFallbackTransport{
		creator:            creator,
		mint:               mint,
		derivedATA:         derivedATA,
		destinationWallet:  destinationWallet,
		destinationAccount: destinationAccount,
		signature:          "AtaHistorySignature111",
		blockTime:          time.Date(2026, 9, 18, 8, 30, 0, 0, time.UTC).Unix(),
	}

	report := InvestigateActorInitialRecipientsWithTransport(
		t.Context(),
		transport,
		creator,
		mint,
		"",
		ActorInitialRecipientOptions{
			MaxRecipients:        5,
			SignaturePageSize:    10,
			MaxPagesPerTokenATA:  2,
			MaxTransactionsParse: 10,
		},
	)
	if report.SourceTokenAccountBasis != "deterministic_ata_fallback" {
		t.Fatalf("source basis=%q report=%#v", report.SourceTokenAccountBasis, report)
	}
	if len(report.DerivedSourceTokenAccounts) != 1 || report.DerivedSourceTokenAccounts[0] != derivedATA {
		t.Fatalf("derived source accounts=%v want=%s", report.DerivedSourceTokenAccounts, derivedATA)
	}
	if report.HistoryComplete {
		t.Fatalf("derived-only ATA history must not claim complete creator token-account coverage: %#v", report)
	}
	if report.DistributionScope != "bounded_deterministic_creator_ata_history" {
		t.Fatalf("distribution scope=%q", report.DistributionScope)
	}
	if report.Status != "recipient_window_resolved" || len(report.Recipients) != 1 {
		t.Fatalf("recipient fallback did not resolve expected transfer: %#v", report)
	}
	if report.Recipients[0].Wallet != destinationWallet || report.Recipients[0].SourceTokenAccount != derivedATA {
		t.Fatalf("wrong derived ATA transfer: %#v", report.Recipients[0])
	}

	evidence := ActorInitialRecipientEvidence(report, "solana-mainnet")
	if len(evidence) != 1 {
		t.Fatalf("evidence=%#v", evidence)
	}
	if evidence[0].Relation != "creator_recipient_in_window" {
		t.Fatalf("derived-only fallback was incorrectly promoted to initial recipient: %#v", evidence[0])
	}
	if evidence[0].VerificationStatus != "verified" || evidence[0].Signature != transport.signature {
		t.Fatalf("transaction-backed fallback evidence lost verification: %#v", evidence[0])
	}
}
