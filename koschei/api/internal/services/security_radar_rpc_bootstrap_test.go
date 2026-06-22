package services

import "testing"

func TestNormalizeProductionSolanaRPCURLReplacesDevnet(t *testing.T) {
	got := normalizeProductionSolanaRPCURL(
		"https://solana-devnet.g.alchemy.com/v2/old-key",
		"production",
		"",
		"mainnet-key",
	)
	want := "https://solana-mainnet.g.alchemy.com/v2/mainnet-key"
	if got != want {
		t.Fatalf("unexpected production RPC: got %q want %q", got, want)
	}
}

func TestNormalizeProductionSolanaRPCURLPrefersConfiguredMainnet(t *testing.T) {
	got := normalizeProductionSolanaRPCURL(
		"https://api.devnet.solana.com",
		"production",
		"https://solana-mainnet.g.alchemy.com/v2/configured",
		"fallback-key",
	)
	want := "https://solana-mainnet.g.alchemy.com/v2/configured"
	if got != want {
		t.Fatalf("unexpected configured production RPC: got %q want %q", got, want)
	}
}

func TestNormalizeProductionSolanaRPCURLPreservesDevelopmentDevnet(t *testing.T) {
	devnet := "https://solana-devnet.g.alchemy.com/v2/dev-key"
	if got := normalizeProductionSolanaRPCURL(devnet, "development", "", "main-key"); got != devnet {
		t.Fatalf("development devnet should be preserved: got %q", got)
	}
}

func TestNormalizeProductionSolanaRPCURLPreservesMainnet(t *testing.T) {
	mainnet := "https://solana-mainnet.g.alchemy.com/v2/main-key"
	if got := normalizeProductionSolanaRPCURL(mainnet, "production", "", "other-key"); got != mainnet {
		t.Fatalf("mainnet should be preserved: got %q", got)
	}
}
