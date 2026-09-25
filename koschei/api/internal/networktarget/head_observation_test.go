package networktarget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeEVMHeadObservationBindsChainAndHeadDigests(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct {
			ID int `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "eth_chainId":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc":"2.0","id":req.ID,"result":"0x1"})
		case "eth_blockNumber":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc":"2.0","id":req.ID,"result":"0x2a"})
		default:
			http.Error(w, "unexpected", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	got, err := ProbeEVMHeadObservation(context.Background(), server.Client(), server.URL, "ethereum-mainnet", time.Date(2026,9,25,8,0,0,0,time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got.HeadBlock != 42 || got.ChainID != "0x1" || len(got.HeadBlockResponseSHA256) != 64 || len(got.ChainIDResponseSHA256) != 64 {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestProbeEVMHeadObservationRejectsNetworkMismatch(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct{ ID int `json:"id"`; Method string `json:"method"` }
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc":"2.0","id":req.ID,"result":"0x1"})
	}))
	defer server.Close()
	if _, err := ProbeEVMHeadObservation(context.Background(), server.Client(), server.URL, "base-mainnet", time.Now().UTC()); err == nil {
		t.Fatal("network mismatch accepted")
	}
}
