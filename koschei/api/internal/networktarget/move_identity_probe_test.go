package networktarget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMoveCanonicalAddressResolutionIsNetworkScopedAndStrict(t *testing.T) {
	address := "0x" + strings.Repeat("ab", 32)
	sui, err := Resolve("sui-mainnet", address)
	if err != nil {
		t.Fatal(err)
	}
	aptos, err := Resolve("aptos-mainnet", address)
	if err != nil {
		t.Fatal(err)
	}
	if sui.SubjectID == aptos.SubjectID || sui.CanonicalRef == aptos.CanonicalRef {
		t.Fatal("same Move address was conflated across Sui and Aptos")
	}
	if sui.Classification != "move_hex_32_canonical_syntax_only" || aptos.Classification != "move_hex_32_canonical_syntax_only" {
		t.Fatalf("unexpected Move classification: sui=%#v aptos=%#v", sui, aptos)
	}
	upper, err := Resolve("sui-mainnet", "0x"+strings.Repeat("AB", 32))
	if err != nil || upper.SubjectID != sui.SubjectID {
		t.Fatal("Move hex case normalization changed Sui identity")
	}
	for _, bad := range []string{"0x1", "0x" + strings.Repeat("ab", 20), strings.Repeat("ab", 32)} {
		if _, err := Resolve("sui-mainnet", bad); err == nil {
			t.Fatalf("non-canonical Move address accepted: %q", bad)
		}
	}
}

func TestProbeSuiMainnetIdentity(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["query"] != "{ chainIdentifier }" {
			t.Fatalf("query=%q", body["query"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"chainIdentifier": suiMainnetChainIdentifierBase58},
		})
	}))
	defer server.Close()

	got, err := ProbeSuiMainnetIdentity(context.Background(), server.Client(), server.URL, time.Date(2026, 9, 23, 6, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got.ChainIdentifier != suiMainnetChainIdentifierBase58 || got.Observation.Network.ID != "sui-mainnet" {
		t.Fatalf("unexpected Sui identity: %#v", got)
	}
	if got.Observation.StakeSharePct != nil || got.EndpointScope != "sui_graphql_chain_identity_only" {
		t.Fatalf("Sui identity probe overclaimed evidence: %#v", got)
	}
}

func TestProbeSuiMainnetIdentityRejectsWrongChain(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"chainIdentifier": "69WiPg3DAQiwdxfncX6wYQ2siKwAe6L9BZthQea3JNMD"},
		})
	}))
	defer server.Close()

	if _, err := ProbeSuiMainnetIdentity(context.Background(), server.Client(), server.URL, time.Now().UTC()); err == nil {
		t.Fatal("non-mainnet Sui identity was accepted")
	}
}

func TestProbeAptosMainnetIdentity(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method=%s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"chain_id":         1,
			"epoch":            "123",
			"ledger_version":   "456789",
			"ledger_timestamp": "1700000000000000",
			"block_height":     "98765",
			"node_role":        "full_node",
			"git_hash":         "abcdef",
		})
	}))
	defer server.Close()

	got, err := ProbeAptosMainnetIdentity(context.Background(), server.Client(), server.URL, time.Date(2026, 9, 23, 6, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got.ChainID != 1 || got.Epoch != 123 || got.LedgerVersion != 456789 || got.BlockHeight != 98765 {
		t.Fatalf("unexpected Aptos identity: %#v", got)
	}
	if got.Observation.Network.ID != "aptos-mainnet" || got.Observation.StakeSharePct != nil {
		t.Fatalf("Aptos identity probe overclaimed evidence: %#v", got)
	}
}

func TestProbeAptosMainnetIdentityRejectsWrongChainAndBadLedger(t *testing.T) {
	cases := []map[string]any{
		{
			"chain_id": 2, "epoch": "1", "ledger_version": "2", "ledger_timestamp": "3", "block_height": "4", "node_role": "full_node",
		},
		{
			"chain_id": 1, "epoch": "bad", "ledger_version": "2", "ledger_timestamp": "3", "block_height": "4", "node_role": "full_node",
		},
	}
	for i, payload := range cases {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(payload)
		}))
		_, err := ProbeAptosMainnetIdentity(context.Background(), server.Client(), server.URL, time.Now().UTC())
		server.Close()
		if err == nil {
			t.Fatalf("case %d unexpectedly accepted", i)
		}
	}
}

func TestMoveIdentityProbesRejectNonHTTPS(t *testing.T) {
	now := time.Now().UTC()
	if _, err := ProbeSuiMainnetIdentity(context.Background(), http.DefaultClient, "http://sui.example/graphql", now); err == nil {
		t.Fatal("non-HTTPS Sui endpoint accepted")
	}
	if _, err := ProbeAptosMainnetIdentity(context.Background(), http.DefaultClient, "http://aptos.example/v1", now); err == nil {
		t.Fatal("non-HTTPS Aptos endpoint accepted")
	}
}
