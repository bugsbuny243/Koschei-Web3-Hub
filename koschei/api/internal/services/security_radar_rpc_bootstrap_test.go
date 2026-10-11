package services

import (
	"os"
	"testing"
)

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

func TestNormalizeProductionSolanaRPCURLWithoutProviderFailsClosed(t *testing.T) {
	got := normalizeProductionSolanaRPCURL("https://api.devnet.solana.com", "production", "", "")
	if got != "" {
		t.Fatalf("expected no production public fallback, got %q", got)
	}
}

func TestNormalizeProductionSolanaRPCURLEmptyInput(t *testing.T) {
	if got := normalizeProductionSolanaRPCURL("", "production", "", ""); got != "" {
		t.Fatalf("expected empty URL without a configured provider, got %q", got)
	}
}

func TestConfigureSecurityRadarRPCUnsetsRejectedProductionDevnet(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SOLANA_RPC_URL", "https://api.devnet.solana.com")
	t.Setenv("ALCHEMY_SOLANA_RPC_URL", "")
	t.Setenv("HELIUS_SOLANA_RPC_URL", "")
	t.Setenv("QUICKNODE_SOLANA_RPC_URL", "")
	t.Setenv("ALCHEMY_API_KEY", "")
	configureSecurityRadarRPC()
	if got := os.Getenv("SOLANA_RPC_URL"); got != "" {
		t.Fatalf("production devnet must be removed, got %q", got)
	}
}

func TestConfigureSecurityRadarRPCPreservesDevelopmentDevnet(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("SOLANA_RPC_URL", "https://api.devnet.solana.com")
	configureSecurityRadarRPC()
	if got := os.Getenv("SOLANA_RPC_URL"); got != "https://api.devnet.solana.com" {
		t.Fatalf("development RPC unexpectedly changed: %q", got)
	}
}
