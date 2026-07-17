package services

import "time"

const (
	LPControlVerifiedBurned       = "burned"
	LPControlVerifiedLocked       = "locked_until"
	LPControlHeldByCreator        = "held_by_creator"
	LPControlUnverified           = "unverified"
	LPControlNotApplicable        = "not_applicable"
	LPControlSourceUnavailable    = "source_unavailable"
)

// LPControlEvidence is an informational evidence record. It cannot issue a
// grade and never infers intent from LP ownership alone.
type LPControlEvidence struct {
	Available          bool              `json:"available"`
	Status             string            `json:"status"`
	ReasonCode         string            `json:"reason_code,omitempty"`
	PoolAddress        string            `json:"pool_address,omitempty"`
	PoolProgram        string            `json:"pool_program,omitempty"`
	PoolType           string            `json:"pool_type,omitempty"`
	LPMint             string            `json:"lp_mint,omitempty"`
	TokenVault         string            `json:"token_vault,omitempty"`
	QuoteVault         string            `json:"quote_vault,omitempty"`
	ReadSlot           uint64            `json:"read_slot,omitempty"`
	TokenReserve       float64           `json:"token_reserve,omitempty"`
	QuoteReserve       float64           `json:"quote_reserve,omitempty"`
	LPSupply           float64           `json:"lp_supply,omitempty"`
	BurnedSharePct     float64           `json:"burned_share_pct,omitempty"`
	LargestLPHolders   []LPHolderEvidence `json:"largest_lp_holders"`
	LockerProgram      string            `json:"locker_program,omitempty"`
	LockerAccount      string            `json:"locker_account,omitempty"`
	LockedUntil        *time.Time         `json:"locked_until,omitempty"`
	CreatorLPSharePct  float64           `json:"creator_lp_share_pct,omitempty"`
	ObservedAt         time.Time          `json:"observed_at"`
	EvidenceKeys       []string           `json:"evidence_keys"`
	Limitations        []string           `json:"limitations"`
}

type LPHolderEvidence struct {
	TokenAccount string  `json:"token_account"`
	OwnerWallet  string  `json:"owner_wallet,omitempty"`
	Amount       float64 `json:"amount"`
	SharePct     float64 `json:"share_pct"`
	AccountOwner string  `json:"account_owner,omitempty"`
	Classification string `json:"classification,omitempty"`
}

// JupiterMarketContext is optional market context. A failed or unavailable
// Jupiter request never changes core evidence, verdicts or signing.
type JupiterMarketContext struct {
	Available          bool      `json:"available"`
	Status             string    `json:"status"`
	PriceAvailable     bool      `json:"price_available"`
	PriceUSD           float64   `json:"price_usd,omitempty"`
	PriceBlockID       uint64    `json:"price_block_id,omitempty"`
	PriceObservedAt    time.Time `json:"price_observed_at,omitempty"`
	DexScreenerPriceUSD float64  `json:"dexscreener_price_usd,omitempty"`
	PriceDifferencePct float64   `json:"price_difference_pct,omitempty"`
	SellImpactAvailable bool     `json:"sell_impact_available"`
	SellInputAmountRaw string    `json:"sell_input_amount_raw,omitempty"`
	SellOutputAmountRaw string   `json:"sell_output_amount_raw,omitempty"`
	SellOutputMint     string    `json:"sell_output_mint,omitempty"`
	EstimatedPriceImpactPct float64 `json:"estimated_price_impact_pct,omitempty"`
	QuoteContextSlot   uint64    `json:"quote_context_slot,omitempty"`
	QuoteObservedAt    time.Time `json:"quote_observed_at,omitempty"`
	RouteLabels        []string  `json:"route_labels"`
	Limitations        []string  `json:"limitations"`
}
