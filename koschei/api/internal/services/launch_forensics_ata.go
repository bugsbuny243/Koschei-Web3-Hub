package services

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

const launchATAConcurrency = 4
const launchTransactionBatchSize = 40

func analyzeLaunchForensicsATA(ctx context.Context, rpcURL, mint string, candidates []launchOwnerCandidate, launchTime time.Time, launchSlot int64, cfg launchForensicsConfig, budget *holderScanRPCBudget) []LaunchActorProfile {
	if len(candidates) == 0 {
		return nil
	}
	profiles := make([]LaunchActorProfile, len(candidates))
	analyzeRange := func(start, end int) {
		sem := make(chan struct{}, launchATAConcurrency)
		var wg sync.WaitGroup
		for i := start; i < end; i++ {
			if ctx.Err() != nil {
				break
			}
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					return
				}
				profiles[index] = analyzeLaunchOwnerATA(ctx, rpcURL, mint, candidates[index], launchTime, launchSlot, cfg, budget)
			}(i)
		}
		wg.Wait()
	}
	priorityEnd := len(candidates)
	if priorityEnd > 10 {
		priorityEnd = 10
	}
	analyzeRange(0, priorityEnd)
	if priorityEnd < len(candidates) && ctx.Err() == nil {
		analyzeRange(priorityEnd, len(candidates))
	}
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

func analyzeLaunchOwnerATA(ctx context.Context, rpcURL, mint string, candidate launchOwnerCandidate, launchTime time.Time, launchSlot int64, cfg launchForensicsConfig, budget *holderScanRPCBudget) LaunchActorProfile {
	base := LaunchActorProfile{
		OwnerWallet: candidate.OwnerWallet, TokenAccounts: append([]string{}, candidate.TokenAccounts...),
		Label: "HISTORY_NOT_CAPTURED", FundingStatus: "not_checked", Source: "ata_history", Evidence: []string{},
	}
	if len(candidate.TokenAccounts) == 0 {
		base.Evidence = append(base.Evidence, "Owner için çözümlenmiş token account/ATA bulunamadı.")
		return base
	}
	bySignature := map[string]SolanaSignatureInfo{}
	allExhausted := true
	for _, tokenAccount := range candidate.TokenAccounts {
		before := ""
		exhausted := false
		for page := 0; page < cfg.ATAMaxPages; page++ {
			if ctx.Err() != nil || !budget.Reserve(1) {
				base.Evidence = append(base.Evidence, "ATA imza taraması RPC bütçesi veya istek süresi nedeniyle kısmi kaldı.")
				allExhausted = false
				break
			}
			signatures, err := SolanaGetSignaturesForAddressBefore(ctx, rpcURL, tokenAccount, cfg.ATASignatureLimit, before)
			if err != nil {
				base.Evidence = append(base.Evidence, "ATA imza geçmişi alınamadı: "+compactClusterError(err))
				allExhausted = false
				break
			}
			for _, signature := range signatures {
				if signature.Signature != "" {
					bySignature[signature.Signature] = signature
				}
			}
			if len(signatures) < cfg.ATASignatureLimit {
				exhausted = true
				break
			}
			if len(signatures) == 0 || strings.TrimSpace(signatures[len(signatures)-1].Signature) == "" {
				exhausted = true
				break
			}
			before = signatures[len(signatures)-1].Signature
		}
		if !exhausted {
			allExhausted = false
		}
	}
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
		transactions, err := SolanaGetTransactionsJSONParsedBatch(ctx, rpcURL, keys)
		if err != nil {
			base.Evidence = append(base.Evidence, "ATA parsed transaction batch alınamadı: "+compactClusterError(err))
			continue
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
