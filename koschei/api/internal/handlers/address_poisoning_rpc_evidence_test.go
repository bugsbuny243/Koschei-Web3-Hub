package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAddressPoisoningHandlerUsesOnlyOutgoingRPCRecipients(t *testing.T) {
	wallet := testRecipientAddress("W")
	candidate := testRecipientAddress("C")
	irrelevant := testRecipientAddress("I")
	mint := testRecipientAddress("M")
	for _, fixture := range []struct {
		name            string
		actualRecipient string
		wantPolicy      string
		wantFinding     string
	}{
		{
			name:            "irrelevant_key_looks_similar",
			actualRecipient: "87654321" + strings.Repeat("R", 24) + "HGFEDCBA",
			wantPolicy:      "warn",
			wantFinding:     "none_observed_in_bounded_scope",
		},
		{
			name:            "actual_outbound_recipient_looks_similar",
			actualRecipient: testRecipientAddress("R"),
			wantPolicy:      "block",
			wantFinding:     "suspected_lookalike",
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var rpc struct {
					Method string `json:"method"`
				}
				if err := json.NewDecoder(r.Body).Decode(&rpc); err != nil {
					t.Errorf("decode RPC: %v", err)
					http.Error(w, "invalid fixture RPC", http.StatusBadRequest)
					return
				}
				var result any
				switch rpc.Method {
				case "getSignaturesForAddress":
					result = []any{map[string]any{"signature": "fixture-recipient-sig", "slot": float64(123456)}}
				case "getTransaction":
					tx := recipientEvidenceTransaction(
						[]any{wallet, irrelevant, mint, fixture.actualRecipient},
						[]any{
							recipientEvidenceInstruction(addressPoisoningSystemProgramID, "transfer",
								map[string]any{"source": irrelevant, "destination": wallet, "lamports": float64(1)}),
							recipientEvidenceInstruction(addressPoisoningSystemProgramID, "transfer",
								map[string]any{"source": wallet, "destination": fixture.actualRecipient, "lamports": float64(300)}),
						},
						map[string]any{"err": nil},
					)
					tx["slot"] = float64(123456)
					result = tx
				default:
					t.Errorf("unexpected RPC method: %s", rpc.Method)
					http.Error(w, "unsupported fixture RPC method", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
			}))
			defer server.Close()
			t.Setenv("SOLANA_RPC_URL", server.URL)
			t.Setenv("ALCHEMY_API_KEY", "")
			payload, _ := json.Marshal(map[string]any{
				"network":   "solana-mainnet",
				"wallet":    wallet,
				"candidate": candidate,
				"limit":     1,
			})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/address-poisoning/check", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			(&Handler{}).AddressPoisoningCheck(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("unexpected HTTP result: %d %s", rec.Code, rec.Body.String())
			}
			var body struct {
				Policy                string                  `json:"policy"`
				FindingState          string                  `json:"finding_state"`
				EnforcementStatus     string                  `json:"enforcement_status"`
				ObservedOutboundCount int                     `json:"observed_outbound_recipient_count"`
				Matches               []addressPoisoningMatch `json:"matches"`
				RPCEvidence           map[string]any          `json:"rpc_evidence"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Policy != fixture.wantPolicy || body.FindingState != fixture.wantFinding ||
				body.EnforcementStatus != "not_connected" || body.ObservedOutboundCount != 1 {
				t.Fatalf("wrong RPC recipient assessment: %+v", body)
			}
			observed, ok := body.RPCEvidence["outbound_recipients"].([]any)
			if !ok || len(observed) != 1 || observed[0] != fixture.actualRecipient {
				t.Fatalf("only actual recipient may be observed: %#v", body.RPCEvidence)
			}
			if fixture.wantPolicy == "block" {
				if len(body.Matches) != 1 || body.Matches[0].EvidenceSource != "rpc_observed_wallet_outbound_transfer" {
					t.Fatalf("missing RPC match provenance: %+v", body.Matches)
				}
			} else if len(body.Matches) != 0 {
				t.Fatalf("unrelated keys or incoming dust produced false match: %+v", body.Matches)
			}
		})
	}
}
