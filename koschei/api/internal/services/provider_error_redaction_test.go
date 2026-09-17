package services

import (
	"errors"
	"strings"
	"testing"
)

func TestSafeProviderErrorRedactsHeliusQueryKey(t *testing.T) {
	const value = "testvalue123"
	err := errors.New(`Post "https://mainnet.helius-rpc.com/?api-key=` + value + `": solana rpc provider cooling down`)
	got := SafeProviderError(err)
	if strings.Contains(got, value) {
		t.Fatalf("provider error leaked query credential: %q", got)
	}
	if !strings.Contains(got, "api-key=[redacted]") {
		t.Fatalf("provider error missing redaction marker: %q", got)
	}
}

func TestRedactProviderCredentialsCoversCommonForms(t *testing.T) {
	const value = "testvalue123"
	cases := []string{
		"https://rpc.example/?api_key=" + value,
		"https://rpc.example/?access_token=" + value + "&x=1",
		"https://rpc.example/?token=" + value,
		"Authorization: Bearer " + value,
		"HELIUS_API_KEY=" + value,
	}
	for _, input := range cases {
		got := RedactProviderCredentials(input)
		if strings.Contains(got, value) {
			t.Fatalf("redaction leaked credential for %q: %q", input, got)
		}
	}
}
