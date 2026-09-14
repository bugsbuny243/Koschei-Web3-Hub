package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// FetchHeliusCreatedMintDiscoveryArchival performs an explicit, bounded Helius
// archival walk for an already-resolved actor wallet. Unlike
// FetchHeliusCreatedMintDiscovery, this function does not consult
// HELIUS_CREATED_MINT_ARCHIVAL_ENABLED: the caller is the opt-in boundary.
//
// Discovery remains OBSERVED-only. The handler must re-read every candidate
// from canonical Solana RPC and verify actor signer + exact create/initialize
// semantics before any candidate becomes VERIFIED actor evidence.
func FetchHeliusCreatedMintDiscoveryArchival(ctx context.Context, rpcURL, wallet string) CreatedMintDiscovery {
	wallet = strings.TrimSpace(wallet)
	out := CreatedMintDiscovery{
		Status:      "not_configured",
		Provider:    "helius_get_transactions_for_address",
		Wallet:      wallet,
		Candidates:  []ActorCreatedMintCandidate{},
		ObservedAt:  time.Now().UTC(),
		Limitations: []string{},
	}
	if wallet == "" {
		out.Status = "wallet_required"
		out.Limitations = append(out.Limitations, "Creator wallet is required for created-mint discovery.")
		return out
	}

	apiKey := heliusEnhancedAPIKey(rpcURL)
	if apiKey == "" {
		out.Limitations = append(out.Limitations, "No Helius API key resolved; explicit archival created-mint discovery was skipped.")
		return out
	}
	out.Configured = true

	endpoint := heliusRPCProviderURL(rpcURL, apiKey)
	maxPages := holderScanEnvInt("HELIUS_CREATED_MINT_MAX_PAGES", 6, 1, 20)
	pageLimit := holderScanEnvInt("HELIUS_CREATED_MINT_PAGE_LIMIT", 250, 10, 1000)
	pageDelay := time.Duration(holderScanEnvInt("HELIUS_CREATED_MINT_PAGE_DELAY_MS", 150, 0, 2000)) * time.Millisecond

	paginationToken := ""
	candidateIndex := map[string]ActorCreatedMintCandidate{}
	for page := 0; page < maxPages && ctx.Err() == nil; page++ {
		if page > 0 && pageDelay > 0 {
			select {
			case <-ctx.Done():
				out.Limitations = append(out.Limitations, "Creator discovery stopped: context deadline.")
				return out
			case <-time.After(pageDelay):
			}
		}

		batch, next, err := fetchHeliusCreatedMintPage(ctx, endpoint, wallet, paginationToken, pageLimit)
		if err != nil {
			if out.PagesFetched == 0 {
				out.Status = "collection_failed"
			} else {
				out.Status = "partial"
			}
			out.Limitations = append(out.Limitations, "Helius creator history could not be collected: "+compactClusterError(err))
			break
		}

		out.PagesFetched++
		out.TransactionsSeen += len(batch)
		for _, candidate := range ExtractActorCreatedMintCandidates(batch, wallet, out.Provider) {
			key := candidate.Mint + "|" + candidate.Signature
			if existing, ok := candidateIndex[key]; !ok || candidate.Slot > existing.Slot {
				candidateIndex[key] = candidate
			}
		}
		if strings.TrimSpace(next) == "" || next == paginationToken {
			paginationToken = ""
			break
		}
		paginationToken = next
	}

	out.NextCursor = paginationToken
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
		case paginationToken != "":
			out.Status = "bounded"
		case len(out.Candidates) == 0:
			out.Status = "complete_no_created_mints_observed"
		default:
			out.Status = "complete"
		}
	}
	if paginationToken != "" {
		out.Limitations = append(out.Limitations, fmt.Sprintf("Created-mint discovery stopped after %d Helius pages; pagination token is preserved.", out.PagesFetched))
	}
	return out
}
