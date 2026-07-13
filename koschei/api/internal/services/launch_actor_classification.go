package services

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	LaunchLabelSniperBot   = "SNIPER_BOT"
	LaunchLabelRhythmBot   = "RHYTHM_BOT"
	LaunchLabelFlipper     = "FLIPPER"
	LaunchLabelAccumulator = "ACCUMULATOR"
	LaunchLabelOrganic     = "ORGANIC"
)

type LaunchTrade struct {
	Mint        string    `json:"mint,omitempty"`
	Trader      string    `json:"trader"`
	Side        string    `json:"side"`
	SOLAmount   float64   `json:"sol_amount,omitempty"`
	TokenAmount float64   `json:"token_amount,omitempty"`
	Slot        int64     `json:"slot,omitempty"`
	BlockTime   time.Time `json:"block_time,omitempty"`
	Signature   string    `json:"signature,omitempty"`
	Source      string    `json:"source,omitempty"`
	Program     string    `json:"counterparty_program,omitempty"`
}

type LaunchActorProfile struct {
	OwnerWallet             string    `json:"owner_wallet"`
	TokenAccounts           []string  `json:"token_accounts,omitempty"`
	EntryRank               int       `json:"entry_rank,omitempty"`
	FirstBuySlot            int64     `json:"first_buy_slot,omitempty"`
	FirstBuyTime            time.Time `json:"first_buy_time,omitempty"`
	MinutesAfterLaunch      float64   `json:"minutes_after_launch,omitempty"`
	SlotOffsetFromLaunch    int64     `json:"slot_offset_from_launch,omitempty"`
	LaunchTimeKnown         bool      `json:"launch_time_known"`
	LaunchSlotKnown         bool      `json:"launch_slot_known"`
	BuyCount                int       `json:"buy_count"`
	SellCount               int       `json:"sell_count"`
	TradeCount              int       `json:"trade_count"`
	AverageIntervalSeconds  float64   `json:"avg_interval_seconds,omitempty"`
	IntervalStddevSeconds   float64   `json:"interval_stddev_seconds,omitempty"`
	BoughtTokenAmount       float64   `json:"bought_token_amount,omitempty"`
	SoldTokenAmount         float64   `json:"sold_token_amount,omitempty"`
	NetPosition             float64   `json:"net_position"`
	SoldWithin15mPercentage float64   `json:"sold_within_15m_percentage,omitempty"`
	Label                   string    `json:"label"`
	Sniper                  bool      `json:"sniper"`
	RhythmBot               bool      `json:"rhythm_bot"`
	Flipper                 bool      `json:"flipper"`
	Accumulator             bool      `json:"accumulator"`
	CreatorLinked           bool      `json:"creator_linked"`
	FundingStatus           string    `json:"funding_status,omitempty"`
	FundingHops             int       `json:"funding_hops,omitempty"`
	FundingPath             []string  `json:"funding_path,omitempty"`
	Evidence                []string  `json:"evidence"`
	Source                  string    `json:"source,omitempty"`
	WindowExhausted         bool      `json:"window_exhausted,omitempty"`
	SignaturesFetched       int       `json:"signatures_fetched,omitempty"`
	TransactionsParsed      int       `json:"transactions_parsed,omitempty"`
}

type LaunchTimelineEntry struct {
	EntryRank          int      `json:"entry_rank"`
	Wallet             string   `json:"wallet"`
	FirstBuySlot       int64    `json:"first_buy_slot,omitempty"`
	FirstBuyTime       string   `json:"first_buy_time,omitempty"`
	SlotOffset         int64    `json:"slot_offset,omitempty"`
	MinutesAfterLaunch float64  `json:"minutes_after_launch,omitempty"`
	LaunchTimeKnown    bool     `json:"launch_time_known"`
	LaunchSlotKnown    bool     `json:"launch_slot_known"`
	Label              string   `json:"label"`
	CreatorLinked      bool     `json:"creator_linked"`
	FundingHops        int      `json:"funding_hops,omitempty"`
	Evidence           []string `json:"evidence"`
}

func classifyLaunchActors(trades []LaunchTrade, launchSlot int64, launchTime time.Time, sniperSlotWindow int) []LaunchActorProfile {
	grouped := map[string][]LaunchTrade{}
	for _, trade := range trades {
		wallet := strings.TrimSpace(trade.Trader)
		side := strings.ToLower(strings.TrimSpace(trade.Side))
		if wallet == "" || (side != "buy" && side != "sell") {
			continue
		}
		trade.Trader = wallet
		trade.Side = side
		grouped[wallet] = append(grouped[wallet], trade)
	}
	profiles := make([]LaunchActorProfile, 0, len(grouped))
	for wallet, walletTrades := range grouped {
		profiles = append(profiles, classifyLaunchActor(wallet, walletTrades, launchSlot, launchTime, sniperSlotWindow))
	}
	sort.SliceStable(profiles, func(i, j int) bool {
		left, right := profiles[i], profiles[j]
		if !left.FirstBuyTime.IsZero() && !right.FirstBuyTime.IsZero() && !left.FirstBuyTime.Equal(right.FirstBuyTime) {
			return left.FirstBuyTime.Before(right.FirstBuyTime)
		}
		if left.FirstBuySlot > 0 && right.FirstBuySlot > 0 && left.FirstBuySlot != right.FirstBuySlot {
			return left.FirstBuySlot < right.FirstBuySlot
		}
		if left.FirstBuyTime.IsZero() != right.FirstBuyTime.IsZero() {
			return !left.FirstBuyTime.IsZero()
		}
		return left.OwnerWallet < right.OwnerWallet
	})
	rank := 0
	for i := range profiles {
		if profiles[i].BuyCount > 0 {
			rank++
			profiles[i].EntryRank = rank
		}
	}
	return profiles
}

func classifyLaunchActor(wallet string, trades []LaunchTrade, launchSlot int64, launchTime time.Time, sniperSlotWindow int) LaunchActorProfile {
	sort.SliceStable(trades, func(i, j int) bool {
		if !trades[i].BlockTime.IsZero() && !trades[j].BlockTime.IsZero() && !trades[i].BlockTime.Equal(trades[j].BlockTime) {
			return trades[i].BlockTime.Before(trades[j].BlockTime)
		}
		if trades[i].Slot != trades[j].Slot {
			return trades[i].Slot < trades[j].Slot
		}
		return trades[i].Signature < trades[j].Signature
	})
	profile := LaunchActorProfile{OwnerWallet: wallet, Label: LaunchLabelOrganic, Evidence: []string{}, FundingStatus: "not_checked"}
	intervals := []float64{}
	var firstTradeTime, lastTradeTime, previousTime, firstSellTime time.Time
	for _, trade := range trades {
		if profile.Source == "" && trade.Source != "" {
			profile.Source = trade.Source
		}
		if !trade.BlockTime.IsZero() {
			if firstTradeTime.IsZero() || trade.BlockTime.Before(firstTradeTime) {
				firstTradeTime = trade.BlockTime
			}
			if lastTradeTime.IsZero() || trade.BlockTime.After(lastTradeTime) {
				lastTradeTime = trade.BlockTime
			}
			if !previousTime.IsZero() && trade.BlockTime.After(previousTime) {
				intervals = append(intervals, trade.BlockTime.Sub(previousTime).Seconds())
			}
			previousTime = trade.BlockTime
		}
		switch trade.Side {
		case "buy":
			profile.BuyCount++
			profile.BoughtTokenAmount += math.Abs(trade.TokenAmount)
			if profile.FirstBuySlot == 0 || (trade.Slot > 0 && trade.Slot < profile.FirstBuySlot) {
				profile.FirstBuySlot = trade.Slot
			}
			if profile.FirstBuyTime.IsZero() || (!trade.BlockTime.IsZero() && trade.BlockTime.Before(profile.FirstBuyTime)) {
				profile.FirstBuyTime = trade.BlockTime
			}
		case "sell":
			profile.SellCount++
			profile.SoldTokenAmount += math.Abs(trade.TokenAmount)
			if firstSellTime.IsZero() || (!trade.BlockTime.IsZero() && trade.BlockTime.Before(firstSellTime)) {
				firstSellTime = trade.BlockTime
			}
		}
	}
	profile.TradeCount = profile.BuyCount + profile.SellCount
	profile.NetPosition = roundLaunchNumber(profile.BoughtTokenAmount-profile.SoldTokenAmount, 9)
	profile.BoughtTokenAmount = roundLaunchNumber(profile.BoughtTokenAmount, 9)
	profile.SoldTokenAmount = roundLaunchNumber(profile.SoldTokenAmount, 9)
	profile.AverageIntervalSeconds, profile.IntervalStddevSeconds = launchIntervalStats(intervals)

	profile.LaunchSlotKnown = launchSlot > 0 && profile.FirstBuySlot > 0
	if profile.LaunchSlotKnown {
		profile.SlotOffsetFromLaunch = profile.FirstBuySlot - launchSlot
	}
	profile.LaunchTimeKnown = !launchTime.IsZero() && !profile.FirstBuyTime.IsZero()
	var launchDelta time.Duration
	if profile.LaunchTimeKnown {
		launchDelta = profile.FirstBuyTime.Sub(launchTime)
		profile.MinutesAfterLaunch = roundLaunchNumber(launchDelta.Seconds()/60, 2)
	}
	if sniperSlotWindow <= 0 {
		sniperSlotWindow = 3
	}
	slotSniper := profile.LaunchSlotKnown && profile.SlotOffsetFromLaunch >= 0 && profile.SlotOffsetFromLaunch <= int64(sniperSlotWindow)
	timeSniper := profile.LaunchTimeKnown && launchDelta >= 0 && launchDelta <= time.Minute
	profile.Sniper = profile.BuyCount > 0 && (slotSniper || timeSniper)
	profile.RhythmBot = profile.TradeCount >= 5 && profile.AverageIntervalSeconds > 0 && profile.IntervalStddevSeconds/profile.AverageIntervalSeconds < 0.25
	if profile.BoughtTokenAmount > 0 && !profile.FirstBuyTime.IsZero() {
		sold15 := 0.0
		deadline := profile.FirstBuyTime.Add(15 * time.Minute)
		for _, trade := range trades {
			if trade.Side == "sell" && !trade.BlockTime.IsZero() && !trade.BlockTime.After(deadline) {
				sold15 += math.Abs(trade.TokenAmount)
			}
		}
		profile.SoldWithin15mPercentage = roundLaunchNumber(math.Min(100, sold15/profile.BoughtTokenAmount*100), 2)
		profile.Flipper = profile.SoldWithin15mPercentage >= 80
	}
	span := time.Duration(0)
	if !firstTradeTime.IsZero() && !lastTradeTime.IsZero() {
		span = lastTradeTime.Sub(firstTradeTime)
	}
	profile.Accumulator = profile.BuyCount >= 3 && profile.SellCount == 0 && span >= 30*time.Minute

	switch {
	case profile.RhythmBot:
		profile.Label = LaunchLabelRhythmBot
	case profile.Flipper:
		profile.Label = LaunchLabelFlipper
	case profile.Sniper:
		profile.Label = LaunchLabelSniperBot
	case profile.Accumulator:
		profile.Label = LaunchLabelAccumulator
	default:
		profile.Label = LaunchLabelOrganic
	}
	profile.Evidence = launchActorEvidence(profile, firstSellTime)
	return profile
}

func launchActorEvidence(profile LaunchActorProfile, firstSellTime time.Time) []string {
	evidence := []string{}
	if profile.Sniper {
		if profile.LaunchSlotKnown && profile.SlotOffsetFromLaunch >= 0 {
			evidence = append(evidence, fmt.Sprintf("Lansmandan %d slot sonra ilk alım yapıldı.", profile.SlotOffsetFromLaunch))
		} else if profile.LaunchTimeKnown && profile.MinutesAfterLaunch >= 0 {
			evidence = append(evidence, fmt.Sprintf("Lansmandan yaklaşık %.0f saniye sonra ilk alım yapıldı.", profile.MinutesAfterLaunch*60))
		}
	}
	if profile.RhythmBot {
		evidence = append(evidence, fmt.Sprintf("%d işlem %.0f sn ± %.0f sn aralıkla gerçekleşti; ritim makine düzenine yakın.", profile.TradeCount, profile.AverageIntervalSeconds, profile.IntervalStddevSeconds))
	}
	if profile.Flipper {
		minutes := 15.0
		if !profile.FirstBuyTime.IsZero() && !firstSellTime.IsZero() {
			minutes = firstSellTime.Sub(profile.FirstBuyTime).Minutes()
			if minutes < 0 {
				minutes = 0
			}
		}
		evidence = append(evidence, fmt.Sprintf("Alınan miktarın %.0f%%'si ilk %.1f dakika içinde satıldı.", profile.SoldWithin15mPercentage, minutes))
	}
	if profile.Accumulator {
		evidence = append(evidence, fmt.Sprintf("%d alım görüldü; gözlenen pencerede satış yok ve birikim en az 30 dakikaya yayıldı.", profile.BuyCount))
	}
	if profile.Label == LaunchLabelOrganic {
		evidence = append(evidence, fmt.Sprintf("%d alım / %d satış içinde sniper, düzenli bot ritmi, hızlı boşaltma veya tek yönlü birikim kuralı doğrulanmadı.", profile.BuyCount, profile.SellCount))
	}
	if len(evidence) == 0 {
		evidence = append(evidence, "Hedef token için sınıflandırılabilir işlem hareketi gözlenmedi.")
	}
	return evidence
}

func launchIntervalStats(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	avg := total / float64(len(values))
	variance := 0.0
	for _, value := range values {
		delta := value - avg
		variance += delta * delta
	}
	variance /= float64(len(values))
	return roundLaunchNumber(avg, 3), roundLaunchNumber(math.Sqrt(variance), 3)
}

func roundLaunchNumber(value float64, digits int) float64 {
	factor := math.Pow10(digits)
	return math.Round(value*factor) / factor
}
