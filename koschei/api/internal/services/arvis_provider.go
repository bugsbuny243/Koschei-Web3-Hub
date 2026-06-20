package services

import (
	"os"
	"strings"
)

func resolvedArvisProvider() string {
	rpcURL := strings.ToLower(strings.TrimSpace(os.Getenv("SOLANA_RPC_URL")))
	switch {
	case strings.Contains(rpcURL, "alchemy"):
		return "alchemy"
	case strings.Contains(rpcURL, "helius"):
		return "helius"
	case strings.Contains(rpcURL, "quicknode"):
		return "quicknode"
	case strings.Contains(rpcURL, "triton"):
		return "triton"
	case rpcURL != "":
		return "solana_rpc"
	default:
		return "unconfigured"
	}
}

func applyResolvedArvisProvider(bundle SecurityRadarBundle) SecurityRadarBundle {
	provider := resolvedArvisProvider()
	bundle.Provider = provider
	if bundle.Metadata == nil {
		bundle.Metadata = map[string]any{}
	}
	bundle.Metadata["provider"] = provider
	arms := ArvisArmsFromBundle(bundle)
	if len(arms) > 0 {
		for i := range arms {
			if arms[i].Signals == nil {
				arms[i].Signals = map[string]any{}
			}
			arms[i].Signals["provider"] = provider
		}
		bundle.Metadata["arvis_arms"] = arms
	}
	bundle.PumpSybilRadar.Signals = withArvisProvider(bundle.PumpSybilRadar.Signals, provider)
	bundle.RaydiumPoolGuardian.Signals = withArvisProvider(bundle.RaydiumPoolGuardian.Signals, provider)
	bundle.WalletlessClaimShield.Signals = withArvisProvider(bundle.WalletlessClaimShield.Signals, provider)
	return bundle
}

func withArvisProvider(signals map[string]any, provider string) map[string]any {
	if signals == nil {
		signals = map[string]any{}
	}
	signals["provider"] = provider
	return signals
}
