package web3

import "time"

type RPCProviderConfig struct {
	Name        string
	URL         string
	Priority    int
	Timeout     time.Duration
	Cooldown    time.Duration
	MaxFailures int
}

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

type RPCProviderState struct {
	Config      RPCProviderConfig
	State       CircuitState
	Failures    int
	OpenedUntil time.Time
	LastError   string
	LastSuccess time.Time
}

type NormalizedTokenData struct {
	Mint                 string    `json:"mint"`
	Network              string    `json:"network"`
	SupplyRaw            string    `json:"supply_raw"`
	Decimals             int       `json:"decimals"`
	MintAuthority        *string   `json:"mint_authority,omitempty"`
	FreezeAuthority      *string   `json:"freeze_authority,omitempty"`
	LargestHolderPercent float64   `json:"largest_holder_percent"`
	TopTenPercent        float64   `json:"top_ten_percent"`
	SourceProvider       string    `json:"source_provider"`
	FetchedAt            time.Time `json:"fetched_at"`
}

type NormalizedTxData struct {
	Signature      string    `json:"signature"`
	Slot           uint64    `json:"slot"`
	BlockTime      time.Time `json:"block_time"`
	FeeLamports    uint64    `json:"fee_lamports"`
	Signers        []string  `json:"signers"`
	Instructions   []string  `json:"instructions"`
	SourceProvider string    `json:"source_provider"`
}

type TokenRiskResult struct {
	Token      NormalizedTokenData `json:"token"`
	Score      int                 `json:"score"`
	RiskLevel  string              `json:"risk_level"`
	Findings   []string            `json:"findings"`
	Disclaimer string              `json:"disclaimer"`
}

const (
	TokenMetadataTTL      = 24 * time.Hour
	RugCheckTTL           = time.Hour
	HolderDistributionTTL = time.Hour
	LivePriceTTL          = 30 * time.Second
)
