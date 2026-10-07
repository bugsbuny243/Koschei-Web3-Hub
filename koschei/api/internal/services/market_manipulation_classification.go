package services

import "math"

// QualifiesMarketRoundTripChurn applies the same explicit thresholds used by
// the persisted trade-ledger query. It is a behavior classifier only; true does
// not mean wash trading or market manipulation has been proven.
func QualifiesMarketRoundTripChurn(buyCount, sellCount int, buySOL, sellSOL float64) bool {
	if buyCount < MarketManipulationMinimumBuyCount || sellCount < MarketManipulationMinimumSellCount {
		return false
	}
	gross := math.Abs(buySOL) + math.Abs(sellSOL)
	if gross < MarketManipulationMinimumGrossSOL || gross <= 0 {
		return false
	}
	net := math.Abs(math.Abs(buySOL) - math.Abs(sellSOL))
	return net/gross <= MarketManipulationMaximumNetGross
}
