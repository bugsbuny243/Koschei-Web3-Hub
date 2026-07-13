package services

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const holderDeepConcurrencyMax = 3

type holderScanTierConfig struct {
	DeepSignatureLimit      int
	DeepTransactionLimit    int
	ShallowSignatureLimit   int
	ShallowTransactionLimit int
	DeepOwnerCount          int
	RPCBudget               int
}

type holderScanPlan struct {
	Tier             string
	SignatureLimit   int
	TransactionLimit int
	BudgetDegraded   bool
}

type holderScanRPCBudget struct {
	mu    sync.Mutex
	limit int
	used  int
}

func loadHolderScanTierConfig() holderScanTierConfig {
	return holderScanTierConfig{
		DeepSignatureLimit:      holderScanEnvInt("ARVIS_DEEP_SIG_LIMIT", 100, 20, 500),
		DeepTransactionLimit:    holderScanEnvInt("ARVIS_DEEP_TX_PARSE", 10, 2, 50),
		ShallowSignatureLimit:   holderScanEnvInt("ARVIS_SHALLOW_SIG_LIMIT", 20, 5, 100),
		ShallowTransactionLimit: holderScanEnvInt("ARVIS_SHALLOW_TX_PARSE", 2, 1, 10),
		DeepOwnerCount:          holderScanEnvInt("ARVIS_DEEP_OWNER_COUNT", 8, 0, 20),
		RPCBudget:               holderScanEnvInt("ARVIS_SCAN_RPC_BUDGET", 600, 25, 5000),
	}
}

func holderScanEnvInt(name string, fallback, min, max int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return fallback
	}
	return value
}

func newHolderScanRPCBudget(limit int) *holderScanRPCBudget {
	if limit <= 0 {
		limit = 600
	}
	return &holderScanRPCBudget{limit: limit}
}

func (b *holderScanRPCBudget) Reserve(calls int) bool {
	if b == nil || calls <= 0 {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.used+calls > b.limit {
		return false
	}
	b.used += calls
	return true
}

func (b *holderScanRPCBudget) Used() int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.used
}

func holderClusterRiskOwnerCandidates(accounts []HolderRoleAccount, limit int) []HolderRoleAccount {
	if limit <= 0 {
		return nil
	}
	byOwner := map[string]*HolderRoleAccount{}
	order := []string{}
	for _, account := range accounts {
		owner := strings.TrimSpace(account.OwnerWallet)
		if owner == "" || account.ExcludedFromHolderRisk || account.Role != "externally_owned_wallet" {
			continue
		}
		aggregate := byOwner[owner]
		if aggregate == nil {
			copy := account
			copy.OwnerWallet = owner
			copy.TokenAccount = ""
			copy.Evidence = append([]string{}, account.Evidence...)
			byOwner[owner] = &copy
			order = append(order, owner)
			continue
		}
		aggregate.Balance += account.Balance
		aggregate.RawPercentage += account.RawPercentage
		aggregate.CirculatingPercentage += account.CirculatingPercentage
		aggregate.Evidence = appendUniqueHolderEvidence(aggregate.Evidence, account.Evidence...)
		if account.Rank > 0 && (aggregate.Rank == 0 || account.Rank < aggregate.Rank) {
			aggregate.Rank = account.Rank
		}
	}
	out := make([]HolderRoleAccount, 0, len(order))
	for _, owner := range order {
		out = append(out, *byOwner[owner])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Balance > out[j].Balance })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func holderClusterAssignScanTiers(candidates []HolderRoleAccount, cfg holderScanTierConfig) []holderScanPlan {
	plans := make([]holderScanPlan, 0, len(candidates))
	remaining := cfg.RPCBudget
	deepCost := 1 + cfg.DeepTransactionLimit
	shallowCost := 1 + cfg.ShallowTransactionLimit
	for i := range candidates {
		plan := holderScanPlan{Tier: "shallow", SignatureLimit: cfg.ShallowSignatureLimit, TransactionLimit: cfg.ShallowTransactionLimit}
		if i < cfg.DeepOwnerCount {
			if remaining >= deepCost {
				plan = holderScanPlan{Tier: "deep", SignatureLimit: cfg.DeepSignatureLimit, TransactionLimit: cfg.DeepTransactionLimit}
				remaining -= deepCost
			} else {
				plan.BudgetDegraded = true
				if remaining >= shallowCost {
					remaining -= shallowCost
				} else {
					remaining = 0
				}
			}
		} else if remaining >= shallowCost {
			remaining -= shallowCost
		} else {
			plan.BudgetDegraded = true
			remaining = 0
		}
		plans = append(plans, plan)
	}
	return plans
}

func analyzeHolderClusterWalletTiered(ctx context.Context, rpcURL, mint string, account HolderRoleAccount, launchBlockTime int64, holderWallets map[string]bool, plan holderScanPlan, budget *holderScanRPCBudget) HolderClusterWallet {
	percentage := account.CirculatingPercentage
	if percentage <= 0 {
		percentage = account.RawPercentage
	}
	row := HolderClusterWallet{
		Rank: account.Rank, Wallet: account.OwnerWallet, HolderPercentage: holderClusterRound(percentage, 4),
		Status: "signature_history_unavailable", Tier: plan.Tier, BudgetDegraded: plan.BudgetDegraded,
		FlowObservations: []HolderClusterFlowObservation{}, Evidence: []string{},
	}
	if !budget.Reserve(1) {
		row.Status = "rpc_budget_exhausted"
		row.Tier = "shallow"
		row.BudgetDegraded = true
		row.Evidence = append(row.Evidence, "RPC budget exhausted before holder signature history could be fetched; no behavior claim was made.")
		return row
	}
	signatures, err := SolanaGetSignaturesForAddress(ctx, rpcURL, account.OwnerWallet, plan.SignatureLimit)
	if err != nil {
		row.Evidence = append(row.Evidence, "Holder signature history could not be fetched: "+compactClusterError(err))
		return row
	}
	row.SignaturesFetched = len(signatures)
	row.SignaturesObserved = len(signatures)
	row.WindowExhausted = len(signatures) < plan.SignatureLimit
	row.HistoryExhausted = row.WindowExhausted
	if len(signatures) == 0 {
		row.Status = "no_observed_signatures"
		row.Evidence = append(row.Evidence, fmt.Sprintf("No signatures were returned in the %s holder history window; this is not a safety signal.", row.Tier))
		return row
	}

	var oldestTime, newestTime, oldestSlot int64
	for _, signature := range signatures {
		if signature.BlockTime == nil || *signature.BlockTime <= 0 {
			continue
		}
		if newestTime == 0 || *signature.BlockTime > newestTime {
			newestTime = *signature.BlockTime
		}
		if oldestTime == 0 || *signature.BlockTime < oldestTime {
			oldestTime = *signature.BlockTime
			oldestSlot = signature.Slot
		}
	}
	row.OldestObservedSlot = oldestSlot
	row.OldestObservedAt = holderClusterUnixTime(oldestTime)
	row.NewestObservedAt = holderClusterUnixTime(newestTime)
	if row.WindowExhausted && launchBlockTime > 0 && oldestTime > 0 {
		delta := oldestTime - launchBlockTime
		row.FreshNearLaunch = delta >= -86400 && delta <= 86400
	}

	for _, index := range holderClusterTransactionIndexesForLimit(signatures, launchBlockTime, plan.TransactionLimit) {
		if index < 0 || index >= len(signatures) || signatures[index].Err != nil || strings.TrimSpace(signatures[index].Signature) == "" {
			continue
		}
		if !budget.Reserve(1) {
			row.BudgetDegraded = true
			row.Evidence = append(row.Evidence, "RPC budget reached while parsing holder transactions; partial evidence is preserved.")
			break
		}
		tx, txErr := SolanaGetTransactionJSONParsed(ctx, rpcURL, signatures[index].Signature)
		if txErr != nil {
			continue
		}
		row.ParsedTransactions++
		row.TxsParsed = row.ParsedTransactions
		txMap := map[string]any(tx)
		blockTime := holderClusterInt64(txMap["blockTime"])
		slot := holderClusterInt64(txMap["slot"])
		row.FlowObservations = append(row.FlowObservations, observeHolderClusterWalletFlow(txMap, signatures[index].Signature, mint, account.OwnerWallet, holderWallets)...)
		if row.FundingSource == "" {
			if source, amount := holderClusterFundingSource(txMap, account.OwnerWallet); source != "" && amount > 0 {
				row.FundingSource = source
				row.FundingAmountSOL = holderClusterRound(amount, 9)
				row.FundingObservedAt = holderClusterUnixTime(blockTime)
			}
		}
		if delta := holderClusterOwnerTokenDelta(txMap, mint, account.OwnerWallet); delta > 0.000000001 {
			if row.AcquisitionSlot == 0 || (slot > 0 && slot < row.AcquisitionSlot) {
				row.AcquisitionSlot = slot
				row.AcquisitionObservedAt = holderClusterUnixTime(blockTime)
			}
		}
	}
	if row.ParsedTransactions > 0 {
		row.Status = "verified_bounded_observation"
	} else {
		row.Status = "signature_only_observation"
	}
	row.Evidence = append(row.Evidence, fmt.Sprintf("%s tier observed %d signatures and parsed %d transactions; window_exhausted=%t.", row.Tier, row.SignaturesFetched, row.TxsParsed, row.WindowExhausted))
	if row.FreshNearLaunch {
		row.Evidence = append(row.Evidence, "Oldest observed wallet activity falls within 24 hours of the bounded token launch estimate.")
	}
	if row.FundingSource != "" {
		row.Evidence = append(row.Evidence, fmt.Sprintf("First bounded funding relation: %s supplied approximately %.9f SOL.", row.FundingSource, row.FundingAmountSOL))
	}
	if row.AcquisitionSlot > 0 {
		row.Evidence = append(row.Evidence, fmt.Sprintf("Positive target-token acquisition was observed at slot %d.", row.AcquisitionSlot))
	}
	return row
}

func holderClusterTransactionIndexesForLimit(signatures []SolanaSignatureInfo, launchBlockTime int64, limit int) []int {
	if limit <= 0 || len(signatures) == 0 {
		return nil
	}
	indexes := holderClusterTransactionIndexes(signatures, launchBlockTime)
	if len(indexes) > limit {
		return indexes[:limit]
	}
	seen := map[int]bool{}
	for _, index := range indexes {
		seen[index] = true
	}
	for i, signature := range signatures {
		if len(indexes) >= limit {
			break
		}
		if seen[i] || signature.Err != nil || strings.TrimSpace(signature.Signature) == "" {
			continue
		}
		seen[i] = true
		indexes = append(indexes, i)
	}
	return indexes
}

var _ = time.RFC3339
var _ = holderDeepConcurrencyMax
