package handlers

import "strings"

const (
	defaultAPIKeyMonthlyLimit = 1000
	defaultAPIKeyRPM          = 60
)

type apiKeyTierCaps struct {
	MaxMonthly int
	MaxRPM     int
}

var apiKeyCapsByTier = map[string]apiKeyTierCaps{
	"basic":      {MaxMonthly: 1000, MaxRPM: 30},
	"pro":        {MaxMonthly: 20000, MaxRPM: 120},
	"enterprise": {MaxMonthly: 200000, MaxRPM: 600},
}

func apiKeyEffectiveTier(evaluation tokenAccessEvaluation, evaluationErr error) string {
	tier := strings.ToLower(strings.TrimSpace(evaluation.Tier))
	if evaluationErr != nil || !evaluation.WalletVerified || tokenTierRank(tier) == 0 {
		return "basic"
	}
	if _, ok := apiKeyCapsByTier[tier]; !ok {
		return "basic"
	}
	return tier
}

func apiKeyCapsForTier(tier string) apiKeyTierCaps {
	if caps, ok := apiKeyCapsByTier[strings.ToLower(strings.TrimSpace(tier))]; ok {
		return caps
	}
	return apiKeyCapsByTier["basic"]
}

func clampAPIKeyLimits(requestedMonthly, requestedRPM int, caps apiKeyTierCaps) (int, int) {
	monthly := requestedMonthly
	if monthly <= 0 {
		monthly = defaultAPIKeyMonthlyLimit
	}
	rpm := requestedRPM
	if rpm <= 0 {
		rpm = defaultAPIKeyRPM
	}
	if monthly > caps.MaxMonthly {
		monthly = caps.MaxMonthly
	}
	if rpm > caps.MaxRPM {
		rpm = caps.MaxRPM
	}
	return monthly, rpm
}

// APIKeyAuth intentionally avoids an RPC-backed token balance evaluation on
// every developer request. The enterprise caps are the absolute server-side
// ceiling for legacy or manually over-provisioned rows. Route-level KOSCH tier
// authorization remains unchanged and continues to run separately.
func clampAPIPrincipalToAbsoluteCaps(p apiPrincipal) apiPrincipal {
	caps := apiKeyCapsByTier["enterprise"]
	p.MonthlyLimit, p.RateLimitPerMinute = clampAPIKeyLimits(p.MonthlyLimit, p.RateLimitPerMinute, caps)
	return p
}
