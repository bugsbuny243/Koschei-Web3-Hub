package services

import (
	"strings"
	"testing"
	"time"
)

func claimTermsInclude(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestClaimQueryEncodedSecretAndSignatureHintsRemainStructural(t *testing.T) {
	evidence := parseArvisClaimSurface("https://example.org/claim?payload=%73eedphrase&flow=%63onnectwallet")
	if !evidence.Available || !claimTermsInclude(evidence.SecretTerms, "seedphrase") ||
		!claimTermsInclude(evidence.SigningTerms, "connectwallet") {
		t.Fatalf("decoded query indications missing: %#v", evidence)
	}
	req := SecurityRadarRequest{Target: evidence.Original, Network: "solana-mainnet"}
	arm := buildWalletlessClaimArm(req, evidence, time.Now().UTC().Format(time.RFC3339))
	if arm.RiskIndex != 0 || arm.Grade != "-" {
		t.Fatalf("decoded URL query cannot issue a risk grade: %#v", arm)
	}
	for _, key := range []string{"origin_identity_verified", "signed_transaction_decoded",
		"contract_or_program_checked", "remote_content_fetched", "wallet_connection_executed"} {
		if arm.Signals[key] != false {
			t.Fatalf("%s must be false with URL-only evidence: %#v", key, arm.Signals)
		}
	}
	if arm.Signals["analysis_scope"] != "user_supplied_url_structure_only" ||
		arm.Signals["decoded_query_depth"] != 1 {
		t.Fatalf("URL observation scope must remain explicit: %#v", arm.Signals)
	}
}

func TestClaimRedirectQuerySeparatesCrossOriginFromSameOrigin(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		host        string
		parsed      int
		crossOrigin int
		nonHTTP     int
	}{
		{
			name: "percent_encoded_cross_origin",
			input: "https://example.org/claim?next=https%3A%2F%2Fwallet.example.net%2Fconnect",
			host: "wallet.example.net", parsed: 1, crossOrigin: 1,
		},
		{
			name: "same_origin_default_port",
			input: "https://example.org/claim?return_url=https%3A%2F%2Fexample.org%3A443%2Fdone",
			host: "example.org", parsed: 1,
		},
		{
			name: "different_port",
			input: "https://example.org/claim?callback=https%3A%2F%2Fexample.org%3A8443%2Fdone",
			host: "example.org", parsed: 1, crossOrigin: 1,
		},
		{
			name: "scheme_relative_redirect",
			input: "https://example.org/claim?redirect=%2F%2Felsewhere.example%2Fstart",
			host: "elsewhere.example", parsed: 1, crossOrigin: 1,
		},
		{
			name: "non_http_scheme",
			input: "https://example.org/claim?next=javascript%3Aalert%281%29",
			nonHTTP: 1,
		},
		{
			name: "relative_path_not_an_origin",
			input: "https://example.org/claim?next=%2Fwelcome",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := parseArvisClaimSurface(test.input)
			if !e.Available || e.ParsedRedirectCandidates != test.parsed ||
				e.CrossOriginRedirectCandidates != test.crossOrigin ||
				e.NonHTTPSchemeRedirectCandidates != test.nonHTTP {
				t.Fatalf("wrong parsed redirect facts: %#v", e)
			}
			if test.host == "" && len(e.RedirectHosts) != 0 {
				t.Fatalf("unexpected redirect host: %#v", e.RedirectHosts)
			}
			if test.host != "" && (len(e.RedirectHosts) != 1 || e.RedirectHosts[0] != test.host) {
				t.Fatalf("missing URL host: %#v", e.RedirectHosts)
			}
			req := SecurityRadarRequest{Target: e.Original, Network: "solana-mainnet"}
			arm := buildClaimSurfaceArm(req, e, time.Now().UTC().Format(time.RFC3339))
			if arm.RiskIndex != 0 || arm.Grade != "-" ||
				arm.Signals["redirect_target_fetched"] != false ||
				arm.Signals["redirect_destination_verified"] != false {
				t.Fatalf("URL-only redirect hints must not claim verification or score: %#v", arm)
			}
			if arm.Signals["cross_origin_redirect_candidate_count"] != test.crossOrigin {
				t.Fatalf("cross-origin evidence not surfaced: %#v", arm.Signals)
			}
		})
	}
}

func TestClaimURLParserRejectsOversizedInputBeforeQueryInspection(t *testing.T) {
	oversized := "https://example.org/claim?payload=" + strings.Repeat("A", maxArvisClaimURLBytes)
	if result := parseArvisClaimSurface(oversized); result.Available {
		t.Fatalf("oversized URL should remain unavailable, got %#v", result)
	}
}
