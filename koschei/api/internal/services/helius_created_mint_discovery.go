package services

// Helius-based created-mint discovery.
//
// This is the Helius counterpart to solscan_created_mint_discovery.go. It finds
// the pump.fun / SPL token launches attributable to a creator wallet using the
// Helius Enhanced Transactions API instead of Solscan, so created-mint
// discovery works with the RPC provider Koschei already uses — no Solscan Pro
// subscription required.
//
// Helius Enhanced tags token creation with a transaction type (TOKEN_MINT /
// CREATE) and exposes the minted address in tokenTransfers[0].mint, so no raw
// message/accountKeys parsing is needed. Output is the same
// ActorCreatedMintCandidate type the Solscan path produces, so every downstream
// consumer (RPC re-verification, evidence persistence) is unchanged.
//
// Design rules honored:
//   - Reuses heliusEnhancedAPIKey + fetchHeliusEnhancedTransactionsPage. No new
//     credentials. If no key resolves, this is a no-op.
//   - Discovery candidates are still only *candidates*: each is independently
//     re-verified from canonical RPC upstream before becoming VERIFIED evidence.
//     A Helius-tagged type is a discovery hint, not proof.
//   - Budget-bounded pagination so a prolific creator can't exhaust quota.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// heliusEnhancedTypedTransaction extends the enhanced-history shape with the
// fields needed to attribute and classify a creation transaction. Helius
// returns these on the same /transactions payload; the holder-history collector
// simply didn't decode them.
type heliusEnhancedTypedTransaction struct {
	Signature      string                `json:"signature"`
	Slot           int64                 `json:"slot"`
	Timestamp      int64                 `json:"timestamp"`
	Type           string                `json:"type"`
	FeePayer       string                `json:"feePayer"`
	Source         string                `json:"source"`
	TokenTransfers []heliusTokenTransfer `json:"tokenTransfers"`
	Instructions   []heliusInstruction   `json:"instructions"`
}

// FetchHeliusCreatedMintDiscovery discovers created-mint candidates for a
// creator wallet from Helius Enhanced transactions. It mirrors the return
// contract of FetchSolscanCreatedMintDiscovery so it can be used as a drop-in
// alternative.
func FetchHeliusCreatedMintDiscovery(ctx context.Context, rpcURL, wallet string) SolscanCreatedMintDiscovery {
	wallet = strings.TrimSpace(wallet)
	out := SolscanCreatedMintDiscovery{
		Status: "not_configured", Provider: "helius_enhanced_transactions",
		Wallet: wallet, Candidates: []ActorCreatedMintCandidate{},
		ObservedAt: time.Now().UTC(), Limitations: []string{},
	}
	if wallet == "" {
		out.Status = "wallet_required"
		out.Limitations = append(out.Limitations, "Creator wallet is required for created-mint discovery.")
		return out
	}

	apiKey := heliusEnhancedAPIKey(rpcURL)
	if apiKey == "" {
		out.Limitations = append(out.Limitations, "No Helius API key resolved; Helius created-mint discovery was skipped.")
		return out
	}
	out.Configured = true

	maxPages := holderScanEnvInt("HELIUS_CREATED_MINT_MAX_PAGES", 10, 1, 30)
	pageLimit := holderScanEnvInt("HELIUS_CREATED_MINT_PAGE_LIMIT", 100, 10, 100)

	before := ""
	candidateIndex := map[string]ActorCreatedMintCandidate{}
	for page := 0; page < maxPages && ctx.Err() == nil; page++ {
		batch, err := fetchHeliusEnhancedTypedTransactionsPage(ctx, apiKey, wallet, before, pageLimit)
		if err != nil {
			if out.PagesFetched == 0 {
				out.Status = "enhanced_endpoint_unavailable"
			} else {
				out.Status = "partial"
			}
			out.Limitations = append(out.Limitations, "Helius enhanced creator page could not be collected: "+compactClusterError(err))
			break
		}
		out.PagesFetched++
		out.TransactionsSeen += len(batch)
		for _, candidate := range extractHeliusCreatedMintCandidates(batch, wallet) {
			key := candidate.Mint + "|" + candidate.Signature
			if existing, ok := candidateIndex[key]; !ok || candidate.Slot > existing.Slot {
				candidateIndex[key] = candidate
			}
		}
		if len(batch) < pageLimit {
			break // history exhausted
		}
		before = batch[len(batch)-1].Signature
	}

	for _, candidate := range candidateIndex {
		out.Candidates = append(out.Candidates, candidate)
	}
	sort.SliceStable(out.Candidates, func(i, j int) bool {
		if out.Candidates[i].Slot != out.Candidates[j].Slot {
			return out.Candidates[i].Slot > out.Candidates[j].Slot
		}
		return out.Candidates[i].Mint < out.Candidates[j].Mint
	})
	out.Available = out.PagesFetched > 0
	if out.Status == "not_configured" {
		switch {
		case out.PagesFetched == 0:
			out.Status = "collection_failed"
		case len(out.Candidates) == 0:
			out.Status = "complete_no_created_mints_observed"
		default:
			out.Status = "complete"
		}
	}
	return out
}

// extractHeliusCreatedMintCandidates identifies token-creation transactions
// signed by the creator from a Helius Enhanced batch. Helius already classifies
// the transaction type, so this trusts the TOKEN_MINT/CREATE tag plus a pump/
// token source and reads the minted address from tokenTransfers.
func extractHeliusCreatedMintCandidates(transactions []heliusEnhancedTypedTransaction, actorWallet string) []ActorCreatedMintCandidate {
	actorWallet = strings.TrimSpace(actorWallet)
	if actorWallet == "" {
		return []ActorCreatedMintCandidate{}
	}
	results := []ActorCreatedMintCandidate{}
	for _, tx := range transactions {
		if strings.TrimSpace(tx.Signature) == "" {
			continue
		}
		// Creator attribution: the fee payer is the transaction signer on
		// pump.fun single-signer creations. This is a discovery hint; upstream
		// RPC re-verification confirms the actual create-instruction signer.
		if !strings.EqualFold(strings.TrimSpace(tx.FeePayer), actorWallet) {
			continue
		}

		txType := strings.ToUpper(strings.TrimSpace(tx.Type))
		source := strings.ToLower(strings.TrimSpace(tx.Source))
		isCreation := txType == "TOKEN_MINT" || txType == "CREATE" ||
			strings.Contains(txType, "CREATE") ||
			(strings.Contains(source, "pump") && txType != "SWAP" && txType != "TRANSFER")
		if !isCreation {
			continue
		}

		mint := ""
		for _, transfer := range tx.TokenTransfers {
			if m := strings.TrimSpace(transfer.Mint); m != "" {
				mint = m
				break
			}
		}
		if mint == "" {
			continue // no mint resolvable; not a usable candidate
		}

		program := ""
		if len(tx.Instructions) > 0 {
			program = strings.TrimSpace(tx.Instructions[0].ProgramID)
		}
		if strings.Contains(source, "pump") && program == "" {
			program = canonicalPumpFunProgramID
		}

		observedAt := time.Time{}
		if tx.Timestamp > 0 {
			observedAt = time.Unix(tx.Timestamp, 0).UTC()
		}

		results = append(results, ActorCreatedMintCandidate{
			Mint:               mint,
			Signature:          tx.Signature,
			Slot:               tx.Slot,
			BlockTime:          tx.Timestamp,
			ObservedAt:         observedAt,
			Program:            program,
			InstructionType:    strings.ToLower(txType),
			ActorSigned:        true, // fee payer == actor; RPC re-verify confirms
			VerificationStatus: "discovery_candidate",
			Source:             "helius_enhanced_transactions",
		})
	}
	return results
}

// fetchHeliusEnhancedTypedTransactionsPage is the typed-decode variant of
// fetchHeliusEnhancedTransactionsPage: same endpoint and pagination, but it
// decodes the type/feePayer/source fields needed for creation attribution.
func fetchHeliusEnhancedTypedTransactionsPage(ctx context.Context, apiKey, address, before string, limit int) ([]heliusEnhancedTypedTransaction, error) {
	if limit <= 0 || limit > heliusEnhancedPageSize {
		limit = heliusEnhancedPageSize
	}
	endpoint := fmt.Sprintf("%s/%s/transactions", heliusEnhancedBaseURL, url.PathEscape(strings.TrimSpace(address)))
	query := url.Values{}
	query.Set("api-key", apiKey)
	query.Set("limit", fmt.Sprintf("%d", limit))
	if strings.TrimSpace(before) != "" {
		query.Set("before", strings.TrimSpace(before))
	}
	reqCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("helius enhanced api status %d", res.StatusCode)
	}
	var out []heliusEnhancedTypedTransaction
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("helius enhanced api decode: %w", err)
	}
	return out, nil
}
