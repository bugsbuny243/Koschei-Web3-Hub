package services

import "testing"

func TestPumpPortalEndpointIsNotSolanaRPCFallback(t *testing.T) {
	t.Setenv("SOLANA_WSS_URL", "")
	t.Setenv("ALCHEMY_SOLANA_WSS_URL", "")
	t.Setenv("HELIUS_SOLANA_WSS_URL", "")
	t.Setenv("QUICKNODE_SOLANA_WSS_URL", "")
	t.Setenv("SOLANA_RPC_URL", "")
	t.Setenv("ALCHEMY_API_KEY", "")
	t.Setenv("PUMPPORTAL_DATA_WS", "wss://pumpportal.fun/api/data")

	if got := resolveSecurityRadarWSSURL(); got != "" {
		t.Fatalf("PumpPortal data websocket must not be used as a Solana JSON-RPC endpoint: %s", got)
	}
}
