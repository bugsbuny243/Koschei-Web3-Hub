package handlers

import (
	"math"
	"strings"
	"time"
)

const (
	boundedMarketBehaviorVersion          = "koschei-market-behavior-indicators-v1"
	boundedRapidRoundTripWindow           = 5 * time.Minute
	boundedCoordinationWindow             = time.Minute
	boundedMinTemporalRows                = 6
	boundedMinDistinctSwapTraders         = 2
	boundedMinRapidRoundTripWallets       = 2
	boundedMinCoordinatedBurstTradeCount  = 4
	boundedMinCoordinatedBurstWalletCount = 3
)

type boundedMarketBehaviorTrade struct {
	direction string
	at        time.Time
}

type boundedMarketBehaviorBucket struct {
	tradeCount int
	wallets    map[string]bool
	buy        bool
	sell       bool
}

// boundedMarketBehaviorIndicators derives evidence-scoped market-behavior
// indicators from the transaction rows already collected by Unified Radar.
// It deliberately does not claim wash trading, market manipulation, common
// control, identity or intent. Those claims require broader market coverage and
// independent corroboration beyond this bounded recent-trade window.
func boundedMarketBehaviorIndicators(values []unifiedTransactionEvidence) map[string]any {
	walletTrades := map[string][]boundedMarketBehaviorTrade{}
	swapWallets := map[string]bool{}
	buckets := map[int64]*boundedMarketBehaviorBucket{}
	temporalRows := 0
	swapRows := 0

	for _, row := range values {
		direction := strings.ToLower(strings.TrimSpace(row.Direction))
		if direction != "buy" && direction != "sell" {
			continue
		}
		wallet := strings.TrimSpace(row.Trader)
		if wallet == "" {
			continue
		}
		swapRows++
		swapWallets[wallet] = true
		if row.BlockTime == nil || row.BlockTime.IsZero() {
			continue
		}
		temporalRows++
		at := row.BlockTime.UTC()
		walletTrades[wallet] = append(walletTrades[wallet], boundedMarketBehaviorTrade{direction: direction, at: at})
		bucketID := at.Unix() / int64(boundedCoordinationWindow/time.Second)
		bucket := buckets[bucketID]
		if bucket == nil {
			bucket = &boundedMarketBehaviorBucket{wallets: map[string]bool{}}
			buckets[bucketID] = bucket
		}
		bucket.tradeCount++
		bucket.wallets[wallet] = true
		if direction == "buy" {
			bucket.buy = true
		} else {
			bucket.sell = true
		}
	}

	rapidRoundTripWallets := 0
	for _, trades := range walletTrades {
		rapid := false
		for i := 0; i < len(trades) && !rapid; i++ {
			for j := i + 1; j < len(trades); j++ {
				if trades[i].direction == trades[j].direction {
					continue
				}
				delta := trades[i].at.Sub(trades[j].at)
				if delta < 0 {
					delta = -delta
				}
				if delta <= boundedRapidRoundTripWindow {
					rapid = true
					break
				}
			}
		}
		if rapid {
			rapidRoundTripWallets++
		}
	}

	coordinatedBursts := 0
	for _, bucket := range buckets {
		if bucket.tradeCount >= boundedMinCoordinatedBurstTradeCount &&
			len(bucket.wallets) >= boundedMinCoordinatedBurstWalletCount &&
			bucket.buy && bucket.sell {
			coordinatedBursts++
		}
	}

	status := "insufficient_evidence"
	indicator := "insufficient_evidence"
	if temporalRows >= boundedMinTemporalRows && len(swapWallets) >= boundedMinDistinctSwapTraders {
		status = "no_bounded_pattern_observed"
		indicator = "not_observed_in_bounded_window"
		if rapidRoundTripWallets >= boundedMinRapidRoundTripWallets || coordinatedBursts > 0 {
			status = "bounded_pattern_observed"
			indicator = "observed_requires_corroboration"
		}
	}

	rapidRatio := 0.0
	if len(swapWallets) > 0 {
		rapidRatio = math.Round((float64(rapidRoundTripWallets)/float64(len(swapWallets)))*10000) / 10000
	}

	return map[string]any{
		"market_behavior_indicator_version": boundedMarketBehaviorVersion,
		"market_behavior_status":            status,
		"wash_like_indicator":               indicator,
		"swap_trade_count":                  int64(swapRows),
		"temporal_trade_count":              int64(temporalRows),
		"distinct_swap_trader_count":        int64(len(swapWallets)),
		"rapid_round_trip_wallet_count":     int64(rapidRoundTripWallets),
		"rapid_round_trip_wallet_ratio":     rapidRatio,
		"rapid_round_trip_window_seconds":   int64(boundedRapidRoundTripWindow / time.Second),
		"coordinated_opposite_side_bursts":  int64(coordinatedBursts),
		"coordination_window_seconds":       int64(boundedCoordinationWindow / time.Second),
		"wash_claim":                        false,
		"market_manipulation_claim":         false,
		"same_operator_claim":               false,
		"evidence_scope":                    "bounded_recent_transaction_rows",
		"market_behavior_interpretation":    "Observed timing patterns are screening indicators only. They do not prove wash trading, manipulation, shared control, identity or intent without broader market evidence and corroboration.",
	}
}
