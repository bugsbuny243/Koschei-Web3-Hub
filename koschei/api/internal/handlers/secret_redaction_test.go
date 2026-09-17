package handlers

import (
	"errors"
	"strings"
	"testing"
)

func TestCreatorIntelCompactErrorRedactsProviderSecrets(t *testing.T) {
	const secret = "example-helius-secret-1234567890"
	err := errors.New(`Post "https://mainnet.helius-rpc.com/?api-key=` + secret + `": solana rpc provider cooling down`)
	got := creatorIntelCompactError(err)
	if strings.Contains(got, secret) {
		t.Fatalf("compact error leaked provider secret: %q", got)
	}
	if !strings.Contains(got, "api-key=[REDACTED]") {
		t.Fatalf("compact error did not preserve redacted query marker: %q", got)
	}
}

func TestRedactSensitiveErrorTextCoversCommonSecretForms(t *testing.T) {
	const secret = "super-secret-value"
	cases := []string{
		"https://rpc.example/?api_key=" + secret,
		"https://rpc.example/?token=" + secret + "&x=1",
		"Authorization: Bearer " + secret,
		"HELIUS_API_KEY=" + secret,
	}
	for _, input := range cases {
		got := redactSensitiveErrorText(input)
		if strings.Contains(got, secret) {
			t.Fatalf("redaction leaked secret for %q: %q", input, got)
		}
	}
}
