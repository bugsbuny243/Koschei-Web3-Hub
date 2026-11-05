package web3

import (
	"context"
	"fmt"
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"
)

func TestRPCProviderHostDropsPathQueryAndCredentials(t *testing.T) {
	got := RPCProviderHost("https://user:secret@mainnet.helius-rpc.com/v0/key?api-key=hidden")
	if got != "mainnet.helius-rpc.com" {
		t.Fatalf("host=%q", got)
	}
	if RPCHTTPStatusClass(429) != "4xx" || RPCHTTPStatusClass(503) != "5xx" || RPCHTTPStatusClass(0) != "none" {
		t.Fatalf("unexpected status classes")
	}
}

func TestLogRPCFailureNeverLeaksAPIKey(t *testing.T) {
	const secret = "super-secret-api-key"
	var out bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&out)
	defer log.SetOutput(previous)

	endpoint := "https://solana-mainnet.g.alchemy.com/v2/" + secret + "?api_key=" + secret
	LogRPCFailure("getTokenSupply", endpoint, 429, errors.New(`Post "`+endpoint+`": quota exhausted`))
	text := out.String()
	if strings.Contains(text, secret) {
		t.Fatalf("secret leaked: %s", text)
	}
	for _, expected := range []string{"provider=solana-mainnet.g.alchemy.com", "http_class=4xx", "status=429", "method=getTokenSupply"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q: %s", expected, text)
		}
	}
}

func TestRPCFailureKindDistinguishesCancellationAndProviderFailure(t *testing.T) {
	cases := []struct {
		status int
		err error
		want string
	}{
		{200, fmt.Errorf("wrapped: %w", context.Canceled), "canceled"},
		{429, context.Canceled, "canceled"},
		{429, errors.New("rate limited"), "throttled"},
		{200, context.DeadlineExceeded, "timeout"},
		{503, errors.New("unavailable"), "upstream_5xx"},
		{403, errors.New("forbidden"), "upstream_4xx"},
		{200, errors.New("invalid response"), "rpc_error"},
	}
	for _, tc := range cases {
		if got := RPCFailureKind(tc.status, tc.err); got != tc.want {
			t.Errorf("status=%d err=%v: got %q want %q", tc.status, tc.err, got, tc.want)
		}
	}
}

func TestCanceledRPCLogIsNotProviderFailure(t *testing.T) {
	var out bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&out)
	defer log.SetOutput(previous)
	LogRPCFailure("getSignaturesForAddress", "https://example.org/private-key", 200, context.Canceled)
	message := out.String()
	if !strings.Contains(message, "solana rpc canceled kind=canceled") || strings.Contains(message, "solana rpc failure") {
		t.Fatalf("canceled request mislabeled as provider failure: %s", message)
	}
}
