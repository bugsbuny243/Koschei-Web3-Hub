package services

import (
	"context"
	"math/big"
	"strings"
)

type CreatorSellVerification struct {
	CandidateSignatures []string `json:"candidate_signatures"`
	VerifiedSignatures  []string `json:"verified_signatures"`
	TransactionsParsed  int      `json:"transactions_parsed"`
	RPCFailures         int      `json:"rpc_failures"`
	Limitations         []string `json:"limitations"`
}

// VerifyCreatorSellTransactions parses only the recent signatures already
// selected by the manual trade ledger. The acceleration rule remains OBSERVED:
// verified signatures support the narrative, while window rates and totals are
// still ledger-derived.
func VerifyCreatorSellTransactions(ctx context.Context, rpcURL string, sales CreatorSellAcceleration) CreatorSellVerification {
	out := CreatorSellVerification{
		CandidateSignatures: append([]string{}, sales.Signatures...),
		VerifiedSignatures:  []string{},
		Limitations:         []string{},
	}
	creator := strings.TrimSpace(sales.CreatorWallet)
	mint := strings.TrimSpace(sales.Mint)
	rpcURL = strings.TrimSpace(rpcURL)
	if creator == "" || mint == "" || len(out.CandidateSignatures) == 0 {
		return out
	}
	if rpcURL == "" {
		out.Limitations = append(out.Limitations, "Solana RPC yapılandırılmadığı için creator satış imzaları transaction seviyesinde doğrulanmadı.")
		return out
	}
	seen := map[string]bool{}
	for _, signature := range out.CandidateSignatures {
		signature = strings.TrimSpace(signature)
		if signature == "" || seen[signature] || ctx.Err() != nil {
			continue
		}
		seen[signature] = true
		tx, err := SolanaGetTransactionJSONParsed(ctx, rpcURL, signature)
		if err != nil {
			out.RPCFailures++
			continue
		}
		out.TransactionsParsed++
		txMap := map[string]any(tx)
		meta := actorFundingMap(txMap["meta"])
		if meta["err"] != nil {
			continue
		}
		message := actorFundingMap(actorFundingMap(txMap["transaction"])["message"])
		if !actorFundingSigners(message)[creator] {
			continue
		}
		if !unifiedCreatorMintBalanceDecreased(meta, creator, mint) {
			continue
		}
		if !unifiedTransactionHasSellMarker(message, meta) {
			continue
		}
		out.VerifiedSignatures = append(out.VerifiedSignatures, signature)
	}
	out.Limitations = append(out.Limitations, "Creator satış ivmesi OBSERVED kalır; doğrulanan imzalar yalnız transaction-backed destek sağlar.")
	return out
}

func unifiedCreatorMintBalanceDecreased(meta map[string]any, creator, mint string) bool {
	pre := unifiedTokenBalanceTotal(meta["preTokenBalances"], creator, mint)
	post := unifiedTokenBalanceTotal(meta["postTokenBalances"], creator, mint)
	return pre.Cmp(post) > 0
}

func unifiedTokenBalanceTotal(raw any, owner, mint string) *big.Int {
	total := new(big.Int)
	items, _ := raw.([]any)
	for _, item := range items {
		row := actorFundingMap(item)
		if strings.TrimSpace(actorFundingString(row["owner"])) != owner || strings.TrimSpace(actorFundingString(row["mint"])) != mint {
			continue
		}
		amount := strings.TrimSpace(actorFundingString(actorFundingMap(row["uiTokenAmount"])["amount"]))
		value := new(big.Int)
		if _, ok := value.SetString(amount, 10); ok {
			total.Add(total, value)
		}
	}
	return total
}

func unifiedTransactionHasSellMarker(message, meta map[string]any) bool {
	for _, instruction := range actorFundingInstructions(message, meta) {
		parsed := actorFundingMap(instruction["parsed"])
		kind := strings.ToLower(strings.TrimSpace(actorFundingString(parsed["type"])))
		if strings.Contains(kind, "sell") || strings.Contains(kind, "swap") {
			return true
		}
	}
	logs, _ := meta["logMessages"].([]any)
	for _, raw := range logs {
		line := strings.ToLower(strings.TrimSpace(actorFundingString(raw)))
		if strings.Contains(line, "instruction: sell") || strings.Contains(line, "instruction: swap") {
			return true
		}
	}
	return false
}
