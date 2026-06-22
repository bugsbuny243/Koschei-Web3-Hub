package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type rpcMethodCounter struct {
	sync.Mutex
	methods map[string]int
}

func (c *rpcMethodCounter) add(method string) {
	c.Lock()
	defer c.Unlock()
	c.methods[method]++
}

func (c *rpcMethodCounter) get(method string) int {
	c.Lock()
	defer c.Unlock()
	return c.methods[method]
}

func TestTokenRPCMethodsSkipKnownNonMintTargets(t *testing.T) {
	resetSolanaRPCCachesForTest()
	counter := &rpcMethodCounter{methods: map[string]int{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		counter.add(request.Method)
		w.Header().Set("Content-Type", "application/json")
		if request.Method != "getAccountInfo" {
			t.Fatalf("unexpected provider method for non-mint target: %s", request.Method)
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"value":{"data":{"parsed":{"type":"account","info":{}}},"executable":false,"lamports":1,"owner":"11111111111111111111111111111111","rentEpoch":0,"space":0}}}`))
	}))
	defer server.Close()

	ctx := context.Background()
	address := "11111111111111111111111111111111"
	if _, err := SolanaGetAccountInfoJSONParsed(ctx, server.URL, address); err != nil {
		t.Fatalf("account info failed: %v", err)
	}
	if _, err := SolanaGetTokenSupply(ctx, server.URL, address); !errors.Is(err, ErrSolanaTargetNotTokenMint) {
		t.Fatalf("expected non-mint error from supply, got %v", err)
	}
	if _, err := SolanaGetTokenLargestAccounts(ctx, server.URL, address); !errors.Is(err, ErrSolanaTargetNotTokenMint) {
		t.Fatalf("expected non-mint error from largest accounts, got %v", err)
	}
	if got := counter.get("getAccountInfo"); got != 1 {
		t.Fatalf("expected one account-info preflight, got %d", got)
	}
	if got := counter.get("getTokenSupply"); got != 0 {
		t.Fatalf("invalid token-supply request reached provider: %d", got)
	}
	if got := counter.get("getTokenLargestAccounts"); got != 0 {
		t.Fatalf("invalid largest-accounts request reached provider: %d", got)
	}
}

func TestTokenRPCMethodsRunForValidatedMint(t *testing.T) {
	resetSolanaRPCCachesForTest()
	counter := &rpcMethodCounter{methods: map[string]int{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		counter.add(request.Method)
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "getAccountInfo":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"value":{"data":{"parsed":{"type":"mint","info":{"decimals":6}}},"executable":false,"lamports":1,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":0,"space":82}}}`))
		case "getTokenSupply":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"value":{"amount":"1000000","decimals":6,"uiAmount":1,"uiAmountString":"1"}}}`))
		case "getTokenLargestAccounts":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"value":[{"address":"holder","amount":"1000000","decimals":6,"uiAmount":1,"uiAmountString":"1"}]}}`))
		default:
			t.Fatalf("unexpected method: %s", request.Method)
		}
	}))
	defer server.Close()

	ctx := context.Background()
	mint := "So11111111111111111111111111111111111111112"
	if _, err := SolanaGetAccountInfoJSONParsed(ctx, server.URL, mint); err != nil {
		t.Fatalf("account info failed: %v", err)
	}
	if _, err := SolanaGetTokenSupply(ctx, server.URL, mint); err != nil {
		t.Fatalf("token supply failed: %v", err)
	}
	if _, err := SolanaGetTokenLargestAccounts(ctx, server.URL, mint); err != nil {
		t.Fatalf("largest accounts failed: %v", err)
	}
	if counter.get("getAccountInfo") != 1 || counter.get("getTokenSupply") != 1 || counter.get("getTokenLargestAccounts") != 1 {
		t.Fatalf("unexpected provider call counts: %#v", counter.methods)
	}
}

func TestLargestAccountsScanLimitIsCached(t *testing.T) {
	resetSolanaRPCCachesForTest()
	counter := &rpcMethodCounter{methods: map[string]int{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		counter.add(request.Method)
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "getAccountInfo":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"value":{"data":{"parsed":{"type":"mint","info":{}}},"executable":false,"lamports":1,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":0,"space":82}}}`))
		case "getTokenLargestAccounts":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32012,"message":"scan aborted: The accumulated scan results exceeded the limit"}}`))
		default:
			t.Fatalf("unexpected method: %s", request.Method)
		}
	}))
	defer server.Close()

	ctx := context.Background()
	mint := "So11111111111111111111111111111111111111112"
	if _, err := SolanaGetAccountInfoJSONParsed(ctx, server.URL, mint); err != nil {
		t.Fatalf("account info failed: %v", err)
	}
	if _, err := SolanaGetTokenLargestAccounts(ctx, server.URL, mint); err == nil {
		t.Fatal("expected scan-limit error")
	}
	if _, err := SolanaGetTokenLargestAccounts(ctx, server.URL, mint); err == nil {
		t.Fatal("expected cached scan-limit error")
	}
	if got := counter.get("getTokenLargestAccounts"); got != 1 {
		t.Fatalf("expected one provider scan before cooldown, got %d", got)
	}
}
