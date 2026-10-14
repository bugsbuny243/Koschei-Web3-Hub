package handlers

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublicImpactCannotPublishUnverifiedAmounts(t *testing.T) {
	// A previously cached record must not make synthetic claims serializable.
	for _, metrics := range []publicImpactMetrics{{}, unavailablePublicImpactMetrics()} {
		metrics.TotalSavedUSD = 999
		metrics.MEVEstimatedLossPreventedUSD = 888
		metrics.LiquidityLossPreventedUSD = 777
		metrics.Last24HPreventedUSD = 666
		metrics.Last7DPreventedUSD = 555
		metrics.RugPullsPrevented = 4
		metrics.Last24HBiggestPrevention = &publicImpactPreventionLog{AmountUSD: 444, RiskScore: 99, RiskLevel: "critical"}
		metrics.RecentLogs = []publicImpactPreventionLog{{AmountUSD: 333, RiskScore: 98, RiskLevel: "high", Source: "fixture"}}
		raw, err := json.Marshal(metrics)
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"total_saved_usd", "mev_estimated_loss_prevented_usd", "liquidity_loss_prevented_usd", "last_24h_prevented_usd", "last_7d_prevented_usd", "rug_pulls_prevented", "last_24h_biggest_prevention", "amount_usd", "risk_score", "risk_level"} {
			if strings.Contains(string(raw), `"`+key+`"`) {
				t.Errorf("unverified field %s published", key)
			}
		}
	}
}
