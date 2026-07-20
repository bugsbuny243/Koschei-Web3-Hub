package services

import (
	"encoding/json"
	"testing"
)

func TestHeliusTokenTransferPreservesAccountsAndMetadata(t *testing.T) {
	payload := []byte(`[
		{
			"signature":"sig-helius",
			"slot":999,
			"timestamp":1700000000,
			"transactionError":null,
			"tokenTransfers":[{
				"fromTokenAccount":"SourceATA",
				"toTokenAccount":"DestinationATA",
				"fromUserAccount":"WalletA",
				"toUserAccount":"WalletB",
				"tokenAmount":12.5,
				"mint":"Mint"
			}],
			"accountData":[{
				"account":"SourceATA",
				"tokenBalanceChanges":[{
					"userAccount":"WalletA",
					"tokenAccount":"SourceATA",
					"mint":"Mint",
					"rawTokenAmount":{"tokenAmount":"12500000","decimals":6}
				}]
			}]
		}
	]`)
	var transactions []heliusEnhancedTransaction
	if err := json.Unmarshal(payload, &transactions); err != nil {
		t.Fatalf("decode enhanced transaction: %v", err)
	}
	if len(transactions) != 1 || len(transactions[0].TokenTransfers) != 1 {
		t.Fatalf("unexpected transactions: %#v", transactions)
	}
	transfer := transactions[0].TokenTransfers[0]
	if transfer.FromTokenAccount != "SourceATA" || transfer.ToTokenAccount != "DestinationATA" {
		t.Fatalf("token accounts were not decoded: %#v", transfer)
	}
	decimals := heliusTransferDecimals(transactions[0], transfer)
	if decimals == nil || *decimals != 6 {
		t.Fatalf("accountData decimals were not decoded: %#v", transactions[0].AccountData)
	}

	fallbackDecimals := 9
	observation, ok := holderClusterObservationFromHeliusTransfer(
		transfer,
		transactions[0],
		"WalletA",
		"Mint",
		map[string]bool{"WalletA": true, "WalletB": true},
		nil,
		heliusAssetMetadata{TokenStandard: "FungibleToken", Decimals: &fallbackDecimals, Available: true},
	)
	if !ok {
		t.Fatal("expected enhanced transfer observation")
	}
	if observation.SourceWallet != "WalletA" || observation.Destination != "WalletB" || observation.Kind != "holder_to_holder" {
		t.Fatalf("wallet endpoints were not preserved: %#v", observation)
	}
	if observation.SourceTokenAccount != "SourceATA" || observation.DestinationTokenAccount != "DestinationATA" {
		t.Fatalf("token-account endpoints were not preserved: %#v", observation)
	}
	if observation.Mint != "Mint" || observation.TokenStandard != "FungibleToken" || observation.Decimals == nil || *observation.Decimals != 6 {
		t.Fatalf("token metadata was not preserved: %#v", observation)
	}
	if observation.Amount != 12.5 || observation.Signature != "sig-helius" || observation.Slot != 999 {
		t.Fatalf("transfer evidence mismatch: %#v", observation)
	}
}

func TestHeliusTransferKeepsZeroDecimalsAndUnresolvedRecipientATA(t *testing.T) {
	zero := 0
	transfer := heliusTokenTransfer{
		FromTokenAccount: "SourceATA",
		ToTokenAccount:   "DestinationATA",
		FromUserAccount:  "WalletA",
		TokenAmount:      1,
		Mint:             "NFTMint",
	}
	observation, ok := holderClusterObservationFromHeliusTransfer(
		transfer,
		heliusEnhancedTransaction{Signature: "sig-nft", Slot: 1000},
		"WalletA",
		"NFTMint",
		map[string]bool{"WalletA": true},
		nil,
		heliusAssetMetadata{TokenStandard: "NonFungible", Decimals: &zero, Available: true},
	)
	if !ok {
		t.Fatal("expected token-account scoped observation")
	}
	if observation.Kind != "token_account_recipient_unresolved" || observation.Destination != "DestinationATA" {
		t.Fatalf("unresolved recipient ATA was not preserved: %#v", observation)
	}
	if observation.Decimals == nil || *observation.Decimals != 0 || observation.TokenStandard != "NonFungible" {
		t.Fatalf("zero-decimal token metadata was lost: %#v", observation)
	}
}
