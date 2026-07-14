package services

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const UnifiedRadarRulesetVersion = "koschei-unified-radar-rules-v1.1.0"

const (
	UnifiedRuleVolumeLiquidityGap          = "ARD-C006"
	UnifiedRuleHolderLiquidityDepth        = "ARD-C007"
	UnifiedRuleCreatorSellAcceleration     = "ARD-C008"
	UnifiedRuleDominantHolderFirstExit     = "ARD-C009"
	unifiedRecentCreatorSellWindow         = 2 * time.Hour
	unifiedPriorCreatorSellWindow          = 22 * time.Hour
	unifiedMinimumVolumeUSD                = 50000.0
	unifiedVolumeLiquidityRatioThreshold   = 8.0
	unifiedHolderLiquidityRatioThreshold   = 1.0
	unifiedMinimumHolderPositionUSD        = 10000.0
	unifiedMinimumDominantHolderPercentage = 20.0
)

type UnifiedRadarSignal struct {
	SignalID          string         `json:"signal_id"`
	RuleID            string         `json:"rule_id"`
	Title             string         `json:"title"`
	Available         bool           `json:"available"`
	Triggered         bool           `json:"triggered"`
	EvidenceStatus    string         `json:"evidence_status"`
	GradeEffect       string         `json:"grade_effect"`
	Summary           string         `json:"summary"`
	EvidenceKeys      []string       `json:"evidence_keys"`
	Signatures        []string       `json:"signatures"`
	Facts             map[string]any `json:"facts"`
	Limitations       []string       `json:"limitations"`
	ObservedAt        time.Time      `json:"observed_at"`
	ObservationScope  string         `json:"observation_scope"`
}

type UnifiedRadarBehaviorInput struct {
	Mint          string
	Network       string
	CreatorWallet string
	Market        TokenMarketSnapshot
	Holder        HolderIntelligence
	Cluster       HolderClusterAnalysis
	ObservedAt    time.Time
}

type UnifiedRadarBehaviorAnalysis struct {
	RulesetVersion string               `json:"ruleset_version"`
	Mint           string               `json:"mint"`
	CreatorWallet  string               `json:"creator_wallet,omitempty"`
	Signals        []UnifiedRadarSignal `json:"signals"`
	TriggeredCount int                  `json:"triggered_count"`
	GeneratedAt    time.Time            `json:"generated_at"`
	Policy         map[string]any       `json:"policy"`
}

type CreatorSellWindowStats struct {
	Available          bool      `json:"available"`
	RecentSellCount    int       `json:"recent_sell_count"`
	PriorSellCount     int       `json:"prior_sell_count"`
	RecentSOLSold      float64   `json:"recent_sol_sold"`
	PriorSOLSold       float64   `json:"prior_sol_sold"`
	RecentTokenSold    float64   `json:"recent_token_sold"`
	PriorTokenSold     float64   `json:"prior_token_sold"`
	RecentCountPerHour float64   `json:"recent_count_per_hour"`
	PriorCountPerHour  float64   `json:"prior_count_per_hour"`
	RecentSOLPerHour   float64   `json:"recent_sol_per_hour"`
	PriorSOLPerHour    float64   `json:"prior_sol_per_hour"`
	Signatures         []string  `json:"signatures"`
	WindowEnd          time.Time `json:"window_end"`
}

func AnalyzeUnifiedRadarBehavior(ctx context.Context, db *sql.DB, input UnifiedRadarBehaviorInput) UnifiedRadarBehaviorAnalysis {
	observedAt := input.ObservedAt.UTC()
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	input.Mint = strings.TrimSpace(input.Mint)
	input.Network = normalizeRadarNetwork(input.Network)
	input.CreatorWallet = strings.TrimSpace(input.CreatorWallet)
	out := UnifiedRadarBehaviorAnalysis{
		RulesetVersion: UnifiedRadarRulesetVersion,
		Mint: input.Mint,
		CreatorWallet: input.CreatorWallet,
		Signals: []UnifiedRadarSignal{},
		GeneratedAt: observedAt,
		Policy: map[string]any{
			"numeric_score_disabled": true,
			"automatic_scanning": false,
			"manual_scan_only": true,
			"inferred_is_watch_only": true,
			"unverified_is_excluded": true,
		},
	}

	out.Signals = append(out.Signals, evaluateUnifiedVolumeLiquidityGap(input.Market, input.Mint, observedAt))
	out.Signals = append(out.Signals, evaluateUnifiedHolderLiquidityDepth(input.Holder, input.Market, input.Mint, observedAt))

	creatorStats, creatorErr := loadCreatorSellWindowStats(ctx, db, input.Mint, input.CreatorWallet, observedAt)
	creatorSignal := EvaluateCreatorSellAcceleration(creatorStats, input.Mint, input.CreatorWallet, observedAt)
	if creatorErr != nil {
		creatorSignal.Limitations = append(creatorSignal.Limitations, "Creator trade ledger could not be queried: "+compactUnifiedRadarError(creatorErr))
	}
	out.Signals = append(out.Signals, creatorSignal)
	out.Signals = append(out.Signals, evaluateUnifiedDominantHolderFirstExit(input.Holder, input.Cluster, input.Mint, observedAt))

	for _, signal := range out.Signals {
		if signal.Triggered {
			out.TriggeredCount++
		}
	}
	return out
}

func evaluateUnifiedVolumeLiquidityGap(market TokenMarketSnapshot, mint string, observedAt time.Time) UnifiedRadarSignal {
	signal := UnifiedRadarSignal{
		SignalID: "volume_liquidity_gap", RuleID: UnifiedRuleVolumeLiquidityGap,
		Title: "Hacim / likidite uçurumu", EvidenceStatus: "unverified",
		GradeEffect: "none", EvidenceKeys: []string{}, Signatures: []string{}, Facts: map[string]any{},
		Limitations: []string{}, ObservedAt: observedAt, ObservationScope: "current_market_snapshot",
	}
	if !market.Available || market.Volume24hUSD <= 0 {
		signal.Summary = "24 saatlik hacim veya piyasa likiditesi doğrulanamadı."
		signal.Limitations = append(signal.Limitations, market.Limitations...)
		return signal
	}
	signal.Available = true
	ratio := 0.0
	zeroLiquidity := market.LiquidityUSD <= 0
	if !zeroLiquidity {
		ratio = market.Volume24hUSD / market.LiquidityUSD
	}
	signal.Facts = map[string]any{
		"mint": mint,
		"volume_24h_usd": roundUnifiedRadar(market.Volume24hUSD, 2),
		"liquidity_usd": roundUnifiedRadar(market.LiquidityUSD, 2),
		"volume_to_liquidity_ratio": roundUnifiedRadar(ratio, 4),
		"minimum_volume_usd": unifiedMinimumVolumeUSD,
		"ratio_threshold": unifiedVolumeLiquidityRatioThreshold,
		"provider": market.Provider,
	}
	signal.EvidenceKeys = []string{fmt.Sprintf("market:%s:%s", mint, observedAt.Format("20060102T15"))}
	triggered := market.Volume24hUSD >= unifiedMinimumVolumeUSD && (zeroLiquidity || ratio >= unifiedVolumeLiquidityRatioThreshold)
	if !triggered {
		signal.EvidenceStatus = "observed"
		signal.Summary = fmt.Sprintf("24 saatlik hacim/likidite oranı %.4fx; deterministik eşik %.2fx ve minimum hacim $%.0f.", ratio, unifiedVolumeLiquidityRatioThreshold, unifiedMinimumVolumeUSD)
		return signal
	}
	signal.Triggered = true
	signal.EvidenceStatus = "observed"
	signal.GradeEffect = "compounding_input"
	if zeroLiquidity {
		signal.Summary = fmt.Sprintf("24 saatlik hacim $%.2f iken sağlayıcı doğrulanmış kullanılabilir likidite bildirmedi.", market.Volume24hUSD)
		signal.Limitations = append(signal.Limitations, "Zero reported liquidity can reflect missing or newly created pair data; this signal is OBSERVED, not proof of manipulation.")
	} else {
		signal.Summary = fmt.Sprintf("24 saatlik hacim likiditenin %.4f katı; oran %.2fx eşiğini geçti.", ratio, unifiedVolumeLiquidityRatioThreshold)
	}
	return signal
}

func evaluateUnifiedHolderLiquidityDepth(holder HolderIntelligence, market TokenMarketSnapshot, mint string, observedAt time.Time) UnifiedRadarSignal {
	signal := UnifiedRadarSignal{
		SignalID: "holder_position_liquidity_depth", RuleID: UnifiedRuleHolderLiquidityDepth,
		Title: "Likidite derinliği / holder pozisyonu", EvidenceStatus: "unverified",
		GradeEffect: "none", EvidenceKeys: []string{}, Signatures: []string{}, Facts: map[string]any{},
		Limitations: []string{}, ObservedAt: observedAt, ObservationScope: "owner_resolved_holder_plus_market_snapshot",
	}
	row, ok := unifiedDominantHolder(holder)
	if !ok || row.ReferenceUSDValue == nil || market.LiquidityUSD <= 0 {
		signal.Summary = "Dominant holder USD pozisyonu veya kullanılabilir likidite doğrulanamadı."
		signal.Limitations = append(signal.Limitations, "Owner-resolved risk-bearing holder, reference price and positive liquidity are required.")
		return signal
	}
	signal.Available = true
	positionUSD := *row.ReferenceUSDValue
	ratio := positionUSD / market.LiquidityUSD
	percentage := row.CirculatingPercentage
	if percentage <= 0 {
		percentage = row.RawPercentage
	}
	signal.Facts = map[string]any{
		"mint": mint,
		"holder_wallet": row.OwnerWallet,
		"holder_position_usd": roundUnifiedRadar(positionUSD, 2),
		"liquidity_usd": roundUnifiedRadar(market.LiquidityUSD, 2),
		"holder_position_to_liquidity_ratio": roundUnifiedRadar(ratio, 4),
		"holder_percentage": roundUnifiedRadar(percentage, 4),
		"ratio_threshold": unifiedHolderLiquidityRatioThreshold,
		"minimum_holder_position_usd": unifiedMinimumHolderPositionUSD,
	}
	signal.EvidenceKeys = []string{fmt.Sprintf("holder-liquidity:%s:%s:%s", mint, row.OwnerWallet, observedAt.Format("20060102T15"))}
	triggered := positionUSD >= unifiedMinimumHolderPositionUSD && percentage >= 10 && ratio >= unifiedHolderLiquidityRatioThreshold
	if !triggered {
		signal.EvidenceStatus = "observed"
		signal.Summary = fmt.Sprintf("Dominant holder referans pozisyonu likiditenin %.4f katı; kural için en az %.2fx, %%10 holder payı ve $%.0f pozisyon gerekir.", ratio, unifiedHolderLiquidityRatioThreshold, unifiedMinimumHolderPositionUSD)
		return signal
	}
	signal.Triggered = true
	signal.EvidenceStatus = "observed"
	signal.GradeEffect = "compounding_input"
	signal.Summary = fmt.Sprintf("Dominant holder'ın yaklaşık $%.2f referans pozisyonu $%.2f likiditenin %.4f katı.", positionUSD, market.LiquidityUSD, ratio)
	signal.Limitations = append(signal.Limitations, "Reference USD position is not guaranteed liquidation value and does not prove the wallet intends to sell.")
	return signal
}

func EvaluateCreatorSellAcceleration(stats CreatorSellWindowStats, mint, creator string, observedAt time.Time) UnifiedRadarSignal {
	signal := UnifiedRadarSignal{
		SignalID: "creator_sell_acceleration", RuleID: UnifiedRuleCreatorSellAcceleration,
		Title: "Creator satış hızının artması", EvidenceStatus: "unverified",
		GradeEffect: "none", EvidenceKeys: []string{}, Signatures: []string{}, Facts: map[string]any{},
		Limitations: []string{}, ObservedAt: observedAt, ObservationScope: "pump_trade_ledger_recent_2h_vs_prior_22h",
	}
	creator = strings.TrimSpace(creator)
	if creator == "" {
		signal.Summary = "Creator/deployer cüzdanı doğrulanamadığı için satış ivmesi hesaplanmadı."
		return signal
	}
	if !stats.Available {
		signal.Summary = "Creator için karşılaştırılabilir satış geçmişi bulunamadı."
		return signal
	}
	signal.Available = true
	signal.Signatures = append([]string{}, stats.Signatures...)
	signal.EvidenceKeys = []string{fmt.Sprintf("creator-sell-window:%s:%s:%s", mint, creator, observedAt.Format("20060102T15"))}
	countMultiplier := safeUnifiedRatio(stats.RecentCountPerHour, stats.PriorCountPerHour)
	solMultiplier := safeUnifiedRatio(stats.RecentSOLPerHour, stats.PriorSOLPerHour)
	signal.Facts = map[string]any{
		"mint": mint,
		"creator_wallet": creator,
		"recent_window_hours": 2,
		"prior_window_hours": 22,
		"recent_sell_count": stats.RecentSellCount,
		"prior_sell_count": stats.PriorSellCount,
		"recent_sol_sold": roundUnifiedRadar(stats.RecentSOLSold, 9),
		"prior_sol_sold": roundUnifiedRadar(stats.PriorSOLSold, 9),
		"recent_count_per_hour": roundUnifiedRadar(stats.RecentCountPerHour, 4),
		"prior_count_per_hour": roundUnifiedRadar(stats.PriorCountPerHour, 4),
		"count_rate_multiplier": roundUnifiedRadar(countMultiplier, 4),
		"sol_rate_multiplier": roundUnifiedRadar(solMultiplier, 4),
	}
	triggered := false
	if stats.PriorSellCount == 0 {
		triggered = stats.RecentSellCount >= 3 && (stats.RecentSOLSold >= 0.5 || stats.RecentTokenSold > 0)
	} else {
		triggered = stats.RecentSellCount >= 2 && countMultiplier >= 3 && (stats.RecentSOLSold >= 0.5 || solMultiplier >= 3)
	}
	if !triggered {
		signal.EvidenceStatus = "observed"
		signal.Summary = fmt.Sprintf("Creator satış hızı son 2 saatte %.4f işlem/saat, önceki 22 saatte %.4f işlem/saat; ivme kuralı tetiklenmedi.", stats.RecentCountPerHour, stats.PriorCountPerHour)
		return signal
	}
	signal.Triggered = true
	signal.EvidenceStatus = "observed"
	signal.GradeEffect = "compounding_input"
	signal.Summary = fmt.Sprintf("Creator satış hızı son 2 saatte %.4f işlem/saat; önceki 22 saatlik hızın %.4f katı.", stats.RecentCountPerHour, countMultiplier)
	signal.Limitations = append(signal.Limitations, "Pump trade ledger side classification is OBSERVED until each signature is independently parsed; acceleration is not proof of malicious intent.")
	return signal
}

func evaluateUnifiedDominantHolderFirstExit(holder HolderIntelligence, cluster HolderClusterAnalysis, mint string, observedAt time.Time) UnifiedRadarSignal {
	signal := UnifiedRadarSignal{
		SignalID: "dominant_holder_first_exit", RuleID: UnifiedRuleDominantHolderFirstExit,
		Title: "Dominant holder'ın ilk gözlenen çıkışı", EvidenceStatus: "unverified",
		GradeEffect: "none", EvidenceKeys: []string{}, Signatures: []string{}, Facts: map[string]any{},
		Limitations: []string{}, ObservedAt: observedAt, ObservationScope: "first_in_current_bounded_koschei_holder_window",
	}
	row, ok := unifiedDominantHolder(holder)
	if !ok {
		signal.Summary = "Owner-resolved dominant holder bulunamadı."
		return signal
	}
	percentage := row.CirculatingPercentage
	if percentage <= 0 {
		percentage = row.RawPercentage
	}
	if percentage < unifiedMinimumDominantHolderPercentage {
		signal.Available = true
		signal.EvidenceStatus = "observed"
		signal.Summary = fmt.Sprintf("En büyük risk-bearing holder payı %.4f%%; dominant-holder çıkış kuralı için en az %.2f%% gerekir.", percentage, unifiedMinimumDominantHolderPercentage)
		return signal
	}
	observation, found := earliestUnifiedHolderOutflow(cluster, row.OwnerWallet)
	if !found {
		signal.Available = cluster.Available
		signal.Summary = "Sınırlı parsed holder penceresinde dominant holder çıkışı gözlenmedi."
		if !cluster.Available {
			signal.Limitations = append(signal.Limitations, cluster.Limitations...)
		}
		return signal
	}
	signal.Available = true
	signal.Triggered = true
	signal.EvidenceStatus = "observed"
	signal.GradeEffect = "compounding_input"
	signal.Signatures = []string{observation.Signature}
	signal.EvidenceKeys = []string{fmt.Sprintf("dominant-first-exit:%s:%s:%s", mint, row.OwnerWallet, observation.Signature)}
	signal.Facts = map[string]any{
		"mint": mint,
		"dominant_holder_wallet": row.OwnerWallet,
		"holder_percentage": roundUnifiedRadar(percentage, 4),
		"destination": observation.Destination,
		"amount": observation.Amount,
		"slot": observation.Slot,
		"signature": observation.Signature,
		"kind": observation.Kind,
		"program_ids": append([]string{}, observation.ProgramIDs...),
		"first_scope": "current bounded Koschei holder observation window",
	}
	signal.Summary = fmt.Sprintf("%.4f%% paylı dominant holder için bounded penceredeki ilk token çıkışı slot %d ve %s imzasında gözlendi.", percentage, observation.Slot, compactUnifiedSignature(observation.Signature))
	signal.Limitations = append(signal.Limitations, "First means the earliest parsed outflow in the current bounded Koschei observation window, not a claim that no older on-chain exit exists.")
	return signal
}

func loadCreatorSellWindowStats(ctx context.Context, db *sql.DB, mint, creator string, now time.Time) (CreatorSellWindowStats, error) {
	out := CreatorSellWindowStats{Signatures: []string{}, WindowEnd: now.UTC()}
	mint = strings.TrimSpace(mint)
	creator = strings.TrimSpace(creator)
	if db == nil || mint == "" || creator == "" {
		return out, nil
	}
	row := db.QueryRowContext(ctx, `
		SELECT
			count(*) FILTER (WHERE COALESCE(block_time,created_at) >= $3-interval '2 hours'),
			count(*) FILTER (WHERE COALESCE(block_time,created_at) < $3-interval '2 hours' AND COALESCE(block_time,created_at) >= $3-interval '24 hours'),
			COALESCE(sum(sol_amount) FILTER (WHERE COALESCE(block_time,created_at) >= $3-interval '2 hours'),0)::double precision,
			COALESCE(sum(sol_amount) FILTER (WHERE COALESCE(block_time,created_at) < $3-interval '2 hours' AND COALESCE(block_time,created_at) >= $3-interval '24 hours'),0)::double precision,
			COALESCE(sum(token_amount) FILTER (WHERE COALESCE(block_time,created_at) >= $3-interval '2 hours'),0)::double precision,
			COALESCE(sum(token_amount) FILTER (WHERE COALESCE(block_time,created_at) < $3-interval '2 hours' AND COALESCE(block_time,created_at) >= $3-interval '24 hours'),0)::double precision
		FROM token_trade_events
		WHERE mint=$1 AND trader=$2 AND side='sell' AND COALESCE(block_time,created_at) >= $3-interval '24 hours'`, mint, creator, now.UTC())
	if err := row.Scan(&out.RecentSellCount, &out.PriorSellCount, &out.RecentSOLSold, &out.PriorSOLSold, &out.RecentTokenSold, &out.PriorTokenSold); err != nil {
		if isUnifiedMissingRelation(err) {
			return out, nil
		}
		return out, err
	}
	out.Available = out.RecentSellCount+out.PriorSellCount > 0
	out.RecentCountPerHour = float64(out.RecentSellCount) / unifiedRecentCreatorSellWindow.Hours()
	out.PriorCountPerHour = float64(out.PriorSellCount) / unifiedPriorCreatorSellWindow.Hours()
	out.RecentSOLPerHour = out.RecentSOLSold / unifiedRecentCreatorSellWindow.Hours()
	out.PriorSOLPerHour = out.PriorSOLSold / unifiedPriorCreatorSellWindow.Hours()
	if !out.Available {
		return out, nil
	}
	rows, err := db.QueryContext(ctx, `
		SELECT signature
		FROM token_trade_events
		WHERE mint=$1 AND trader=$2 AND side='sell'
		  AND COALESCE(block_time,created_at) >= $3-interval '2 hours'
		ORDER BY COALESCE(block_time,created_at) ASC,slot ASC NULLS LAST
		LIMIT 12`, mint, creator, now.UTC())
	if err != nil {
		if isUnifiedMissingRelation(err) {
			return out, nil
		}
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var signature string
		if err := rows.Scan(&signature); err != nil {
			return out, err
		}
		if signature = strings.TrimSpace(signature); signature != "" {
			out.Signatures = append(out.Signatures, signature)
		}
	}
	return out, rows.Err()
}

func unifiedDominantHolder(holder HolderIntelligence) (HolderIntelligenceRow, bool) {
	for _, row := range holder.Rows {
		if !row.RiskBearing || row.ExcludedFromHolderRisk || !row.OwnerResolved || strings.TrimSpace(row.OwnerWallet) == "" {
			continue
		}
		return row, true
	}
	return HolderIntelligenceRow{}, false
}

func earliestUnifiedHolderOutflow(cluster HolderClusterAnalysis, wallet string) (HolderClusterFlowObservation, bool) {
	wallet = strings.TrimSpace(wallet)
	if wallet == "" {
		return HolderClusterFlowObservation{}, false
	}
	candidates := []HolderClusterFlowObservation{}
	for _, item := range cluster.Wallets {
		if item.Wallet != wallet {
			continue
		}
		for _, observation := range item.FlowObservations {
			if observation.SourceWallet == wallet && observation.Amount > 0 && strings.TrimSpace(observation.Signature) != "" {
				candidates = append(candidates, observation)
			}
		}
	}
	if len(candidates) == 0 {
		return HolderClusterFlowObservation{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Slot == candidates[j].Slot {
			return candidates[i].Signature < candidates[j].Signature
		}
		if candidates[i].Slot == 0 {
			return false
		}
		if candidates[j].Slot == 0 {
			return true
		}
		return candidates[i].Slot < candidates[j].Slot
	})
	return candidates[0], true
}

func safeUnifiedRatio(current, prior float64) float64 {
	if current <= 0 {
		return 0
	}
	if prior <= 0 {
		return math.Inf(1)
	}
	return current / prior
}

func roundUnifiedRadar(value float64, places int) float64 {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return value
	}
	factor := math.Pow10(places)
	return math.Round(value*factor) / factor
}

func compactUnifiedRadarError(err error) string {
	if err == nil {
		return ""
	}
	value := strings.Join(strings.Fields(err.Error()), " ")
	if len(value) > 180 {
		value = value[:180]
	}
	return value
}

func compactUnifiedSignature(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 18 {
		return value
	}
	return value[:9] + "…" + value[len(value)-7:]
}

func isUnifiedMissingRelation(err error) bool {
	if err == nil {
		return false
	}
	value := strings.ToLower(err.Error())
	return strings.Contains(value, "does not exist") || strings.Contains(value, "undefined table")
}
