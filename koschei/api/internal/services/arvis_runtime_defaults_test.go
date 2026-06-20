package services

import "testing"

func clearArvisRPCTestEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"SOLANA_RPC_URL", "ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL", "ALCHEMY_API_KEY"} {
		t.Setenv(key, "")
	}
}

func TestResolvedArvisRPCURLPriority(t *testing.T) {
	clearArvisRPCTestEnv(t)
	t.Setenv("SOLANA_RPC_URL", "https://primary.example")
	t.Setenv("ALCHEMY_SOLANA_RPC_URL", "https://alchemy.example")
	if got := resolvedArvisRPCURLFromEnv(); got != "https://primary.example" {
		t.Fatalf("expected primary RPC, got %s", got)
	}
}

func TestResolvedArvisRPCURLUsesProviderURL(t *testing.T) {
	clearArvisRPCTestEnv(t)
	t.Setenv("HELIUS_SOLANA_RPC_URL", "https://helius.example")
	if got := resolvedArvisRPCURLFromEnv(); got != "https://helius.example" {
		t.Fatalf("expected Helius RPC, got %s", got)
	}
}

func TestResolvedArvisRPCURLBuildsAlchemyURL(t *testing.T) {
	clearArvisRPCTestEnv(t)
	t.Setenv("ALCHEMY_API_KEY", "test-key")
	want := "https://solana-mainnet.g.alchemy.com/v2/test-key"
	if got := resolvedArvisRPCURLFromEnv(); got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestResolvedArvisRPCURLFallsBackToPublicMainnet(t *testing.T) {
	clearArvisRPCTestEnv(t)
	if got := resolvedArvisRPCURLFromEnv(); got != defaultSolanaMainnetRPC {
		t.Fatalf("expected public fallback %s, got %s", defaultSolanaMainnetRPC, got)
	}
}
