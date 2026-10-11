package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSolanaTransactionRequestsOnlySupportedVersion(t *testing.T) {
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
		if got := config["maxSupportedTransactionVersion"]; got != float64(SolanaMaxSupportedTransactionVersion) {
			t.Fatalf("maxSupportedTransactionVersion=%v want=%d", got, SolanaMaxSupportedTransactionVersion)
		}
		writeTransactionResult(w, "supported-version")
	}))
	defer server.Close()

	result, err := SolanaGetTransactionJSONParsed(context.Background(), server.URL, "supported-version-signature")
	if err != nil {
		t.Fatal(err)
	}
	if result["signature"] != "supported-version" {
		t.Fatalf("unexpected transaction result: %#v", result)
	}
}
