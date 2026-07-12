package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultTokenMarketEndpoint = "https://api.dexscreener.com/tokens/v1/solana"

// TokenMarketSnapshot is market context, not an on-chain ownership verdict.
// Price and USD values are reference estimates from the most liquid returned
// Solana pair and must never be presented as guaranteed liquidation value.
type TokenMarketSnapshot struct {
	Available             bool      `json:"available"`
	Status                string    `json:"status"`
	Provider              string    `json:"provider"`
	Mint                  string    `json:"mint"`
	Name                  string    `json:"name,omitempty"`
	Symbol                string    `json:"symbol,omitempty"`
	PriceUSD              float64   `json:"price_usd"`
	PriceChange24hPct     float64   `json:"price_change_24h_pct"`
	Volume24hUSD          float64   `json:"volume_24h_usd"`
	LiquidityUSD          float64   `json:"liquidity_usd"`
	MarketCapUSD          float64   `json:"market_cap_usd"`
	FDVUSD                float64   `json:"fdv_usd"`
	Buys24h               int       `json:"buys_24h"`
	Sells24h              int       `json:"sells_24h"`
	PairCount             int       `json:"pair_count"`
	BestPairAddress       string    `json:"best_pair_address,omitempty"`
	BestPairDEX           string    `json:"best_pair_dex,omitempty"`
	BestPairLiquidityUSD  float64   `json:"best_pair_liquidity_usd"`
	BestPairVolume24hUSD  float64   `json:"best_pair_volume_24h_usd"`
	ObservedAt            time.Time `json:"observed_at"`
	ValuationScope        string    `json:"valuation_scope"`
	Limitations           []string  `json:"limitations"`
}

type tokenMarketPair struct {
	ChainID     string `json:"chainId"`
	DexID       string `json:"dexId"`
	PairAddress string `json:"pairAddress"`
	BaseToken   struct {
		Address string `json:"address"`
		Name    string `json:"name"`
		Symbol  string `json:"symbol"`
	} `json:"baseToken"`
	QuoteToken struct {
		Address string `json:"address"`
		Name    string `json:"name"`
		Symbol  string `json:"symbol"`
	} `json:"quoteToken"`
	PriceUSD   string `json:"priceUsd"`
	Txns       map[string]struct {
		Buys  int `json:"buys"`
		Sells int `json:"sells"`
	} `json:"txns"`
	Volume      map[string]float64 `json:"volume"`
	PriceChange map[string]float64 `json:"priceChange"`
	Liquidity   *struct {
		USD float64 `json:"usd"`
	} `json:"liquidity"`
	MarketCap float64 `json:"marketCap"`
	FDV       float64 `json:"fdv"`
}

type TokenMarketClient struct {
	Endpoint string
	Client   *http.Client
}

func NewTokenMarketClient() *TokenMarketClient {
	endpoint := strings.TrimRight(strings.TrimSpace(os.Getenv("TOKEN_MARKET_DEXSCREENER_ENDPOINT")), "/")
	if endpoint == "" {
		endpoint = defaultTokenMarketEndpoint
	}
	return &TokenMarketClient{Endpoint: endpoint, Client: &http.Client{Timeout: 8 * time.Second}}
}

func FetchSolanaTokenMarketSnapshot(ctx context.Context, mint string) TokenMarketSnapshot {
	return NewTokenMarketClient().Fetch(ctx, mint)
}

func (c *TokenMarketClient) Fetch(ctx context.Context, mint string) TokenMarketSnapshot {
	mint = strings.TrimSpace(mint)
	out := TokenMarketSnapshot{
		Status: "market_unavailable", Provider: "dexscreener", Mint: mint,
		ObservedAt: time.Now().UTC(), ValuationScope: "most_liquid_solana_pair_reference_price",
		Limitations: []string{},
	}
	if mint == "" {
		out.Status = "mint_required"
		out.Limitations = append(out.Limitations, "A token mint is required for market context.")
		return out
	}
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	endpoint := strings.TrimRight(strings.TrimSpace(c.Endpoint), "/")
	if endpoint == "" {
		endpoint = defaultTokenMarketEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/"+mint, nil)
	if err != nil {
		out.Status = "request_build_failed"
		out.Limitations = append(out.Limitations, compactTokenMarketError(err))
		return out
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Koschei-ARVIS-Holder-Intelligence/1.0")
	resp, err := client.Do(req)
	if err != nil {
		out.Status = "market_request_failed"
		out.Limitations = append(out.Limitations, compactTokenMarketError(err))
		return out
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		out.Status = "market_response_unreadable"
		out.Limitations = append(out.Limitations, compactTokenMarketError(err))
		return out
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		out.Status = fmt.Sprintf("market_http_%d", resp.StatusCode)
		return out
	}
	pairs := []tokenMarketPair{}
	if err := json.Unmarshal(body, &pairs); err != nil {
		out.Status = "market_decode_failed"
		out.Limitations = append(out.Limitations, compactTokenMarketError(err))
		return out
	}

	seen := map[string]bool{}
	bestMetric := -1.0
	for _, pair := range pairs {
		if strings.TrimSpace(pair.ChainID) != "solana" {
			continue
		}
		baseMatches := strings.TrimSpace(pair.BaseToken.Address) == mint
		quoteMatches := strings.TrimSpace(pair.QuoteToken.Address) == mint
		if !baseMatches && !quoteMatches {
			continue
		}
		pairKey := strings.TrimSpace(pair.PairAddress)
		if pairKey == "" {
			pairKey = strings.TrimSpace(pair.DexID) + "|" + pair.BaseToken.Address + "|" + pair.QuoteToken.Address
		}
		if seen[pairKey] {
			continue
		}
		seen[pairKey] = true
		out.PairCount++
		volume := positiveTokenMarketNumber(pair.Volume["h24"])
		out.Volume24hUSD += volume
		if tx, ok := pair.Txns["h24"]; ok {
			if tx.Buys > 0 {
				out.Buys24h += tx.Buys
			}
			if tx.Sells > 0 {
				out.Sells24h += tx.Sells
			}
		}
		liquidity := 0.0
		if pair.Liquidity != nil {
			liquidity = positiveTokenMarketNumber(pair.Liquidity.USD)
			out.LiquidityUSD += liquidity
		}

		// DEX Screener's priceUsd describes the base token. A requested token
		// appearing only as the quote token must not inherit the base price.
		if !baseMatches {
			continue
		}
		metric := liquidity
		if metric <= 0 {
			metric = volume / 1000000
		}
		if metric < bestMetric {
			continue
		}
		price, _ := strconv.ParseFloat(strings.TrimSpace(pair.PriceUSD), 64)
		if price < 0 {
			price = 0
		}
		bestMetric = metric
		out.Name = strings.TrimSpace(pair.BaseToken.Name)
		out.Symbol = strings.TrimSpace(pair.BaseToken.Symbol)
		out.PriceUSD = price
		out.PriceChange24hPct = pair.PriceChange["h24"]
		out.MarketCapUSD = positiveTokenMarketNumber(pair.MarketCap)
		out.FDVUSD = positiveTokenMarketNumber(pair.FDV)
		out.BestPairAddress = strings.TrimSpace(pair.PairAddress)
		out.BestPairDEX = strings.TrimSpace(pair.DexID)
		out.BestPairLiquidityUSD = liquidity
		out.BestPairVolume24hUSD = volume
	}

	out.Volume24hUSD = roundTokenMarketUSD(out.Volume24hUSD)
	out.LiquidityUSD = roundTokenMarketUSD(out.LiquidityUSD)
	out.MarketCapUSD = roundTokenMarketUSD(out.MarketCapUSD)
	out.FDVUSD = roundTokenMarketUSD(out.FDVUSD)
	out.BestPairLiquidityUSD = roundTokenMarketUSD(out.BestPairLiquidityUSD)
	out.BestPairVolume24hUSD = roundTokenMarketUSD(out.BestPairVolume24hUSD)
	if out.PairCount == 0 {
		out.Status = "no_solana_pairs"
		out.Limitations = append(out.Limitations, "No Solana market pair was returned for this mint.")
		return out
	}
	out.Available = out.PriceUSD > 0 || out.Volume24hUSD > 0 || out.LiquidityUSD > 0
	out.Status = "verified_market_snapshot"
	if out.PriceUSD <= 0 {
		out.Limitations = append(out.Limitations, "No base-token USD reference price was available; holder USD values remain unavailable.")
	}
	out.Limitations = append(out.Limitations, "USD holder values use the most liquid returned Solana pair reference price and are not guaranteed liquidation proceeds.")
	return out
}

func positiveTokenMarketNumber(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func roundTokenMarketUSD(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return float64(int64(value*100+0.5)) / 100
}

func compactTokenMarketError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.Join(strings.Fields(strings.TrimSpace(err.Error())), " ")
	if len(message) > 180 {
		message = message[:180]
	}
	return message
}
