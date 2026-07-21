package services

import "testing"

func TestResolvedSolanaRPCURLPreservesExplicitEndpoint(t *testing.T) {
	t.Setenv("SOLANA_RPC_URL", "https://canonical.invalid")
	if got := resolvedSolanaRPCURL(" https://explicit.invalid "); got != "https://explicit.invalid" {
		t.Fatalf("resolved endpoint = %q", got)
	}
}

func TestResolvedSolanaRPCURLUsesProviderSpecificFallback(t *testing.T) {
	t.Setenv("SOLANA_RPC_URL", "")
	t.Setenv("ALCHEMY_SOLANA_RPC_URL", "")
	t.Setenv("HELIUS_SOLANA_RPC_URL", "https://helius.invalid")
	t.Setenv("QUICKNODE_SOLANA_RPC_URL", "")
	t.Setenv("ALCHEMY_API_KEY", "")
	if got := resolvedSolanaRPCURL(""); got != "https://helius.invalid" {
		t.Fatalf("resolved endpoint = %q", got)
	}
}
