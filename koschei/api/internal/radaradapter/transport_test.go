package radaradapter

import (
	"context"
	"testing"
)

func TestReadOnlyTransportRejectsBroadcastBeforeIO(t *testing.T) {
	called := false
	tr := TransportFunc(func(context.Context, Request) (Response, error) { called = true; return Response{}, nil })
	for _, method := range []string{"eth_sendRawTransaction", "eth_sign", "personal_unlockAccount", "sendTransaction", "sendBundle"} {
		if _, _, err := RPC(context.Background(), tr, "fixture", "primary", method, []any{}); err == nil {
			t.Fatal(method)
		}
		if ReadOnlyRequest(Request{Method: "POST", Body: map[string]any{"method": method}}) {
			t.Fatal(method)
		}
	}
	if called {
		t.Fatal("write attempted network IO")
	}
	if ReadOnlyRequest(Request{Method: "POST", Path: "/wallet/broadcasttransaction"}) {
		t.Fatal("Tron broadcast allowed")
	}
}
