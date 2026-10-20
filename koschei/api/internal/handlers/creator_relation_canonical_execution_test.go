package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCanonicalCreatorVerificationFailsClosedForExecutionAndMintMismatch(t *testing.T) {
	const creator = "CreatorCanonicalProof"
	const target = "MintCanonicalProof"

	cases := []struct {
		name          string
		hasMetaErr    bool
		metaErr       any
		instruction   string
		initMint      string
		wantStatus    string
	}{
		{
			name: "failed on-chain transaction", hasMetaErr: true,
			metaErr: map[string]any{"InstructionError": []any{0, "Custom"}},
			instruction: "initializeMint2", initMint: target,
			wantStatus: "canonical_transaction_failed",
		},
		{
			name: "missing execution status", hasMetaErr: false,
			instruction: "initializeMint2", initMint: target,
			wantStatus: "canonical_execution_status_unavailable",
		},
		{
			name: "different mint initialized, target merely referenced",
			hasMetaErr: true, instruction: "initializeMint2", initMint: "OtherMint",
			wantStatus: "mint_not_referenced",
		},
		{
			name: "target only transferred during another launch",
			hasMetaErr: true, instruction: "transferChecked", initMint: target,
			wantStatus: "mint_not_referenced",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meta := map[string]any{
				"postTokenBalances": []any{map[string]any{"mint": target, "owner": creator}},
				"logMessages": []any{"Program log: Instruction: Create", "Program log: Instruction: InitializeMint2"},
			}
			if tc.hasMetaErr {
				meta["err"] = tc.metaErr
			}
			result := map[string]any{
				"slot": 12345,
				"transaction": map[string]any{
					"message": map[string]any{
						"accountKeys": []any{
							map[string]any{"pubkey": creator, "signer": true},
							map[string]any{"pubkey": target, "signer": false},
							map[string]any{"pubkey": "OtherMint", "signer": false},
						},
						"instructions": []any{
							map[string]any{"parsed": map[string]any{
								"type": tc.instruction,
								"info": map[string]any{"mint": tc.initMint},
							}},
						},
					},
				},
				"meta": meta,
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0", "id": 1, "result": result,
				})
			}))
			defer server.Close()
			t.Setenv("SOLANA_RPC_URL", server.URL)
			t.Setenv("ALCHEMY_API_KEY", "")

			verification := (&Handler{}).verifyCanonicalCreatorRelation(
				t.Context(), target, "solana-mainnet", creator, "CanonicalSignature",
			)
			if verification.Verified || verification.Status != tc.wantStatus {
				t.Fatalf("verification=%#v; wanted unverified status %q", verification, tc.wantStatus)
			}
			claim := applyCanonicalCreatorVerification(map[string]any{
				"creator_wallet": creator, "creator_relation_verified": true,
			}, verification)
			if claim["creator_relation_verified"] != false {
				t.Fatalf("untrusted creator claim was not downgraded: %#v", claim)
			}
		})
	}
}

func TestCanonicalCreatorTargetMintInitializedAcceptsMatchingInnerInstruction(t *testing.T) {
	message := map[string]any{"instructions": []any{}}
	meta := map[string]any{
		"innerInstructions": []any{map[string]any{
			"instructions": []any{map[string]any{
				"parsed": map[string]any{
					"type": "initializeMint2",
					"info": map[string]any{"mint": "MintInner"},
				},
			}},
		}},
	}
	if !canonicalCreatorTargetMintInitialized(message, meta, "MintInner") {
		t.Fatal("a matching successful inner initializeMint2 should be recognized")
	}
	if canonicalCreatorTargetMintInitialized(message, meta, "UnrelatedMint") {
		t.Fatal("a different target mint must not inherit the inner initialization")
	}
}
