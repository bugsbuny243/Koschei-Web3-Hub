package radaradapter

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestTronWalletAdapterCollectObserved(t *testing.T) {
	const account = "41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	client := &http.Client{Transport: tronAccountRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{"address":"` + account +
				`","owner_permission":{"type":0,"id":0,"threshold":1,"keys":[{"address":"` +
				account + `","weight":1}]}}`))}, nil
	})}
	adapter, err := NewTronWalletAdapter(client, func(context.Context) (int64, error) { return 123, nil }, account)
	if err != nil {
		t.Fatal(err)
	}
	target := Target{Network: "tron-mainnet", Kind: "wallet", Raw: account}
	if !adapter.Supports(target) || adapter.Supports(Target{Network: "ethereum", Kind: "wallet", Raw: account}) {
		t.Fatal("invalid Tron adapter routing")
	}
	rows, limitations, err := adapter.Collect(context.Background(), target)
	if err != nil || len(rows) != 1 || rows[0].Status != "observed" || len(limitations) != 1 {
		t.Fatalf("unexpected Tron adapter result: %+v %+v %v", rows, limitations, err)
	}
	if rows[0].Attributes["account_state_height_verified"] != false {
		t.Fatal("unanchored account state promoted to verified")
	}
}

func TestTronWalletAdapterFailsClosed(t *testing.T) {
	const account = "41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := NewTronWalletAdapter(nil, nil, account); err == nil {
		t.Fatal("accepted missing height reader")
	}
	adapter, err := NewTronWalletAdapter(nil, func(context.Context) (int64, error) { return 0, nil }, account)
	if err != nil {
		t.Fatal(err)
	}
	target := Target{Network: "tron-mainnet", Kind: "account", Raw: account}
	if _, _, err := adapter.Collect(context.Background(), target); err == nil {
		t.Fatal("accepted missing chain height")
	}
	adapter.ReadHeight = func(context.Context) (int64, error) { return 0, errors.New("height unavailable") }
	if _, _, err := adapter.Collect(context.Background(), target); err == nil {
		t.Fatal("ignored height provider failure")
	}
	rows, limits, err := adapter.Collect(context.Background(), Target{Network: "tron-mainnet", Kind: "token", Raw: account})
	if err != nil || len(rows) != 0 || len(limits) != 1 {
		t.Fatalf("unsupported target not rejected: %+v %+v %v", rows, limits, err)
	}
}
