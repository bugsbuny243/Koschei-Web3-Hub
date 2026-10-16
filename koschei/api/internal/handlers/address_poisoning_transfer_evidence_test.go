package handlers

import (
	"reflect"
	"strings"
	"testing"

	"koschei/api/internal/services"
)

func recipientEvidenceTransaction(keys []any, instructions []any, meta map[string]any) services.SolanaTransactionResult {
	return services.SolanaTransactionResult{
		"transaction": map[string]any{
			"message": map[string]any{
				"accountKeys": keys,
				"instructions": instructions,
			},
		},
		"meta": meta,
	}
}

func recipientEvidenceInstruction(programID, kind string, info map[string]any) map[string]any {
	return map[string]any{
		"programId": programID,
		"parsed": map[string]any{"type": kind, "info": info},
	}
}

func TestRecipientEvidenceOnlySuccessfulOutgoingSOLTransfers(t *testing.T) {
	wallet := testRecipientAddress("W")
	outbound := testRecipientAddress("R")
	unrelated := testRecipientAddress("X")
	inboundSender := testRecipientAddress("I")
	mint := testRecipientAddress("M")
	tx := recipientEvidenceTransaction(
		[]any{wallet, outbound, unrelated, mint, inboundSender},
		[]any{
			recipientEvidenceInstruction(addressPoisoningSystemProgramID, "transfer",
				map[string]any{"source": inboundSender, "destination": wallet, "lamports": float64(1)}),
			recipientEvidenceInstruction(addressPoisoningSystemProgramID, "transfer",
				map[string]any{"source": wallet, "destination": unrelated, "lamports": float64(0)}),
			recipientEvidenceInstruction("Spoofed111111111111111111111111111111", "transfer",
				map[string]any{"source": wallet, "destination": unrelated, "lamports": float64(50)}),
			recipientEvidenceInstruction(addressPoisoningSystemProgramID, "transfer",
				map[string]any{"source": wallet, "destination": outbound, "lamports": float64(1234)}),
		},
		map[string]any{"err": nil, "postTokenBalances": []any{map[string]any{"owner": unrelated, "mint": mint}}},
	)
	result := extractSolanaOutboundRecipients(tx, wallet)
	if !reflect.DeepEqual(result.Recipients, []string{outbound}) || result.TransferCount != 1 ||
		result.UnresolvedTransfers != 0 || result.UnparsedInstructions != 0 || result.FailedTransaction || result.MissingStructure {
		t.Fatalf("incorrect wallet-origin recipient extraction: %+v", result)
	}
}

func TestRecipientEvidenceTokenOwnerOnlyAndInnerInstructions(t *testing.T) {
	wallet := testRecipientAddress("W")
	recipient := testRecipientAddress("R")
	sourceATA := testRecipientAddress("S")
	destinationATA := testRecipientAddress("D")
	mint := testRecipientAddress("M")
	keys := []any{
		map[string]any{"pubkey": wallet},
		map[string]any{"pubkey": sourceATA},
		map[string]any{"pubkey": destinationATA},
		map[string]any{"pubkey": mint},
	}
	tx := recipientEvidenceTransaction(keys, []any{}, map[string]any{
		"err": nil,
		"preTokenBalances": []any{
			map[string]any{"accountIndex": float64(1), "owner": wallet, "mint": mint},
			map[string]any{"accountIndex": float64(2), "owner": recipient, "mint": mint},
		},
		"postTokenBalances": []any{
			map[string]any{"accountIndex": float64(1), "owner": wallet, "mint": mint},
			map[string]any{"accountIndex": float64(2), "owner": recipient, "mint": mint},
		},
		"innerInstructions": []any{map[string]any{"index": float64(0), "instructions": []any{
			recipientEvidenceInstruction(addressPoisoningToken2022ProgramID, "transferChecked",
				map[string]any{"source": sourceATA, "destination": destinationATA, "authority": wallet, "mint": mint,
					"tokenAmount": map[string]any{"amount": "25", "decimals": float64(6)}}),
		}}},
	})
	result := extractSolanaOutboundRecipients(tx, wallet)
	if !reflect.DeepEqual(result.Recipients, []string{recipient}) || result.TransferCount != 1 ||
		result.UnresolvedTransfers != 0 || result.UnparsedInstructions != 0 {
		t.Fatalf("Token-2022 recipient must be owner, not token account or mint: %+v", result)
	}
}

func TestRecipientEvidenceRejectsUnknownOrConflictingTokenOwner(t *testing.T) {
	wallet, source, destination, mint := testRecipientAddress("W"), testRecipientAddress("S"), testRecipientAddress("D"), testRecipientAddress("M")
	firstOwner, changedOwner := testRecipientAddress("R"), testRecipientAddress("C")
	instruction := recipientEvidenceInstruction(addressPoisoningSPLTokenProgramID, "transfer",
		map[string]any{"source": source, "destination": destination, "authority": wallet, "amount": "10"})
	tx := recipientEvidenceTransaction([]any{wallet, source, destination}, []any{instruction}, map[string]any{
		"err": nil,
		"preTokenBalances": []any{
			map[string]any{"accountIndex": float64(1), "owner": wallet, "mint": mint},
			map[string]any{"accountIndex": float64(2), "owner": firstOwner, "mint": mint},
		},
		"postTokenBalances": []any{
			map[string]any{"accountIndex": float64(2), "owner": changedOwner, "mint": mint},
		},
	})
	result := extractSolanaOutboundRecipients(tx, wallet)
	if len(result.Recipients) != 0 || result.UnresolvedTransfers != 1 {
		t.Fatalf("ambiguous token owner must not be a known recipient: %+v", result)
	}
}

func TestRecipientEvidenceIgnoresFailedAndOpaqueTransactions(t *testing.T) {
	wallet, recipient := testRecipientAddress("W"), testRecipientAddress("R")
	transfer := recipientEvidenceInstruction(addressPoisoningSystemProgramID, "transfer",
		map[string]any{"source": wallet, "destination": recipient, "lamports": float64(10)})
	for _, test := range []struct {
		name string
		tx services.SolanaTransactionResult
		failed, incomplete bool
	}{
		{"onchain_failed", recipientEvidenceTransaction([]any{wallet, recipient}, []any{transfer}, map[string]any{"err": "InstructionError"}), true, false},
		{"no_meta", recipientEvidenceTransaction([]any{wallet, recipient}, []any{transfer}, nil), false, true},
		{"no_transaction", services.SolanaTransactionResult{"meta": map[string]any{"err": nil}}, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := extractSolanaOutboundRecipients(test.tx, wallet)
			if len(result.Recipients) != 0 || result.FailedTransaction != test.failed || result.MissingStructure != test.incomplete {
				t.Fatalf("failed or incomplete transaction contributed false contacts: %+v", result)
			}
		})
	}
	opaque := recipientEvidenceTransaction([]any{wallet, recipient},
		[]any{map[string]any{"programId": addressPoisoningSystemProgramID, "data": "opaque"}}, map[string]any{"err": nil})
	result := extractSolanaOutboundRecipients(opaque, wallet)
	if len(result.Recipients) != 0 || result.UnparsedInstructions != 1 {
		t.Fatalf("opaque instruction must be bounded unknown, not recipient: %+v", result)
	}
}

func TestRecipientEvidenceProvenanceIsNotWalletEnforcement(t *testing.T) {
	wallet := testRecipientAddress("W")
	recipient := testRecipientAddress("R")
	candidate := "12345678" + strings.Repeat("C", 24) + "ABCDEFGH"
	out := evaluateAddressPoisoning(wallet, candidate, []string{recipient}, map[string]any{
		"collected": true,
		"coverage_status": "bounded_recent_history",
		"outbound_recipients": []string{recipient},
	})
	if out["policy"] != "block" || out["enforcement_status"] != "not_connected" || out["observed_outbound_recipient_count"] != 1 {
		t.Fatalf("visual collision is advice, never wallet enforcement: %#v", out)
	}
	matches, ok := out["matches"].([]addressPoisoningMatch)
	if !ok || len(matches) != 1 || matches[0].EvidenceSource != "rpc_observed_wallet_outbound_transfer" {
		t.Fatalf("observed recipient provenance missing: %#v", out["matches"])
	}
	userOnly := evaluateAddressPoisoning(wallet, candidate, []string{recipient}, map[string]any{"collected": false})
	found := userOnly["matches"].([]addressPoisoningMatch)
	if len(found) != 1 || found[0].EvidenceSource != "user_supplied_or_unattributed_unverified" ||
		userOnly["finding_state"] != "incomplete_evidence" {
		t.Fatalf("user-supplied address must remain unverified: %#v", userOnly)
	}
}

func TestAddressPoisoningRPCURLRespectsRequestedNetwork(t *testing.T) {
	t.Setenv("SOLANA_RPC_URL", "https://example.invalid/mainnet")
	t.Setenv("SOLANA_DEVNET_RPC_URL", "https://example.invalid/devnet")
	t.Setenv("ALCHEMY_API_KEY", "")
	if got := addressPoisoningRPCURL("solana-mainnet"); got != "https://example.invalid/mainnet" {
		t.Fatalf("wrong mainnet provider: %q", got)
	}
	if got := addressPoisoningRPCURL("solana-devnet"); got != "https://example.invalid/devnet" {
		t.Fatalf("devnet must not reuse mainnet RPC: %q", got)
	}
	t.Setenv("SOLANA_DEVNET_RPC_URL", "")
	t.Setenv("ALCHEMY_SOLANA_DEVNET_RPC_URL", "")
	if got := addressPoisoningRPCURL("solana-devnet"); got != "" {
		t.Fatalf("unconfigured devnet must fail closed, got %q", got)
	}
}
