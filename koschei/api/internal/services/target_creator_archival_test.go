package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestFetchHeliusTargetCreatorArchivalFindsPumpCreateWithoutGlobalArchivalFlag(t *testing.T) {
	const mint = "MintPump111"
	const creator = "CreatorPump111"
	const signature = "PumpCreateSig111"

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Method != "getTransactionsForAddress" {
			t.Fatalf("unexpected method: %s", payload.Method)
		}
		if len(payload.Params) != 2 || payload.Params[0] != mint {
			t.Fatalf("unexpected params: %#v", payload.Params)
		}
		options, _ := payload.Params[1].(map[string]any)
		if options["sortOrder"] != "asc" || options["transactionDetails"] != "full" || options["encoding"] != "jsonParsed" {
			t.Fatalf("unexpected archival options: %#v", options)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"result": map[string]any{
				"paginationToken": "",
				"data": []any{map[string]any{
					"slot": float64(435759952),
					"blockTime": float64(1753650000),
					"transaction": map[string]any{
						"signatures": []any{signature},
						"message": map[string]any{
							"accountKeys": []any{
								map[string]any{"pubkey": creator, "signer": true},
								map[string]any{"pubkey": mint, "signer": true},
							},
							"instructions": []any{map[string]any{
								"programId": canonicalPumpFunProgramID,
								"parsed": map[string]any{
									"type": "create",
									"info": map[string]any{"mint": mint},
								},
							}},
						},
					},
				}},
			},
		})
	}))
	defer server.Close()

	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	previousClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: heliusRewriteTransport{target: target, base: server.Client().Transport}}
	defer func() { http.DefaultClient = previousClient }()

	t.Setenv("HELIUS_API_KEY", "test-key")
	t.Setenv("HELIUS_CREATED_MINT_ARCHIVAL_ENABLED", "false")
	out := FetchHeliusTargetCreatorArchival(t.Context(), "", mint)

	if calls != 1 {
		t.Fatalf("expected one target archival request, got %d", calls)
	}
	if !out.Configured || !out.Available || out.Status != "observed_external_attribution" {
		t.Fatalf("unexpected archival result: %#v", out)
	}
	if out.Provider != "helius_target_mint_archival" {
		t.Fatalf("unexpected provider: %s", out.Provider)
	}
	if out.Mint != mint || out.Creator != creator || out.Signature != signature {
		t.Fatalf("Pump creator evidence not extracted: %#v", out)
	}
}
