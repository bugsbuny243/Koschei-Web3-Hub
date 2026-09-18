package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"koschei/api/internal/cache"
	"koschei/api/internal/services"
	"koschei/api/internal/web3"
)

func TestInvestigateActorInitialRecipientsUsesRPCManagerFailover(t *testing.T) {
	const creator = "CreatorFailover111"
	const mint = "MintFailover111"
	const sourceTokenAccount = "CreatorTokenAccount111"
	const creationSignature = "CreationSigFailover111"

	var primaryCalls atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"Invalid param: Invalid"}}`))
	}))
	defer primary.Close()

	var fallbackCalls atomic.Int32
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls.Add(1)
		defer r.Body.Close()
		var request struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "getTransaction":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"slot":888123,"blockTime":1783364366,"transaction":{"message":{"accountKeys":[{"pubkey":"CreatorFailover111","signer":true},{"pubkey":"CreatorTokenAccount111","signer":false}],"instructions":[]}},"meta":{"err":null,"preTokenBalances":[],"postTokenBalances":[{"accountIndex":1,"mint":"MintFailover111","owner":"CreatorFailover111"}]}}}`))
		case "getTokenAccountsByOwner":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"value":[]}}`))
		case "getSignaturesForAddress":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":[]}`))
		case "getTokenSupply":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"value":{"amount":"100","decimals":0,"uiAmount":100,"uiAmountString":"100"}}}`))
		case "getTokenLargestAccounts":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"value":[]}}`))
		default:
			http.Error(w, "unexpected method: "+request.Method, http.StatusBadRequest)
		}
	}))
	defer fallback.Close()

	web3.ResetSolanaRPCProviderGovernorForTest()
	t.Setenv("APP_ENV", "test")
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "0")
	t.Setenv("SOLANA_RPC_URL", primary.URL)
	t.Setenv("SOLANA_RPC_FALLBACK_URL", fallback.URL)
	t.Setenv("ALCHEMY_SOLANA_RPC_URL", "")
	t.Setenv("HELIUS_SOLANA_RPC_URL", "")
	t.Setenv("QUICKNODE_SOLANA_RPC_URL", "")

	rpc := web3.NewSolanaRPC(cache.NewNoop())
	rpc.Client = http.DefaultClient
	h := &Handler{SolanaRPC: rpc}
	report := h.investigateActorInitialRecipients(
		t.Context(),
		creator,
		mint,
		creationSignature,
		"solana-mainnet",
		services.ActorInitialRecipientOptions{
			MaxRecipients:        5,
			SignaturePageSize:    10,
			MaxPagesPerTokenATA:  1,
			MaxTransactionsParse: 5,
		},
	)

	if primaryCalls.Load() == 0 || fallbackCalls.Load() == 0 {
		t.Fatalf("expected primary failure and fallback recovery; primary=%d fallback=%d", primaryCalls.Load(), fallbackCalls.Load())
	}
	if len(report.SourceTokenAccounts) != 1 || report.SourceTokenAccounts[0] != sourceTokenAccount {
		t.Fatalf("creation transaction source account was not recovered through fallback: %#v", report)
	}
	if report.Status != "no_creator_distribution_observed" || !report.HistoryComplete {
		t.Fatalf("unexpected bounded distribution report after fallback: %#v", report)
	}
	for _, limitation := range report.Limitations {
		if limitation == "Creation transaction token-account resolution failed: rpc error: Invalid param: Invalid" {
			t.Fatalf("primary provider error leaked despite successful fallback: %#v", report.Limitations)
		}
	}
}
