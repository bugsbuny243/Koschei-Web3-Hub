package services

import (
	"strings"
	"time"
)

// TargetIdentity is the canonical, non-verdict identity envelope attached to a
// token investigation. Market metadata is observed context and never changes a
// deterministic actor or unified Radar grade.
type TargetIdentity struct {
	Chain          string               `json:"chain"`
	Mint           string               `json:"mint"`
	Name           string               `json:"name,omitempty"`
	Symbol         string               `json:"symbol,omitempty"`
	LogoURL        string               `json:"logo_url,omitempty"`
	MetadataStatus string               `json:"metadata_status"`
	MetadataSource string               `json:"metadata_source,omitempty"`
	Pair           TargetIdentityPair   `json:"pair"`
	Market         TargetIdentityMarket `json:"market"`
	ScannedAt      time.Time            `json:"scanned_at"`
}

type TargetIdentityPair struct {
	Address     string `json:"address,omitempty"`
	DEX         string `json:"dex,omitempty"`
	BaseSymbol  string `json:"base_symbol,omitempty"`
	QuoteSymbol string `json:"quote_symbol,omitempty"`
	Canonical   bool   `json:"canonical"`
}

type TargetIdentityMarket struct {
	PriceUSD     float64 `json:"price_usd"`
	MarketCapUSD float64 `json:"market_cap_usd"`
	Volume24hUSD float64 `json:"volume_24h_usd"`
	LiquidityUSD float64 `json:"liquidity_usd"`
}

func BuildTargetIdentity(mint string, market TokenMarketSnapshot, scannedAt time.Time) TargetIdentity {
	identity := TargetIdentity{
		Chain: "solana", Mint: strings.TrimSpace(mint), MetadataStatus: "unknown",
		Pair: TargetIdentityPair{}, Market: TargetIdentityMarket{}, ScannedAt: scannedAt.UTC(),
	}
	if market.Available || strings.TrimSpace(market.Name) != "" || strings.TrimSpace(market.Symbol) != "" {
		identity.Name, identity.Symbol, identity.LogoURL = strings.TrimSpace(market.Name), strings.TrimSpace(market.Symbol), strings.TrimSpace(market.LogoURL)
		identity.MetadataStatus, identity.MetadataSource = "observed", "DexScreener"
	}
	identity.Pair = TargetIdentityPair{
		Address: strings.TrimSpace(market.BestPairAddress), DEX: strings.TrimSpace(market.BestPairDEX),
		BaseSymbol: strings.TrimSpace(market.BestPairBaseSymbol), QuoteSymbol: strings.TrimSpace(market.BestPairQuoteSymbol),
		Canonical: strings.TrimSpace(market.BestPairAddress) != "",
	}
	identity.Market = TargetIdentityMarket{
		PriceUSD: market.PriceUSD, MarketCapUSD: market.MarketCapUSD,
		Volume24hUSD: market.BestPairVolume24hUSD, LiquidityUSD: market.BestPairLiquidityUSD,
	}
	return identity
}
