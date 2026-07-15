package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"koschei/api/internal/cache"
	"koschei/api/internal/web3"
)

func TestRequireTokenTierRejectsBasicHolderFromProRoute(t *testing.T) {
	db := openQuotaTestDB(t)
	defer db.Close()
	subject := "tier-test-basic-subject"
	wallet := "BasicWallet11111111111111111111111111111"
	_, _ = db.Exec(`DELETE FROM token_access_snapshots WHERE auth_subject=$1`, subject)
	_, _ = db.Exec(`DELETE FROM verified_wallet_links WHERE auth_subject=$1`, subject)
	defer func() {
		_, _ = db.Exec(`DELETE FROM token_access_snapshots WHERE auth_subject=$1`, subject)
		_, _ = db.Exec(`DELETE FROM verified_wallet_links WHERE auth_subject=$1`, subject)
	}()
	if _, err := db.Exec(`
		INSERT INTO verified_wallet_links(auth_subject,wallet_address,network,status)
		VALUES($1,$2,'solana-mainnet','active')`, subject, wallet); err != nil {
		t.Fatal(err)
	}

	rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		var result any
		switch request.Method {
		case "getTokenSupply":
			result = map[string]any{"value": map[string]any{
				"amount": "1000000000000000", "decimals": 6, "uiAmountString": "1000000000",
			}}
		case "getTokenAccountsByOwner":
			result = map[string]any{"value": []any{
				map[string]any{
					"pubkey": "BasicATA1111111111111111111111111111111",
					"account": map[string]any{
						"data": map[string]any{
							"parsed": map[string]any{
								"info": map[string]any{
									"mint": "11111111111111111111111111111111",
									"tokenAmount": map[string]any{
										"amount": "25000000000", "decimals": 6, "uiAmountString": "25000",
									},
								},
							},
						},
					},
				},
			}}
		default:
			result = map[string]any{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}))
	defer rpcServer.Close()
	t.Setenv("SOLANA_RPC_URL", rpcServer.URL)
	t.Setenv("SOLANA_RPC_FALLBACK_URL", rpcServer.URL)
	t.Setenv("KOSCHEI_TOKEN_GATE_ENABLED", "true")
	t.Setenv("KOSCHEI_TOKEN_MINT", "11111111111111111111111111111111")
	t.Setenv("KOSCHEI_TOKEN_TIER_BASIC", "25000")
	t.Setenv("KOSCHEI_TOKEN_TIER_PRO", "250000")
	t.Setenv("KOSCHEI_TOKEN_TIER_ENTERPRISE", "2000000")

	rpc := web3.NewSolanaRPC(cache.NewNoop())
	rpc.Client = rpcServer.Client()
	h := &Handler{DB: db, SolanaRPC: rpc}
	nextCalled := false
	wrapped := h.RequireTokenTier("pro", func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/radar/actor-intelligence", nil)
	req = req.WithContext(context.WithValue(req.Context(), authContextKey, neonJWTClaims{Sub: subject}))
	recorder := httptest.NewRecorder()
	wrapped(recorder, req)

	if nextCalled {
		t.Fatal("Basic holder reached Pro handler")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["error"] != "token_tier_required" || payload["required_tier"] != "pro" || payload["current_tier"] != "basic" {
		t.Fatalf("unexpected tier response: %#v", payload)
	}
}
