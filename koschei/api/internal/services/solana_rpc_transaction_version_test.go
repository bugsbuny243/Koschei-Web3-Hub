package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSolanaTransactionRequestsVersionOneSupport(t *testing.T) {
	configureSolanaTransactionSingleflightTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		params, ok := request.Params.([]any)
		if !ok || len(params) != 2 {
			t.Fatalf("getTransaction params=%#v", request.Params)
		}
		config, ok := params[1].(map[string]any)
		if !ok {
			t.Fatalf("getTransaction config=%#v", params[1])
		}
		if got := config["maxSupportedTransactionVersion"]; got != float64(solanaMaxSupportedTransactionVersion) {
			t.Fatalf("maxSupportedTransactionVersion=%v want=%d", got, solanaMaxSupportedTransactionVersion)
		}
		writeTransactionResult(w, "version-one-supported")
	}))
	defer server.Close()

	result, err := SolanaGetTransactionJSONParsed(context.Background(), server.URL, "version-one-signature")
	if err != nil {
		t.Fatal(err)
	}
	if result["signature"] != "version-one-supported" {
		t.Fatalf("unexpected transaction result: %#v", result)
	}
}
