package services

import (
	"context"
	"fmt"
	"strings"
)

func traceLaunchFunding(ctx context.Context, rpcURL, creator, creatorFunder string, profile *LaunchActorProfile, cfg launchForensicsConfig, budget *holderScanRPCBudget) {
	if profile == nil {
		return
	}
	profile.FundingStatus = "funding_not_traced"
	if strings.TrimSpace(rpcURL) == "" || strings.TrimSpace(profile.OwnerWallet) == "" {
		profile.Evidence = append(profile.Evidence, "Funding not traced: Solana RPC or owner wallet is unavailable.")
		return
	}
	if budget.Used() >= cfg.FundingRPCBudget {
		profile.FundingStatus = "funding_budget_exhausted"
		profile.Evidence = append(profile.Evidence, "Funding not traced: per-scan funding RPC budget was exhausted.")
		return
	}
	first, amount := earliestIncomingSOLSource(ctx, rpcURL, profile.OwnerWallet, cfg, budget)
	if first == "" {
		if budget.Used() >= cfg.FundingRPCBudget {
			profile.FundingStatus = "funding_budget_exhausted"
		}
		profile.Evidence = append(profile.Evidence, "Funding not traced: no verified incoming SOL source was resolved in the bounded two-page history.")
		return
	}
	profile.FundingStatus = "funding_source_observed"
	profile.FundingPath = []string{profile.OwnerWallet, first}
	profile.FundingHops = 1
	profile.Evidence = append(profile.Evidence, fmt.Sprintf("İlk doğrulanmış SOL kaynağı %s (yaklaşık %.6f SOL).", first, amount))
	if fundingMatches(first, creator, creatorFunder) {
		markCreatorLinked(profile, creator, creatorFunder)
		return
	}
	if budget.Used() >= cfg.FundingRPCBudget {
		profile.FundingStatus = "funding_budget_exhausted"
		profile.Evidence = append(profile.Evidence, "İkinci funding hop'u bütçe nedeniyle izlenemedi.")
		return
	}
	second, secondAmount := earliestIncomingSOLSource(ctx, rpcURL, first, cfg, budget)
	if second == "" {
		return
	}
	profile.FundingPath = append(profile.FundingPath, second)
	profile.FundingHops = 2
	profile.Evidence = append(profile.Evidence, fmt.Sprintf("İkinci funding hop'u %s (yaklaşık %.6f SOL).", second, secondAmount))
	if fundingMatches(second, creator, creatorFunder) {
		markCreatorLinked(profile, creator, creatorFunder)
	}
}

func earliestIncomingSOLSource(ctx context.Context, rpcURL, wallet string, cfg launchForensicsConfig, budget *holderScanRPCBudget) (string, float64) {
	wallet = strings.TrimSpace(wallet)
	if wallet == "" || strings.TrimSpace(rpcURL) == "" {
		return "", 0
	}
	before := ""
	signatures := []SolanaSignatureInfo{}
	for page := 0; page < 2; page++ {
		if ctx.Err() != nil || !budget.Reserve(1) {
			return "", 0
		}
		pageRows, err := SolanaGetSignaturesForAddressBefore(ctx, rpcURL, wallet, cfg.FundingSigLimit, before)
		if err != nil {
			return "", 0
		}
		signatures = append(signatures, pageRows...)
		if len(pageRows) < cfg.FundingSigLimit || len(pageRows) == 0 {
			break
		}
		before = pageRows[len(pageRows)-1].Signature
	}
	if len(signatures) == 0 || !budget.Reserve(1) {
		return "", 0
	}
	// Oldest observations are the most useful for initial-funding evidence.
	start := 0
	if len(signatures) > 40 {
		start = len(signatures) - 40
	}
	keys := make([]string, 0, len(signatures)-start)
	for i := len(signatures) - 1; i >= start; i-- {
		if signatures[i].Err == nil && strings.TrimSpace(signatures[i].Signature) != "" {
			keys = append(keys, signatures[i].Signature)
		}
	}
	transactions, err := SolanaGetTransactionsJSONParsedBatch(ctx, rpcURL, keys)
	if err != nil {
		return "", 0
	}
	for _, signature := range keys {
		tx, ok := transactions[signature]
		if !ok {
			continue
		}
		if source, amount := holderClusterFundingSource(map[string]any(tx), wallet); source != "" && amount > 0 {
			return source, amount
		}
	}
	return "", 0
}

func fundingMatches(value, creator, creatorFunder string) bool {
	value = strings.TrimSpace(value)
	return value != "" && (strings.EqualFold(value, strings.TrimSpace(creator)) || strings.EqualFold(value, strings.TrimSpace(creatorFunder)))
}

func markCreatorLinked(profile *LaunchActorProfile, creator, creatorFunder string) {
	profile.CreatorLinked = true
	profile.FundingStatus = "creator_linked"
	label := strings.TrimSpace(creator)
	if label == "" {
		label = strings.TrimSpace(creatorFunder)
	}
	profile.Evidence = append(profile.Evidence, fmt.Sprintf("CREATOR_LINKED: funding zinciri %d hop içinde creator/deployer veya creator fonlayıcısına ulaştı (%s).", profile.FundingHops, label))
}
