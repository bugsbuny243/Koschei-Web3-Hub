package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeSolanaValidatorTelemetryNormalizesStakeShares(t *testing.T) {
	resetSolanaRPCCachesForTest()
	t.Cleanup(resetSolanaRPCCachesForTest)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Method != "getVoteAccounts" {
			t.Fatalf("unexpected method %q", req.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id": req.ID,
			"result": map[string]any{
				"current": []any{
					map[string]any{
						"votePubkey": "Vote111111111111111111111111111111111111111",
						"nodePubkey": "Node111111111111111111111111111111111111111",
						"activatedStake": 75,
						"commission": 5,
						"lastVote": 100,
						"rootSlot": 90,
						"epochVoteAccount": true,
					},
				},
				"delinquent": []any{
					map[string]any{
						"votePubkey": "Vote222222222222222222222222222222222222222",
						"nodePubkey": "Node222222222222222222222222222222222222222",
						"activatedStake": 25,
						"commission": 10,
						"lastVote": 80,
						"rootSlot": 70,
						"epochVoteAccount": true,
					},
				},
			},
		})
	}))
	defer server.Close()

	got, err := ProbeSolanaValidatorTelemetry(t.Context(), server.URL, time.Date(2026, 9, 23, 3, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentCount != 1 || got.DelinquentCount != 1 || got.TotalActivatedStake != 100 || len(got.Validators) != 2 {
		t.Fatalf("unexpected report: %#v", got)
	}
	if got.Validators[0].Observation.StakeSharePct == nil || *got.Validators[0].Observation.StakeSharePct != 75 {
		t.Fatalf("current stake share=%v", got.Validators[0].Observation.StakeSharePct)
	}
	if got.Validators[1].Observation.StakeSharePct == nil || *got.Validators[1].Observation.StakeSharePct != 25 || !got.Validators[1].Delinquent {
		t.Fatalf("delinquent validator mismatch: %#v", got.Validators[1])
	}
	if got.Validators[0].Observation.Source != "solana-rpc:getVoteAccounts" {
		t.Fatalf("source=%q", got.Validators[0].Observation.Source)
	}
}
