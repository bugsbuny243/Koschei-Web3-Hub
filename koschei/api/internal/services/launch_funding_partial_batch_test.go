package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLaunchFundingPreservesVerifiedLinkWhenSiblingTransactionFails(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "batch")
	t.Setenv("SOLANA_RPC_TX_BATCH_SIZE", "8")

	const wallet = "test-funded-wallet"
	const funder = "verified-creator-funder"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Errorf("decode RPC: %v", err)
			return
		}
		if len(raw) == 0 {
			t.Error("empty RPC envelope")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if raw[0] == '[' {
			var requests []solanaRPCRequest
			if err := json.Unmarshal(raw, &requests); err != nil {
				t.Errorf("decode RPC batch: %v", err)
				return
			}
			responses := make([]map[string]any, 0, len(requests))
			for _, request := range requests {
				params := request.Params.([]any)
				if params[0] == "sig-verified" {
					responses = append(responses, map[string]any{
						"jsonrpc": "2.0", "id": request.ID,
						"result": map[string]any{
							"slot": 50,
							"transaction": map[string]any{"message": map[string]any{
								"accountKeys": []any{funder, wallet},
							}},
							"meta": map[string]any{
								"preBalances":  []any{2000000000, 0},
								"postBalances": []any{1000000000, 1000000000},
							},
						},
					})
				} else {
					responses = append(responses, map[string]any{
						"jsonrpc": "2.0", "id": request.ID,
						"error": map[string]any{"code": -32015, "message": "Transaction version (1) is not supported by the requesting client"},
					})
				}
			}
			_ = json.NewEncoder(w).Encode(responses)
			return
		}
		var request solanaRPCRequest
		if err := json.Unmarshal(raw, &request); err != nil {
			t.Errorf("decode RPC single: %v", err)
			return
		}
		if request.Method != "getSignaturesForAddress" {
			t.Errorf("unexpected RPC method: %s", request.Method)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": request.ID,
			"result": []map[string]any{
				{"signature": "sig-unsupported", "slot": 51, "err": nil},
				{"signature": "sig-verified", "slot": 50, "err": nil},
			},
		})
	}))
	defer server.Close()

	profile := &LaunchActorProfile{OwnerWallet: wallet}
	cfg := launchForensicsConfig{FundingRPCBudget: 10, FundingSigLimit: 10, FundingTxParse: 8}
	traceLaunchFunding(context.Background(), server.URL, funder, funder, profile, cfg, newHolderScanRPCBudget(10))
	if !profile.CreatorLinked || profile.FundingStatus != "creator_linked" {
		t.Fatalf("verified creator funding disappeared after sibling RPC failure: %#v", profile)
	}
	if len(profile.FundingPath) != 2 || profile.FundingPath[1] != funder {
		t.Fatalf("canonical funding edge not retained: %#v", profile.FundingPath)
	}
	partialNoted := false
	for _, evidence := range profile.Evidence {
		if strings.Contains(evidence, "was partial") {
			partialNoted = true
		}
	}
	if !partialNoted {
		t.Fatalf("partial RPC evidence boundary was hidden: %#v", profile.Evidence)
	}
}

func TestLaunchFundingEarlierPageSurvivesLaterSignaturePageError(t *testing.T) {
	prepareSolanaRPCBatchTest(t)
	t.Setenv("SOLANA_RPC_BATCH_MODE", "batch")
	const wallet, funder = "wallet-bounded-history", "funder-bounded-history"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Errorf("decode RPC: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if len(raw) > 0 && raw[0] == '[' {
			var requests []solanaRPCRequest
			if err := json.Unmarshal(raw, &requests); err != nil {
				t.Errorf("decode batch: %v", err)
				return
			}
			rows := make([]map[string]any, 0, len(requests))
			for _, request := range requests {
				rows = append(rows, map[string]any{
					"jsonrpc": "2.0", "id": request.ID,
					"result": map[string]any{
						"slot": 1,
						"transaction": map[string]any{"message": map[string]any{"accountKeys": []any{funder, wallet}}},
						"meta": map[string]any{
							"preBalances":  []any{2000000000, 0},
							"postBalances": []any{1000000000, 1000000000},
						},
					},
				})
			}
			_ = json.NewEncoder(w).Encode(rows)
			return
		}
		var request solanaRPCRequest
		if err := json.Unmarshal(raw, &request); err != nil {
			t.Errorf("decode single: %v", err)
			return
		}
		params := request.Params.([]any)
		options := params[1].(map[string]any)
		if options["before"] != nil {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": request.ID,
				"error": map[string]any{"code": -32005, "message": "node is behind"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": request.ID,
			"result": []map[string]any{
				{"signature": "sig-newer", "slot": 2, "err": nil},
				{"signature": "sig-older", "slot": 1, "err": nil},
			},
		})
	}))
	defer server.Close()
	cfg := launchForensicsConfig{FundingSigLimit: 2, FundingTxParse: 8, FundingRPCBudget: 10}
	source, amount, incomplete := earliestIncomingSOLSource(context.Background(), server.URL, wallet, cfg, newHolderScanRPCBudget(10))
	if source != funder || amount != 1 || !incomplete {
		t.Fatalf("verified earlier-page funding must survive later-page failure with incomplete=true: source=%q amount=%v incomplete=%v", source, amount, incomplete)
	}
}
