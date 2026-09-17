package services

import (
	"errors"
	"strings"
	"testing"
)

func TestSafeProviderErrorRedactsHeliusQueryKey(t *testing.T) {
	const secret = "example-helius-secret-1234567890"
	err := errors.New(`Post "https://mainnet.helius-rpc.com/?api-key=` + secret + `": solana rpc provider cooling down`)
	got := SafeProviderError(err)
	if strings.Contains(got, secret) {
		t.Fatalf("provider error leaked Helius key: %q", got)
	}
	if !strings.Contains(got, "api-key=[redacted]") {
		t.Fatalf("provider error missing redaction marker: %q", got)
	}
}

func TestRedactProviderCredentialsCoversCommonSecretForms(t *testing.T) {
	const secret = "super-secret-value"
	cases := []string{
		"https://rpc.example/?api_key=" + secret,
		"https://rpc.example/?access_token=" + secret + "&x=1",
		"https://rpc.example/?token=" + secret,
		"Authorization: Bearer " + secret,
		"HELIUS_API_KEY=" + secret,
	}
	for _, input := range cases {
		got := RedactProviderCredentials(input)
		if strings.Contains(got, secret) {
			t.Fatalf("redaction leaked secret for %q: %q", input, got)
		}
	}
}
