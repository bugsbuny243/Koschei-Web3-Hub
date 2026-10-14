package web3

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) Do(r *http.Request) (*http.Response, error) { return f(r) }

func TestRPCManager_FailsOverAfterFiveConsecutiveErrors(t *testing.T) {
	calls := map[string]int{}
	client := roundTrip(func(r *http.Request) (*http.Response, error) {
		calls[r.URL.String()]++
		if r.URL.String() == "https://primary" {
			return nil, errors.New("boom")
		}
		return jsonResponse(`{"jsonrpc":"2.0","result":{"ok":true}}`), nil
	})
	m := NewRPCManager(client, []RPCProviderConfig{{Name: "primary", URL: "https://primary", Priority: 1, MaxFailures: 5, Cooldown: time.Hour}, {Name: "backup", URL: "https://backup", Priority: 2}})
	for i := 0; i < 5; i++ {
		var out map[string]any
		if _, err := m.Call(context.Background(), "x", nil, &out); err != nil {
			t.Fatal(err)
		}
	}
	st, _ := m.State("primary")
	if st.State != CircuitOpen {
		t.Fatalf("primary state=%s", st.State)
	}
	var out map[string]any
	provider, err := m.Call(context.Background(), "x", nil, &out)
	if err != nil {
		t.Fatal(err)
	}
	if provider != "backup" {
		t.Fatalf("provider=%s", provider)
	}
}

func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: ioNopCloser{strings.NewReader(body)}, Header: make(http.Header)}
}

type ioNopCloser struct{ *strings.Reader }

func (ioNopCloser) Close() error { return nil }

func TestRPCManagerForbiddenProviderQuarantinedAndRedacted(t *testing.T) {
	calls := map[string]int{}
	client := roundTrip(func(r *http.Request) (*http.Response, error) {
		calls[r.URL.Host]++
		if r.URL.Host == "primary.example" {
			return &http.Response{StatusCode: http.StatusForbidden, Body: ioNopCloser{strings.NewReader("")}, Header: make(http.Header)}, nil
		}
		return jsonResponse(`{"jsonrpc":"2.0","result":{"ok":true}}`), nil
	})
	m := NewRPCManager(client, []RPCProviderConfig{
		{Name: "primary", URL: "https://primary.example/secret-key", Priority: 1, MaxFailures: 5},
		{Name: "backup", URL: "https://backup.example", Priority: 2},
	})
	for i := 0; i < 3; i++ {
		var result map[string]any
		provider, err := m.Call(context.Background(), "getSlot", nil, &result)
		if err != nil || provider != "backup" {
			t.Fatalf("failover provider=%q err=%v", provider, err)
		}
	}
	if calls["primary.example"] != 1 || calls["backup.example"] != 3 {
		t.Fatalf("unexpected calls: %v", calls)
	}
	state, _ := m.State("primary")
	if state.State != CircuitOpen || !state.OpenedUntil.IsZero() {
		t.Fatalf("403 must remain quarantined: %+v", state)
	}
	if !strings.Contains(state.LastError, "provider=primary rpc status 403") || strings.Contains(state.LastError, "secret-key") {
		t.Fatalf("unsafe or missing provider identity: %q", state.LastError)
	}
}
