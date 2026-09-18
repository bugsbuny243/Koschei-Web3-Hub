package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

const launchATAConcurrency = 4
const launchTransactionBatchSize = 40
const launchATAMinTransactionReserve = 2

func launchATAFairShareBudgets(total, owners int) []int {
	if owners <= 0 {
		return nil
	}
	out := make([]int, owners)
	if total <= 0 {
		return out
	}
	base := total / owners
	remainder := total % owners
	for index := range out {
		out[index] = base
		if index < remainder {
			out[index]++
		}
	}
	return out
}

func runLaunchATARankedWorkerPool(ctx context.Context, total, workers int, work func(index int)) {
	if total <= 0 || work == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if workers < 1 {
		workers = 1
	}
	if workers > total {
		workers = total
	}

	jobs := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				if ctx.Err() != nil {
					return
				}
				work(index)
			}
		}()
	}

sendLoop:
	for index := 0; index < total; index++ {
		select {
		case <-ctx.Done():
			break sendLoop
		case jobs <- index:
		}
	}
	close(jobs)
	wg.Wait()
}

func analyzeLaunchForensicsATA(ctx context.Context, rpcURL, mint string, candidates []launchOwnerCandidate, launchTime time.Time, launchSlot int64, cfg launchForensicsConfig, budget *holderScanRPCBudget) []LaunchActorProfile {
	if len(candidates) == 0 {
		return nil
	}
	totalBudget := cfg.RPCBudget
	if budget != nil {
		totalBudget = budget.Remaining()
	}
	ownerBudgets := launchATAFairShareBudgets(totalBudget, len(candidates))
	profiles := make([]LaunchActorProfile, len(candidates))
	runLaunchATARankedWorkerPool(ctx, len(candidates), launchATAConcurrency, func(index int) {
		quota := ownerBudgets[index]
		if quota <= 0 {
			profiles[index] = LaunchActorProfile{
				OwnerWallet: candidates[index].OwnerWallet, TokenAccounts: append([]string{}, candidates[index].TokenAccounts...),
				Label: "HISTORY_NOT_CAPTURED", FundingStatus: "not_checked", Source: "ata_history",
				Evidence: []string{"Launch ATA fair-share RPC budget did not allocate a call to this owner; missing history is not treated as safe evidence."},
			}
			return
		}
		localBudget := newHolderScanRPCBudget(quota)
		profile := analyzeLaunchOwnerATA(ctx, rpcURL, mint, candidates[index], launchTime, launchSlot, cfg, localBudget)
		used := localBudget.Used()
		if budget != nil && used > 0 {
			if !budget.Reserve(used) {
				profile.Evidence = append(profile.Evidence, "Launch ATA fair-share accounting rejected local RPC usage; evidence was preserved but the scan remains bounded.")
			}
		}
		profile.Evidence = append(profile.Evidence, fmt.Sprintf("Launch ATA fair-share quota: %d RPC calls allocated; %d used.", quota, used))
		profiles[index] = profile
	})
	out := make([]LaunchActorProfile, 0, len(profiles))
	for i, profile := range profiles {
		if strings.TrimSpace(profile.OwnerWallet) == "" {
			profile = LaunchActorProfile{
				OwnerWallet: candidates[i].OwnerWallet, TokenAccounts: append([]string{}, candidates[i].TokenAccounts...),
				Label: "HISTORY_NOT_CAPTURED", FundingStatus: "not_checked", Source: "ata_history",
				Evidence: []string{"ATA taraması istek süresi veya RPC bütçesi nedeniyle tamamlanamadı; sonuç güvenli sayılmadı."},
			}
		}
		out = append(out, profile)
	}
	return out
}

type launchATASignaturePageFetcher func(context.Context, string, int, string) ([]SolanaSignatureInfo, error)

type launchATAPageState struct {
	address   string
	before    string
	pages     int
	exhausted bool
	failed    bool
}

func collectLaunchATASignaturesRoundRobin(
	ctx context.Context,
	tokenAccounts []string,
	maxPages int,
	signatureLimit int,
	budget *holderScanRPCBudget,
	fetch launchATASignaturePageFetcher,
) (map[string]SolanaSignatureInfo, bool, []string) {
	bySignature := map[string]SolanaSignatureInfo{}
	limitations := []string{}
	if ctx == nil {
		ctx = context.Background()
	}
	if maxPages <= 0 || signatureLimit <= 0 || fetch == nil {
		return bySignature, false, []string{"ATA signature pagination configuration is incomplete; history was not treated as exhausted."}
	}

	states := make([]launchATAPageState, 0, len(tokenAccounts))
	seenAccounts := map[string]bool{}
	for _, tokenAccount := range tokenAccounts {
		tokenAccount = strings.TrimSpace(tokenAccount)
		if tokenAccount == "" || seenAccounts[tokenAccount] {
			continue
		}
		seenAccounts[tokenAccount] = true
		states = append(states, launchATAPageState{address: tokenAccount})
	}
	if len(states) == 0 {
		return bySignature, false, []string{"No non-empty mint-specific token account was available for ATA history pagination."}
	}

	stoppedEarly := false
pagination:
	for page := 0; page < maxPages; page++ {
		activeThisRound := false
		for index := range states {
			state := &states[index]
			if state.exhausted || state.failed || state.pages >= maxPages {
				continue
			}
			activeThisRound = true

			// Give every still-eligible ATA its turn for the current page before
			// advancing any account to the next page. Once signatures exist,
			// preserve transaction-parse capacity after first-page coverage.
			firstPagesPending := 0
			for check := range states {
				if !states[check].exhausted && !states[check].failed && states[check].pages == 0 {
					firstPagesPending++
				}
			}
			if len(bySignature) > 0 && budget != nil && state.pages > 0 && budget.Remaining() <= launchATAMinTransactionReserve {
				limitations = append(limitations, "ATA round-robin pagination preserved remaining RPC calls for parsed transaction verification.")
				stoppedEarly = true
				break pagination
			}
			if len(bySignature) > 0 && budget != nil && state.pages == 0 && firstPagesPending > 0 &&
				budget.Remaining() <= launchATAMinTransactionReserve+firstPagesPending-1 {
				limitations = append(limitations, "ATA round-robin pagination could not cover every first page while preserving parsed transaction capacity.")
				stoppedEarly = true
				break pagination
			}
			if ctx.Err() != nil || !budget.Reserve(1) {
				limitations = append(limitations, "ATA signature pagination stopped at the request or RPC budget boundary.")
				stoppedEarly = true
				break pagination
			}

			signatures, err := fetch(ctx, state.address, signatureLimit, state.before)
			state.pages++
			if err != nil {
				state.failed = true
				limitations = append(limitations, "ATA signature history could not be read: "+compactClusterError(err))
				continue
			}
			for _, signature := range signatures {
				if signature.Signature != "" {
					bySignature[signature.Signature] = signature
				}
			}
			if len(signatures) < signatureLimit || len(signatures) == 0 ||
				strings.TrimSpace(signatures[len(signatures)-1].Signature) == "" {
				state.exhausted = true
				continue
			}
			state.before = signatures[len(signatures)-1].Signature
		}
		if !activeThisRound {
			break
		}
	}

	allExhausted := !stoppedEarly
	for _, state := range states {
		if !state.exhausted {
			allExhausted = false
			break
		}
	}
	return bySignature, allExhausted, limitations
}

func analyzeLaunchOwnerATA(ctx context.Context, rpcURL, mint string, candidate launchOwnerCandidate, launchTime time.Time, launchSlot int64, cfg launchForensicsConfig, budget *holderScanRPCBudget) LaunchActorProfile {
	base := LaunchActorProfile{
		OwnerWallet: candidate.OwnerWallet, TokenAccounts: append([]string{}, candidate.TokenAccounts...),
		Label: "HISTORY_NOT_CAPTURED", FundingStatus: "not_checked", Source: "ata_history", Evidence: []string{},
	}
	if len(candidate.TokenAccounts) == 0 {
		base.Evidence = append(base.Evidence, "Owner için çözümlenmiş token account/ATA bulunamadı.")
		return base
	}
	bySignature, allExhausted, paginationLimitations := collectLaunchATASignaturesRoundRobin(
		ctx,
		candidate.TokenAccounts,
		cfg.ATAMaxPages,
		cfg.ATASignatureLimit,
		budget,
		func(fetchCtx context.Context, tokenAccount string, limit int, before string) ([]SolanaSignatureInfo, error) {
			return SolanaGetSignaturesForAddressBefore(fetchCtx, rpcURL, tokenAccount, limit, before)
		},
	)
	base.Evidence = append(base.Evidence, paginationLimitations...)
	base.SignaturesFetched = len(bySignature)
	base.WindowExhausted = allExhausted
	if len(bySignature) == 0 {
		base.Evidence = append(base.Evidence, "ATA geçmişinde hedef-token işlemi bulunamadı; bu yokluk güvenlik sinyali değildir.")
		return base
	}

	signatures := make([]SolanaSignatureInfo, 0, len(bySignature))
	for _, signature := range bySignature {
		if signature.Err == nil {
			signatures = append(signatures, signature)
		}
	}
	sort.SliceStable(signatures, func(i, j int) bool {
		if signatures[i].BlockTime != nil && signatures[j].BlockTime != nil && *signatures[i].BlockTime != *signatures[j].BlockTime {
			return *signatures[i].BlockTime < *signatures[j].BlockTime
		}
		return signatures[i].Slot < signatures[j].Slot
	})
	trades := []LaunchTrade{}
	for start := 0; start < len(signatures); start += launchTransactionBatchSize {
		if ctx.Err() != nil {
			base.Evidence = append(base.Evidence, "ATA işlem ayrıştırması istek süresi nedeniyle kısmi kaldı.")
			break
		}
		end := start + launchTransactionBatchSize
		if end > len(signatures) {
			end = len(signatures)
		}
		keys := make([]string, 0, end-start)
		for _, signature := range signatures[start:end] {
			keys = append(keys, signature.Signature)
		}
		granted := budget.ReserveUpTo(len(keys))
		if granted == 0 {
			base.Evidence = append(base.Evidence, "ATA işlem ayrıştırma bütçesi doldu; kısmi geçmiş korundu.")
			break
		}
		keys = keys[:granted]
		transactions, batchErr := SolanaGetTransactionsJSONParsedBatch(ctx, rpcURL, keys)
		if batchErr != nil {
			base.Evidence = append(base.Evidence, "ATA parsed transaction batch alınamadı: "+compactClusterError(batchErr))
		}
		base.TransactionsParsed += len(transactions)
		for _, signature := range signatures[start : start+granted] {
			tx, ok := transactions[signature.Signature]
			if !ok {
				continue
			}
			txMap := map[string]any(tx)
			delta := holderClusterOwnerTokenDelta(txMap, mint, candidate.OwnerWallet)
			if math.Abs(delta) <= holderClusterFlowEpsilon {
				continue
			}
			side := "buy"
			if delta < 0 {
				side = "sell"
			}
			blockTime := holderClusterInt64(txMap["blockTime"])
			trades = append(trades, LaunchTrade{
				Mint: mint, Trader: candidate.OwnerWallet, Side: side,
				TokenAmount: math.Abs(delta), SOLAmount: math.Abs(launchOwnerSOLDelta(txMap, candidate.OwnerWallet)),
				Slot: holderClusterInt64(txMap["slot"]), BlockTime: launchUnixTime(blockTime),
				Signature: signature.Signature, Source: "ata_history", Program: launchCounterpartyProgram(txMap),
			})
		}
		if IsSolanaRPCBatchUnavailable(batchErr) {
			base.Evidence = append(base.Evidence, "RPC provider batch erişimini reddetti; kalan ATA işlemleri tekrar denenmeden kısmi kanıt korundu.")
			break
		}
		if batchErr != nil && len(transactions) == 0 {
			continue
		}
	}
	classified := classifyLaunchActors(trades, launchSlot, launchTime, cfg.SniperSlotWindow)
	if len(classified) == 0 {
		base.Evidence = append(base.Evidence, "ATA transactionları parse edildi ancak owner için hedef-token bakiye değişimi doğrulanmadı.")
		return base
	}
	profile := classified[0]
	profile.TokenAccounts = append([]string{}, candidate.TokenAccounts...)
	profile.Source = "ata_history"
	profile.WindowExhausted = base.WindowExhausted
	profile.SignaturesFetched = base.SignaturesFetched
	profile.TransactionsParsed = base.TransactionsParsed
	profile.FundingStatus = "not_checked"
	profile.Evidence = append(profile.Evidence, base.Evidence...)
	profile.Evidence = append(profile.Evidence, launchCoverageEvidence(profile))
	return profile
}

func launchOwnerSOLDelta(tx map[string]any, wallet string) float64 {
	message := holderClusterMap(holderClusterMap(tx["transaction"])["message"])
	meta := holderClusterMap(tx["meta"])
	keys := holderClusterAccountKeys(message["accountKeys"])
	pre := holderClusterNumberSlice(meta["preBalances"])
	post := holderClusterNumberSlice(meta["postBalances"])
	limit := len(keys)
	if len(pre) < limit {
		limit = len(pre)
	}
	if len(post) < limit {
		limit = len(post)
	}
	for i := 0; i < limit; i++ {
		if keys[i] == wallet {
			return float64(post[i]-pre[i]) / 1e9
		}
	}
	return 0
}

func launchCounterpartyProgram(tx map[string]any) string {
	message := holderClusterMap(holderClusterMap(tx["transaction"])["message"])
	for _, key := range holderClusterAccountKeys(message["accountKeys"]) {
		switch {
		case key == defaultPumpProgramID:
			return "pump.fun"
		case key == defaultPumpSwapProgramID, key == pumpLiquidityProgramID:
			return "pumpswap"
		case isKnownRaydiumProgram(key):
			return "raydium"
		}
	}
	return ""
}

func launchUnixTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.Unix(value, 0).UTC()
}

func launchCoverageEvidence(profile LaunchActorProfile) string {
	window := "pencere tam tükendi"
	if !profile.WindowExhausted {
		window = "maksimum sayfa sınırında kısmi pencere"
	}
	return strings.Join([]string{
		"ATA geçmişi",
		strconvItoa(profile.SignaturesFetched) + " imza",
		strconvItoa(profile.TransactionsParsed) + " işlem", window,
	}, " · ")
}

func strconvItoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	buf := [32]byte{}
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
