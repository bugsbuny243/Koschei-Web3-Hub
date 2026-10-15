package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// These frozen owner/type observations use the same account owner and parsed
// account classification returned for the PYUSD mint by finalized Solana RPC.
// The test server is not a claim of fresh onchain evidence.
func TestUnifiedMintOwnerCollectorRPCResponses(t *testing.T) {
	const pyusd = "2b1kV6DkPAnxd5ixfnxCpjxmKwqjjaYmCZfHsFu24GXo"
	const token2022 = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
	const classic = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	tests := []struct {
		name           string
		value          string
		httpStatus     int
		expectedStatus string
		classified     bool
		token2022      bool
	}{
		{name: "observed_pyusd_token_2022", value: `{"owner":"` + token2022 + `","data":{"parsed":{"type":"mint"}}}`, expectedStatus: "verified", classified: true, token2022: true},
		{name: "classic_spl_mint", value: `{"owner":"` + classic + `","data":{"parsed":{"type":"mint"}}}`, expectedStatus: "verified", classified: true},
		{name: "unrecognized_program", value: `{"owner":"OtherProgram11111111111111111111111111111","data":{"parsed":{"type":"mint"}}}`, expectedStatus: "unverified"},
		{name: "non_mint_token_account", value: `{"owner":"` + token2022 + `","data":{"parsed":{"type":"account"}}}`, expectedStatus: "unverified"},
		{name: "missing_account", value: `null`, expectedStatus: "unverified"},
		{name: "provider_forbidden", httpStatus: http.StatusForbidden, expectedStatus: "unverified"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodPost {
					t.Errorf("unexpected RPC method: %s", r.Method)
				}
				if tc.httpStatus != 0 {
					w.WriteHeader(tc.httpStatus)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"context":{"slot":456947593},"value":%s}}`, tc.value)
			}))
			defer server.Close()
			t.Setenv("SOLANA_RPC_URL", server.URL)
			got := (&Handler{}).collectUnifiedMintOwnerEvidence(context.Background(), pyusd, "solana-mainnet")
			if got["mint_owner_program_evidence_status"] != tc.expectedStatus {
				t.Fatalf("wrong evidence status: %v", got)
			}
			classification, exists := got["token_2022"]
			if exists != tc.classified || (exists && classification != tc.token2022) {
				t.Fatalf("unsupported classification: %v", got)
			}
			_, ownerExists := got["mint_owner_program"]
			if ownerExists != tc.classified {
				t.Fatalf("owner marked verified without recognized mint: %v", got)
			}
			if calls.Load() != 1 {
				t.Fatalf("expected exactly one RPC read, got %d", calls.Load())
			}
		})
	}
}

func TestUnifiedMintOwnerRejectsNonSolanaAndEmptyTargetWithoutRPC(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv("SOLANA_RPC_URL", server.URL)
	h := &Handler{}
	for _, request := range []struct{ target, network string }{
		{"2b1kV6DkPAnxd5ixfnxCpjxmKwqjjaYmCZfHsFu24GXo", "ethereum-mainnet"},
		{"", "solana-mainnet"},
	} {
		got := h.collectUnifiedMintOwnerEvidence(context.Background(), request.target, request.network)
		if got["mint_owner_program_evidence_status"] != "unverified" {
			t.Fatalf("non-applicable target was verified: %v", got)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("unsupported inputs made %d RPC requests", calls.Load())
	}
}
