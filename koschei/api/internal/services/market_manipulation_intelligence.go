package services

import (
	"context"
	"database/sql"
	"math"
	"strings"
	"time"
)

const (
	MarketManipulationIntelligenceVersion = "koschei-market-manipulation-intelligence-v1"
	MarketManipulationWindowHours         = 24
	MarketManipulationMinimumBuyCount     = 3
	MarketManipulationMinimumSellCount    = 3
	MarketManipulationMinimumGrossSOL     = 5.0
	MarketManipulationMaximumNetGross     = 0.15
	MarketManipulationDefaultLimit        = 25
	MarketManipulationMaximumLimit        = 100
)

type MarketRoundTripChurnCandidate struct {
	Wallet                 string    `json:"wallet"`
	BuyCount               int       `json:"buy_count"`
	SellCount              int       `json:"sell_count"`
	BuySOL                 float64   `json:"buy_sol"`
	SellSOL                float64   `json:"sell_sol"`
	GrossSOL               float64   `json:"gross_sol"`
	NetSOL                 float64   `json:"net_sol"`
	NetToGrossRatio        float64   `json:"net_to_gross_ratio"`
	DistinctSignatureCount int       `json:"distinct_signature_count"`
	FirstObservedAt        time.Time `json:"first_observed_at"`
	LastObservedAt         time.Time `json:"last_observed_at"`
	EvidenceStatus         string    `json:"evidence_status"`
	Classification         string    `json:"classification"`
}

type MarketManipulationIntelligence struct {
	Version            string                          `json:"version"`
	Mint               string                          `json:"mint"`
	Status             string                          `json:"status"`
	Available          bool                            `json:"available"`
	Complete           bool                            `json:"complete"`
	WindowHours        int                             `json:"window_hours"`
	CandidateCount     int                             `json:"candidate_count"`
	Candidates         []MarketRoundTripChurnCandidate `json:"candidates"`
	WashTradingProven       bool                            `json:"wash_trading_proven"`
	ManipulationClaim       bool                            `json:"manipulation_claim"`
	SameOperatorClaim       bool                            `json:"same_operator_claim"`
	VerdictAuthority        bool                            `json:"verdict_authority"`
	Thresholds              map[string]any                  `json:"thresholds"`
	Limitations             []string                        `json:"limitations"`
	GeneratedAt             time.Time                       `json:"generated_at"`
}

// LoadMarketManipulationIntelligence identifies transaction-ledger patterns that
// merit investigation. A balanced buy/sell round trip is not, by itself, proof
// of wash trading or manipulation. Legitimate market making, arbitrage, routing
// and ordinary active trading remain possible explanations until stronger
// counterparty, timing and control evidence is available.
func LoadMarketManipulationIntelligence(ctx context.Context, db *sql.DB, mint string, now time.Time, limit int) (MarketManipulationIntelligence, error) {
	mint = strings.TrimSpace(mint)
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	if limit <= 0 {
		limit = MarketManipulationDefaultLimit
	}
	if limit > MarketManipulationMaximumLimit {
		limit = MarketManipulationMaximumLimit
	}
	out := MarketManipulationIntelligence{
		Version:       MarketManipulationIntelligenceVersion,
		Mint:          mint,
		Status:        "trade_ledger_unavailable",
		WindowHours:   MarketManipulationWindowHours,
		Candidates:    []MarketRoundTripChurnCandidate{},
		Thresholds: map[string]any{
			"minimum_buy_count":       MarketManipulationMinimumBuyCount,
			"minimum_sell_count":      MarketManipulationMinimumSellCount,
			"minimum_gross_sol":       MarketManipulationMinimumGrossSOL,
			"maximum_net_gross_ratio": MarketManipulationMaximumNetGross,
		},
		Limitations: []string{
			"Round-trip churn is an investigation indicator, not proof of wash trading, manipulation, common control, intent or wrongdoing.",
			"Legitimate market making, arbitrage, routing and active trading can produce similar buy/sell balance patterns.",
			"This v1 classifier does not yet prove counterparty self-dealing or coordinated control across wallets.",
		},
		GeneratedAt: now,
	}
	if db == nil || mint == "" {
		return out, nil
	}

	windowStart := now.Add(-MarketManipulationWindowHours * time.Hour)
	rows, err := db.QueryContext(ctx, `
		WITH per_trader AS (
			SELECT
				trader,
				count(*) FILTER (WHERE side='buy')::bigint AS buy_count,
				count(*) FILTER (WHERE side='sell')::bigint AS sell_count,
				COALESCE(sum(ABS(sol_amount)) FILTER (WHERE side='buy'),0)::double precision AS buy_sol,
				COALESCE(sum(ABS(sol_amount)) FILTER (WHERE side='sell'),0)::double precision AS sell_sol,
				count(DISTINCT NULLIF(btrim(signature),''))::bigint AS signature_count,
				min(COALESCE(block_time,created_at)) AS first_seen_at,
				max(COALESCE(block_time,created_at)) AS last_seen_at
			FROM token_trade_events
			WHERE mint=$1
			  AND COALESCE(block_time,created_at) >= $2
			  AND COALESCE(block_time,created_at) <= $3
			  AND side IN ('buy','sell')
			  AND btrim(trader)<>''
			GROUP BY trader
		), candidates AS (
			SELECT *,
				(buy_sol + sell_sol) AS gross_sol,
				ABS(buy_sol - sell_sol) AS net_sol,
				CASE WHEN (buy_sol + sell_sol)>0
					THEN ABS(buy_sol - sell_sol)/(buy_sol + sell_sol)
					ELSE 1 END AS net_gross_ratio
			FROM per_trader
		)
		SELECT trader,buy_count,sell_count,buy_sol,sell_sol,gross_sol,net_sol,
		       net_gross_ratio,signature_count,first_seen_at,last_seen_at
		FROM candidates
		WHERE buy_count >= $4
		  AND sell_count >= $5
		  AND gross_sol >= $6
		  AND net_gross_ratio <= $7
		ORDER BY gross_sol DESC,last_seen_at DESC,trader ASC
		LIMIT $8`,
		mint, windowStart, now,
		MarketManipulationMinimumBuyCount, MarketManipulationMinimumSellCount,
		MarketManipulationMinimumGrossSOL, MarketManipulationMaximumNetGross, limit,
	)
	if err != nil {
		if isSecurityRadarMissingRelation(err) {
			out.Status = "trade_ledger_schema_unavailable"
			return out, nil
		}
		return out, err
	}
	defer rows.Close()

	for rows.Next() {
		var candidate MarketRoundTripChurnCandidate
		var buyCount, sellCount, signatureCount int64
		if err := rows.Scan(
			&candidate.Wallet,
			&buyCount,
			&sellCount,
			&candidate.BuySOL,
			&candidate.SellSOL,
			&candidate.GrossSOL,
			&candidate.NetSOL,
			&candidate.NetToGrossRatio,
			&signatureCount,
			&candidate.FirstObservedAt,
			&candidate.LastObservedAt,
		); err != nil {
			return out, err
		}
		candidate.BuyCount = int(buyCount)
		candidate.SellCount = int(sellCount)
		candidate.DistinctSignatureCount = int(signatureCount)
		candidate.BuySOL = roundManipulationMetric(candidate.BuySOL)
		candidate.SellSOL = roundManipulationMetric(candidate.SellSOL)
		candidate.GrossSOL = roundManipulationMetric(candidate.GrossSOL)
		candidate.NetSOL = roundManipulationMetric(candidate.NetSOL)
		candidate.NetToGrossRatio = roundManipulationMetric(candidate.NetToGrossRatio)
		candidate.EvidenceStatus = "observed"
		if candidate.DistinctSignatureCount >= candidate.BuyCount+candidate.SellCount && candidate.DistinctSignatureCount > 0 {
			candidate.EvidenceStatus = "verified_ledger_rows"
		}
		candidate.Classification = "balanced_round_trip_churn_candidate"
		out.Candidates = append(out.Candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	out.Available = true
	out.Complete = true
	out.CandidateCount = len(out.Candidates)
	if out.CandidateCount > 0 {
		out.Status = "round_trip_churn_candidates_observed"
	} else {
		out.Status = "no_round_trip_churn_candidate_observed"
	}
	return out, nil
}

func roundManipulationMetric(value float64) float64 {
	return math.Round(value*10000) / 10000
}
