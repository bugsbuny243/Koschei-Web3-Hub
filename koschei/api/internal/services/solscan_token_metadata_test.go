package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchSolscanTokenMetadata(t *testing.T) {
	const mint = "9cRCn9rGT8V2imeM2BaKs13yhMEais3ruM3rPvTGpump"
	const creator = "yHCxHBEaJW5tbndqC8JciSThr7U1cqLpdCsvHcx6PRe"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token/meta" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("address"); got != mint {
			t.Fatalf("unexpected address: %s", got)
		}
		if got := r.Header.Get("token"); got != "test-key" {
			t.Fatalf("unexpected token header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"address":"` + mint + `","name":"The Black Bull","symbol":"ANSEM","creator":"` + creator + `","create_tx":"create-signature","created_time":1784480000,"first_mint_tx":"first-mint-signature","first_mint_time":1784480001,"mint_authority":"","freeze_authority":"","onchain_extensions":{"token_2022":true}}}`))
	}))
	defer server.Close()

	t.Setenv("SOLSCAN_API_KEY", "test-key")
	t.Setenv("SOLSCAN_API_BASE_URL", server.URL)
	meta := FetchSolscanTokenMetadata(context.Background(), mint)
	if !meta.Available || meta.Status != "complete" {
		t.Fatalf("unexpected metadata status: %+v", meta)
	}
	if meta.Creator != creator || meta.CreateTransaction != "create-signature" {
		t.Fatalf("unexpected creator metadata: %+v", meta)
	}
	if meta.Name != "The Black Bull" || meta.Symbol != "ANSEM" {
		t.Fatalf("unexpected token identity: %+v", meta)
	}
}
