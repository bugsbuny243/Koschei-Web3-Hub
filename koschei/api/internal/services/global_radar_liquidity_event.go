package services

import (
	"errors"
	"strings"
)

// BuildSolanaLiquidityRadarEvent converts two provider-backed market snapshots
// into one descriptive liquidity-change observation. It does not label a rug,
// drain, manipulation event, or safety outcome.
func BuildSolanaLiquidityRadarEvent(previous, current TokenMarketSnapshot) (GlobalRadarObservation, error) {
	if !previous.Available || !current.Available ||
		previous.Status != "verified_market_snapshot" ||
		current.Status != "verified_market_snapshot" {
		return GlobalRadarObservation{}, errors.New("two available market snapshots are required")
	}
	if strings.TrimSpace(previous.Mint) == "" ||
		strings.TrimSpace(previous.Mint) != strings.TrimSpace(current.Mint) {
		return GlobalRadarObservation{}, errors.New("liquidity snapshots must reference the same mint")
	}
	if previous.ObservedAt.IsZero() || current.ObservedAt.IsZero() ||
		!current.ObservedAt.After(previous.ObservedAt) {
		return GlobalRadarObservation{}, errors.New("liquidity snapshots must have increasing observation times")
	}
	if strings.TrimSpace(previous.Provider) == "" || strings.TrimSpace(current.Provider) == "" {
		return GlobalRadarObservation{}, errors.New("market snapshot provider is required")
	}

	subject := ClassifyIntelligenceSubject(current.Mint, "solana-mainnet")
	if subject.ChainFamily != IntelligenceChainFamilySolana {
		return GlobalRadarObservation{}, errors.New("valid Solana mint syntax is required")
	}
	subject.Kind = IntelligenceSubjectToken
	subject.ClassificationBasis = "solana_token_market_snapshot"

	liquidityDelta := current.LiquidityUSD - previous.LiquidityUSD
	volumeDelta := current.Volume24hUSD - previous.Volume24hUSD
	var liquidityDeltaPct any
	if previous.LiquidityUSD > 0 {
		liquidityDeltaPct = (liquidityDelta / previous.LiquidityUSD) * 100
	}

	attributes := map[string]any{
		"previous_observed_at":             previous.ObservedAt.UTC(),
		"current_observed_at":              current.ObservedAt.UTC(),
		"previous_liquidity_usd":           previous.LiquidityUSD,
		"current_liquidity_usd":            current.LiquidityUSD,
		"liquidity_delta_usd":              liquidityDelta,
		"liquidity_delta_pct":              liquidityDeltaPct,
		"previous_volume_24h_usd":          previous.Volume24hUSD,
		"current_volume_24h_usd":           current.Volume24hUSD,
		"volume_24h_delta_usd":             volumeDelta,
		"previous_price_usd":               previous.PriceUSD,
		"current_price_usd":                current.PriceUSD,
		"previous_pair_address":            previous.BestPairAddress,
		"current_pair_address":             current.BestPairAddress,
		"previous_dex":                     previous.BestPairDEX,
		"current_dex":                      current.BestPairDEX,
		"previous_provider":                previous.Provider,
		"current_provider":                 current.Provider,
		"valuation_scope":                  current.ValuationScope,
		"limitations":                      append([]string(nil), current.Limitations...),
		"market_data_can_issue_verdict":    false,
		"liquidity_change_classification":  "descriptive_only",
		"rug_or_drain_claim":               false,
	}

	identity := strings.Join([]string{
		"global-radar-liquidity-event",
		subject.ID,
		previous.ObservedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		current.ObservedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		previous.Provider,
		current.Provider,
	}, ":")

	evidence := IntelligenceEvidence{
		ID:          intelligenceStableID(identity),
		SubjectID:   subject.ID,
		ChainFamily: subject.ChainFamily,
		Chain:       subject.Chain,
		Network:     subject.Network,
		Source:      "token_market_snapshot_delta",
		Status:      IntelligenceEvidenceObserved,
		ObservedAt:  current.ObservedAt.UTC(),
		Address:     current.Mint,
		Contract:    current.BestPairAddress,
		Method:      "market_liquidity_snapshot_delta",
		StateChange: "liquidity_context_changed",
		Provenance:  "provider_market_context_not_onchain_verdict",
		Confidence:  0.8,
		Attributes:  attributes,
	}
	return BuildGlobalRadarObservation(GlobalRadarObservationLiquidity, subject, evidence)
}
