package handlers

import (
	"testing"
	"time"

	"koschei/api/internal/services"
)

func TestActorDefenseTokenAccountOwners(t *testing.T) {
	keys := []string{"source-token-account", "destination-token-account"}
	meta := map[string]any{
		"preTokenBalances": []any{
			map[string]any{"accountIndex": float64(0), "owner": "actor-wallet", "mint": "mint-one"},
		},
		"postTokenBalances": []any{
			map[string]any{"accountIndex": float64(1), "owner": "recipient-wallet", "mint": "mint-one"},
		},
	}
	owners := actorDefenseTokenAccountOwners(meta, keys)
	if owners["source-token-account"].Owner != "actor-wallet" {
		t.Fatalf("source owner=%q", owners["source-token-account"].Owner)
	}
	if owners["destination-token-account"].Owner != "recipient-wallet" {
		t.Fatalf("destination owner=%q", owners["destination-token-account"].Owner)
	}
	if owners["destination-token-account"].Mint != "mint-one" {
		t.Fatalf("destination mint=%q", owners["destination-token-account"].Mint)
	}
}

func TestActorDefenseTokenAccountOwnersIgnoresMissingIndex(t *testing.T) {
	owners := actorDefenseTokenAccountOwners(map[string]any{
		"postTokenBalances": []any{
			map[string]any{"owner": "must-not-map-to-zero", "mint": "mint-one"},
		},
	}, []string{"account-zero"})
	if _, exists := owners["account-zero"]; exists {
		t.Fatal("missing accountIndex must not be coerced to account zero")
	}
}

func TestActorDefenseLiquidityRemovalFromParsedInstruction(t *testing.T) {
	message := map[string]any{"instructions": []any{
		map[string]any{"parsed": map[string]any{"type": "removeLiquidity", "info": map[string]any{}}},
	}}
	found, kinds := actorDefenseLiquidityRemoval(message, map[string]any{})
	if !found {
		t.Fatal("expected parsed remove-liquidity observation")
	}
	if len(kinds) != 1 || kinds[0] != "removeliquidity" {
		t.Fatalf("instruction kinds=%v", kinds)
	}
	if !actorDefenseParsedLiquidityRemoval(kinds) {
		t.Fatal("parsed removal must satisfy the parsed-instruction boundary")
	}
}

func TestActorDefenseLiquidityRemovalFromLogsRemainsUnverified(t *testing.T) {
	meta := map[string]any{"logMessages": []any{"Program log: remove_liquidity"}}
	found, kinds := actorDefenseLiquidityRemoval(map[string]any{}, meta)
	if !found {
		t.Fatal("expected log-backed remove-liquidity observation")
	}
	if actorDefenseParsedLiquidityRemoval(kinds) {
		t.Fatal("log-only marker must not satisfy the VERIFIED parsed-instruction boundary")
	}
}

func TestActorDefenseKnownTransactionMints(t *testing.T) {
	meta := map[string]any{
		"preTokenBalances": []any{
			map[string]any{"mint": "known-two"},
			map[string]any{"mint": "unrelated"},
		},
		"postTokenBalances": []any{
			map[string]any{"mint": "known-one"},
			map[string]any{"mint": "known-two"},
		},
	}
	got := actorDefenseKnownTransactionMints(meta, map[string]bool{"known-one": true, "known-two": true})
	if len(got) != 2 || got[0] != "known-one" || got[1] != "known-two" {
		t.Fatalf("known mints=%v", got)
	}
}

func TestActorDefenseTokenAmountCheckedTransfer(t *testing.T) {
	amount := actorDefenseTokenAmount(map[string]any{"tokenAmount": map[string]any{
		"uiAmountString": "42.125", "decimals": float64(6), "amount": "42125000",
	}})
	if amount != 42.125 {
		t.Fatalf("amount=%v", amount)
	}
	if scope := actorDefenseTokenAmountScope(map[string]any{"tokenAmount": map[string]any{"uiAmountString": "42.125"}}); scope != "ui_amount" {
		t.Fatalf("scope=%q", scope)
	}
}

func TestActorDefenseRawTransferDoesNotPretendBaseUnitsAreTokens(t *testing.T) {
	info := map[string]any{"amount": "42125000"}
	if amount := actorDefenseTokenAmount(info); amount != 0 {
		t.Fatalf("raw base units were exposed as token amount: %v", amount)
	}
	if scope := actorDefenseTokenAmountScope(info); scope != "raw_base_units_only" {
		t.Fatalf("scope=%q", scope)
	}
}

func TestActorDefenseContainsExactIsCaseSensitive(t *testing.T) {
	if actorDefenseContainsExact([]string{"ActorABC"}, "actorabc") {
		t.Fatal("Solana wallet comparison must remain case-sensitive")
	}
	if !actorDefenseContainsExact([]string{"ActorABC"}, "ActorABC") {
		t.Fatal("exact signer should match")
	}
}

func TestActorDefenseInstructionEvidenceRequiresExactAuthority(t *testing.T) {
	dossier := services.ActorDefenseDossier{Wallet: "ActorABC", Network: "solana-mainnet"}
	signature := services.SolanaSignatureInfo{Signature: "sig-one", Slot: 99}
	owners := map[string]actorDefenseTokenAccountOwner{
		"source-ata": {Owner: "ActorABC", Mint: "mint-one"},
		"dest-ata":   {Owner: "holder-wallet", Mint: "mint-one"},
	}
	related := map[string]services.ActorDefenseRelatedActor{
		"holder-wallet": {Wallet: "holder-wallet", SharedTokenCount: 2, MaxPercentage: 55},
	}
	instruction := map[string]any{
		"program": "spl-token",
		"parsed": map[string]any{
			"type": "transferChecked",
			"info": map[string]any{
				"source":      "source-ata",
				"destination": "dest-ata",
				"authority":   "actorabc",
				"tokenAmount": map[string]any{"uiAmountString": "10"},
			},
		},
	}
	rows := actorDefenseInstructionEvidence(dossier, signature, time.Unix(1700000000, 0).UTC(), true, instruction, owners, map[string]bool{"mint-one": true}, related, 0)
	if len(rows) != 0 {
		t.Fatalf("case-mismatched authority produced verified evidence: %#v", rows)
	}

	instruction["parsed"].(map[string]any)["info"].(map[string]any)["authority"] = "ActorABC"
	rows = actorDefenseInstructionEvidence(dossier, signature, time.Unix(1700000000, 0).UTC(), true, instruction, owners, map[string]bool{"mint-one": true}, related, 0)
	if len(rows) != 1 {
		t.Fatalf("exact authority evidence rows=%d", len(rows))
	}
	if rows[0].VerificationStatus != "verified" || rows[0].Relation != "direct_token_transfer_out" {
		t.Fatalf("unexpected evidence=%#v", rows[0])
	}
	if value, _ := rows[0].Metadata["dominant_holder_relation"].(bool); !value {
		t.Fatalf("dominant holder relation was not preserved: %#v", rows[0].Metadata)
	}
}

func TestActorDefenseLowShareRelatedWalletDoesNotBecomeDominantHolder(t *testing.T) {
	metadata := map[string]any{}
	actorDefenseApplyRelatedActorMetadata(metadata, map[string]services.ActorDefenseRelatedActor{
		"small-holder": {Wallet: "small-holder", SharedTokenCount: 2, MaxPercentage: 4.5},
	}, "small-holder")
	if observed, _ := metadata["related_actor_observed"].(bool); !observed {
		t.Fatal("related actor observation should remain visible")
	}
	if dominant, _ := metadata["known_related_actor"].(bool); dominant {
		t.Fatal("low-share related wallet must not activate dominant-holder funding rule")
	}
}

func TestActorDefenseSystemTransferRequiresActorSignature(t *testing.T) {
	dossier := services.ActorDefenseDossier{Wallet: "ActorABC", Network: "solana-mainnet"}
	instruction := map[string]any{
		"program": "system",
		"parsed": map[string]any{
			"type": "transfer",
			"info": map[string]any{
				"source": "ActorABC", "destination": "WalletTwo", "lamports": float64(1_000_000_000),
			},
		},
	}
	rows := actorDefenseInstructionEvidence(dossier, services.SolanaSignatureInfo{Signature: "sig-two", Slot: 100}, time.Now().UTC(), false, instruction, nil, nil, nil, 0)
	if len(rows) != 0 {
		t.Fatalf("unsigned outgoing SOL transfer produced verified evidence: %#v", rows)
	}
	rows = actorDefenseInstructionEvidence(dossier, services.SolanaSignatureInfo{Signature: "sig-two", Slot: 100}, time.Now().UTC(), true, instruction, nil, nil, nil, 0)
	if len(rows) != 1 || rows[0].AmountNative != 1 {
		t.Fatalf("signed SOL evidence=%#v", rows)
	}
}

func TestActorDefenseObservedAtUsesTransactionBlockTime(t *testing.T) {
	got := actorDefenseObservedAt(services.SolanaSignatureInfo{}, map[string]any{"blockTime": float64(1700000000)})
	want := time.Unix(1700000000, 0).UTC()
	if !got.Equal(want) {
		t.Fatalf("observed_at=%s want=%s", got, want)
	}
}
