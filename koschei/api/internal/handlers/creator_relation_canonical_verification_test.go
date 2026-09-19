package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyCanonicalCreatorRelationRequiresSignerMintAndLaunchSemantics(t *testing.T) {
	const mint = "MintCanonical111"
	const creator = "CreatorCanonical111"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Method != "getTransaction" {
			http.Error(w, "unexpected method", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"slot":555123,"blockTime":1783364366,"transaction":{"message":{"accountKeys":[{"pubkey":"CreatorCanonical111","signer":true,"writable":true},{"pubkey":"MintCanonical111","signer":false,"writable":true}],"instructions":[{"program":"spl-token-2022","parsed":{"type":"initializeMint2","info":{"mint":"MintCanonical111"}}}]}},"meta":{"err":null,"preTokenBalances":[],"postTokenBalances":[{"mint":"MintCanonical111","owner":"CreatorCanonical111"}],"logMessages":["Program log: Instruction: Create","Program log: Instruction: InitializeMint2"]}}}`))
	}))
	defer server.Close()

	t.Setenv("SOLANA_RPC_URL", server.URL)
	t.Setenv("ALCHEMY_API_KEY", "legacy-key-must-not-win")
	h := &Handler{}
	got := h.verifyCanonicalCreatorRelation(t.Context(), mint, "solana-mainnet", creator, "Signature111")
	if !got.Verified || got.Status != "verified_canonical_create_transaction" {
		t.Fatalf("expected canonical verification, got %#v", got)
	}
	if got.Slot != 555123 || !got.CreatorSigner || !got.MintReferenced || !got.LaunchLike {
		t.Fatalf("canonical proof fields missing: %#v", got)
	}

	upgraded := applyCanonicalCreatorVerification(map[string]any{
		"creator_wallet": creator,
		"source":         "helius_das_and_rpc",
	}, got)
	if upgraded["creator_relation_verified"] != true || upgraded["slot"] != int64(555123) {
		t.Fatalf("source context was not upgraded: %#v", upgraded)
	}
	if upgraded["source"] != "solana_rpc_create_transaction" {
		t.Fatalf("verified provenance must become canonical Solana RPC, got %#v", upgraded["source"])
	}
}

func TestVerifyCanonicalCreatorRelationRejectsNonSigner(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"slot":99,"blockTime":1783364366,"transaction":{"message":{"accountKeys":[{"pubkey":"CreatorNoSign","signer":false,"writable":true},{"pubkey":"MintNoSign","signer":false,"writable":true}],"instructions":[{"parsed":{"type":"initializeMint2","info":{"mint":"MintNoSign"}}}]}},"meta":{"err":null,"logMessages":["Program log: Instruction: Create"]}}}`))
	}))
	defer server.Close()
	t.Setenv("SOLANA_RPC_URL", server.URL)
	t.Setenv("ALCHEMY_API_KEY", "")

	got := (&Handler{}).verifyCanonicalCreatorRelation(t.Context(), "MintNoSign", "solana-mainnet", "CreatorNoSign", "SigNoSign")
	if got.Verified || got.Status != "creator_not_signer" {
		t.Fatalf("non-signer must not be upgraded: %#v", got)
	}
}

func TestVerifyCanonicalCreatorRelationRejectsWrongMint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"slot":100,"blockTime":1783364366,"transaction":{"message":{"accountKeys":[{"pubkey":"CreatorWrongMint","signer":true,"writable":true},{"pubkey":"DifferentMint","signer":false,"writable":true}],"instructions":[{"parsed":{"type":"initializeMint2","info":{"mint":"DifferentMint"}}}]}},"meta":{"err":null,"logMessages":["Program log: Instruction: Create"]}}}`))
	}))
	defer server.Close()
	t.Setenv("SOLANA_RPC_URL", server.URL)
	t.Setenv("ALCHEMY_API_KEY", "")

	got := (&Handler{}).verifyCanonicalCreatorRelation(t.Context(), "ExpectedMint", "solana-mainnet", "CreatorWrongMint", "SigWrongMint")
	if got.Verified || got.Status != "mint_not_referenced" {
		t.Fatalf("wrong mint must not be upgraded: %#v", got)
	}
}

func TestVerifyCanonicalCreatorRelationCandidatesRecoversFromRejectedFirstSignature(t *testing.T) {
	const mint = "MintCandidate111"
	const creator = "CreatorCandidate111"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Method != "getTransaction" || len(request.Params) == 0 {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		var signature string
		if err := json.Unmarshal(request.Params[0], &signature); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if signature == "RejectedCandidate" {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"Invalid param: Invalid"}}`))
			return
		}
		if signature != "VerifiedCandidate" {
			http.Error(w, "unexpected signature", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"slot":777123,"blockTime":1783364366,"transaction":{"message":{"accountKeys":[{"pubkey":"CreatorCandidate111","signer":true,"writable":true},{"pubkey":"MintCandidate111","signer":false,"writable":true}],"instructions":[{"program":"spl-token","parsed":{"type":"initializeMint2","info":{"mint":"MintCandidate111"}}}]}},"meta":{"err":null,"preTokenBalances":[],"postTokenBalances":[{"mint":"MintCandidate111","owner":"CreatorCandidate111"}],"logMessages":["Program log: Instruction: Create","Program log: Instruction: InitializeMint2"]}}}`))
	}))
	defer server.Close()

	t.Setenv("SOLANA_RPC_URL", server.URL)
	t.Setenv("ALCHEMY_API_KEY", "")
	got := (&Handler{}).verifyCanonicalCreatorRelationCandidates(
		t.Context(),
		mint,
		"solana-mainnet",
		creator,
		[]string{"RejectedCandidate", "VerifiedCandidate"},
	)
	if !got.Verified || got.Status != "verified_canonical_create_transaction" {
		t.Fatalf("second candidate was not canonically verified: %#v", got)
	}
	if got.Signature != "VerifiedCandidate" || got.Slot != 777123 {
		t.Fatalf("unexpected recovered canonical proof: %#v", got)
	}
}

func TestCanonicalCreatorSignatureCandidatesPrioritizeCreationAndDeduplicate(t *testing.T) {
	got := canonicalCreatorSignatureCandidates(map[string]any{
		"creation_signature":   "CreateSig",
		"launch_signature":     "CreateSig",
		"first_mint_signature": "FirstMintSig",
		"signature":            "GenericSig",
	})
	want := []string{"CreateSig", "FirstMintSig", "GenericSig"}
	if len(got) != len(want) {
		t.Fatalf("candidates=%v want=%v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("candidate[%d]=%q want=%q; all=%v", index, got[index], want[index], got)
		}
	}
}
