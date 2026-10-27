package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const actorFundingOldestFirstLimit = 25

// FindActorFundingOriginOldestFirst reads a wallet's first successful
// transactions with Helius getTransactionsForAddress (sortOrder asc). The
// newest-first signature walk cannot reach the start of a busy wallet's
// history; this reads it directly, so the earliest system transfer into the
// wallet is its initial funding. ok is false when Helius is unavailable or no
// funding transfer appears in the first page; the caller then keeps the
// bounded result unchanged.
func FindActorFundingOriginOldestFirst(ctx context.Context, rpcURL, wallet string) (ActorFundingOrigin, bool) {
	wallet = strings.TrimSpace(wallet)
	apiKey := heliusEnhancedAPIKey(rpcURL)
	if wallet == "" || apiKey == "" {
		return ActorFundingOrigin{}, false
	}
	rows, err := fetchHeliusOldestTransactions(ctx, heliusRPCProviderURL(rpcURL, apiKey), wallet)
	if err != nil || len(rows) == 0 {
		return ActorFundingOrigin{}, false
	}
	return actorFundingOriginFromOldestTransactions(wallet, rows)
}

// actorFundingOriginFromOldestTransactions picks the first system transfer
// into wallet from transactions already ordered oldest first.
func actorFundingOriginFromOldestTransactions(wallet string, rows []map[string]any) (ActorFundingOrigin, bool) {
	parsed := 0
	for _, tx := range rows {
		meta := actorFundingMap(tx["meta"])
		if meta["err"] != nil {
			continue
		}
		transaction := actorFundingMap(tx["transaction"])
		message := actorFundingMap(transaction["message"])
		signatures, _ := transaction["signatures"].([]any)
		signature := ""
		if len(signatures) > 0 {
			signature = strings.TrimSpace(actorFundingString(signatures[0]))
		}
		if signature == "" {
			continue
		}
		parsed++
		info := SolanaSignatureInfo{Signature: signature, Slot: actorFundingInt64(tx["slot"])}
		observedAt := actorFundingTransactionTime(info, tx)
		signers := actorFundingSigners(message)
		for _, instruction := range actorFundingInstructions(message, meta) {
			candidate, ok := actorFundingInstructionCandidate(wallet, info, observedAt, signers, instruction)
			if !ok {
				continue
			}
			origin := ActorFundingOrigin{
				Wallet: wallet, Status: "initial_funding_observed",
				SourceWallet: candidate.Source, DestinationWallet: candidate.Destination,
				AmountSOL: float64(candidate.Lamports) / 1e9, Signature: candidate.Signature,
				Slot: candidate.Slot, ObservedAt: candidate.ObservedAt,
				Program: "system", InstructionType: candidate.InstructionType,
				VerificationStatus: "observed", TrailStatus: "source_wallet_observed",
				IdentityScope: "onchain_wallet_only", TransactionsParsed: parsed,
				ResultState: ActorFundingResultVerified,
				Boundary: ActorFundingBoundary{
					Kind: "oldest_first_history", TransactionsParsed: parsed,
					OldestSignature: signature, OldestSlot: info.Slot,
				},
				Limitations: []string{
					"Read from the wallet's first successful transactions in chain order (Helius getTransactionsForAddress, oldest first).",
					"Funding source yalnız zincir üstü cüzdan olarak raporlanır; CEX veya gerçek kişi etiketi ayrı doğrulanmadıkça kimlik iddiası üretilmez.",
				},
			}
			if candidate.SourceSigned && candidate.Slot > 0 && !candidate.ObservedAt.IsZero() {
				origin.VerificationStatus = "verified"
			}
			return origin, true
		}
	}
	return ActorFundingOrigin{}, false
}

func fetchHeliusOldestTransactions(ctx context.Context, endpoint, wallet string) ([]map[string]any, error) {
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "koschei-funding-origin-oldest",
		"method":  "getTransactionsForAddress",
		"params": []any{wallet, map[string]any{
			"transactionDetails":             "full",
			"sortOrder":                      "asc",
			"limit":                          actorFundingOldestFirstLimit,
			"encoding":                       "jsonParsed",
			"maxSupportedTransactionVersion": SolanaMaxSupportedTransactionVersion,
			"filters":                        map[string]any{"status": "succeeded"},
		}},
	})
	if err != nil {
		return nil, err
	}
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	body, err := postHeliusRPC(reqCtx, endpoint, payload, 16<<20)
	if err != nil {
		return nil, err
	}
	var response heliusTransactionsForAddressResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("helius oldest history decode: %w", err)
	}
	if response.Error != nil {
		return nil, fmt.Errorf("helius oldest history error %d: %s", response.Error.Code, strings.TrimSpace(response.Error.Message))
	}
	return response.Result.Data, nil
}
