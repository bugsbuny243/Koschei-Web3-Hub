package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSolscanActorDiscoveryRequiresConfiguration(t *testing.T) {
	client := &SolscanClient{}
	out := client.DiscoverActor(context.Background(), "Wallet111111111111111111111111111111111", 40)
	if out.Configured {
		t.Fatal("client without API key reported configured")
	}
	if out.Status != "not_configured" {
		t.Fatalf("unexpected status: %s", out.Status)
	}
	if out.Available {
		t.Fatal("unconfigured discovery reported available")
	}
}

func TestSolscanActorDiscoveryCollectsObservedAttributionWithoutPromotingVerification(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("token") != "test-key" {
			t.Fatalf("missing Solscan token header")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/account/metadata":
			_, _ = w.Write([]byte(`{"success":true,"data":{"account_address":"Creator111111111111111111111111111111111","account_label":"Example Hot Wallet","account_tags":["exchange_wallet"],"account_type":"address","active_age":84,"funded_by":{"funded_by":"Funder1111111111111111111111111111111111","tx_hash":"Sig111111111111111111111111111111111111111111111111111111111111","block_time":1700000000}}}`))
		case "/account/transactions":
			if r.URL.Query().Get("limit") != "40" {
				t.Fatalf("unexpected transaction limit: %s", r.URL.Query().Get("limit"))
			}
			_, _ = w.Write([]byte(`{"success":true,"data":[{"slot":123,"fee":5000,"status":"Success","signer":"Creator111111111111111111111111111111111","block_time":1700000001,"tx_hash":"Tx111111111111111111111111111111111111111111111111111111111111","program_ids":["Program111"],"parsed_instructions":[]}]}`))
		case "/account/token-accounts":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"token_account":"ATA11111111111111111111111111111111111111","token_address":"Mint1111111111111111111111111111111111111","amount":650000000,"amount_str":"650000000","token_decimals":6,"owner":"Creator111111111111111111111111111111111"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &SolscanClient{APIKey: "test-key", BaseURL: server.URL, Client: server.Client()}
	out := client.DiscoverActor(context.Background(), "Creator111111111111111111111111111111111", 99)
	if out.Status != "complete" || !out.Available {
		t.Fatalf("unexpected discovery result: %#v", out)
	}
	if out.Metadata.Label != "Example Hot Wallet" {
		t.Fatalf("metadata label missing: %#v", out.Metadata)
	}
	if out.Metadata.FundedBy.Wallet != "Funder1111111111111111111111111111111111" {
		t.Fatalf("funder missing: %#v", out.Metadata.FundedBy)
	}
	if len(out.TransactionCandidates) != 1 || len(out.TokenAccounts) != 1 {
		t.Fatalf("discovery coverage missing: tx=%d token_accounts=%d", len(out.TransactionCandidates), len(out.TokenAccounts))
	}

	evidence := SolscanActorDiscoveryEvidence(out, "solana-mainnet")
	if len(evidence) != 3 {
		t.Fatalf("expected attribution, funding and token-account evidence; got %d", len(evidence))
	}
	for _, item := range evidence {
		if item.VerificationStatus != "observed" {
			t.Fatalf("Solscan discovery promoted evidence without RPC verification: %#v", item)
		}
	}
	if evidence[1].Signature == "" {
		t.Fatal("external funding candidate lost its verification signature")
	}
	wantObservedAt := time.Unix(1700000000, 0).UTC()
	if !evidence[1].ObservedAt.Equal(wantObservedAt) {
		t.Fatalf("funding observation time mismatch: got %s want %s", evidence[1].ObservedAt, wantObservedAt)
	}
}
