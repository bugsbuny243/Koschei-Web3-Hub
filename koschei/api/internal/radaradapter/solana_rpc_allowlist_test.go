package radaradapter

import (
	"context"
	"testing"
)

func TestSolanaReadOnlyRPCAllowlist(t *testing.T) {
	for _, method := range []string{"getAccountInfo", "getMultipleAccounts", "getSlot", "getGenesisHash"} {
		if !ReadOnlyRPCMethod(method) {
			t.Errorf("Solana read-only method rejected: %s", method)
		}
		if !ReadOnlyRequest(Request{Network: "solana-mainnet", Provider: "primary", Method: "POST", Body: map[string]any{"method": method}}) {
			t.Errorf("Solana read-only request rejected: %s", method)
		}
	}
	for _, method := range []string{"sendTransaction", "requestAirdrop", "simulateTransaction", "getLatestBlockhash", "getSignaturesForAddress"} {
		if ReadOnlyRPCMethod(method) {
			t.Errorf("unreviewed or write-capable method admitted: %s", method)
		}
		if ReadOnlyRequest(Request{Method: "POST", Body: map[string]any{"method": method}}) {
			t.Errorf("unreviewed request admitted: %s", method)
		}
	}
	called := false
	transport := TransportFunc(func(_ context.Context, _ Request) (Response, error) {
		called = true
		return Response{}, nil
	})
	if _, _, err := RPC(context.Background(), transport, "solana-mainnet", "primary", "sendTransaction", []any{}); err == nil || called {
		t.Fatal("write-capable RPC must fail before transport")
	}
}
