package services

import "testing"

func TestResolveSecurityRadarWSSURLUsesHeliusAPIKeyBeforeAuthlessRPC(t *testing.T) {
	t.Setenv("SOLANA_WSS_URL", "")
	t.Setenv("ALCHEMY_SOLANA_WSS_URL", "")
	t.Setenv("HELIUS_SOLANA_WSS_URL", "")
	t.Setenv("QUICKNODE_SOLANA_WSS_URL", "")
	t.Setenv("HELIUS_API_KEY", "helius test/key")
	t.Setenv("SOLANA_RPC_URL", "https://mainnet.helius-rpc.com/")

	got := resolveSecurityRadarWSSURL()
	want := "wss://mainnet.helius-rpc.com/?api-key=helius+test%2Fkey"
	if got != want {
		t.Fatalf("resolveSecurityRadarWSSURL()=%q want=%q", got, want)
	}
}

func TestResolveSecurityRadarWSSURLPrefersExplicitWSS(t *testing.T) {
	t.Setenv("SOLANA_WSS_URL", "wss://example.invalid/ws")
	t.Setenv("HELIUS_API_KEY", "secret")

	if got := resolveSecurityRadarWSSURL(); got != "wss://example.invalid/ws" {
		t.Fatalf("explicit WSS was not preferred: %q", got)
	}
}
