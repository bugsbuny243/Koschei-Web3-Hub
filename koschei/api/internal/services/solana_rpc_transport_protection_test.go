package services

import (
	"net/http"
	"testing"

	"koschei/api/internal/outboundhttp"
)

func TestSolanaFailoverTransportPreservesProtectedOutboundBoundary(t *testing.T) {
	t.Setenv("APP_ENV", "production")

	protected, err := (&solanaFailoverTransport{base: http.DefaultTransport}).ProtectOutboundTransport()
	if err != nil {
		t.Fatalf("protect failover transport: %v", err)
	}
	if _, ok := protected.(*solanaFailoverTransport); !ok {
		t.Fatalf("protected transport type=%T want *solanaFailoverTransport", protected)
	}
}

func TestConfiguredSolanaRPCTransportChainIsProtectableInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")

	if solanaRPCClient.Transport == nil {
		t.Fatal("configured Solana RPC transport is nil")
	}
	if _, err := outboundhttp.ProtectTransport(solanaRPCClient.Transport); err != nil {
		t.Fatalf("configured Solana RPC transport chain must preserve protected boundary: %v", err)
	}
}
