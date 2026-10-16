package handlers

import (
	"strings"
	"testing"
	"time"
)

// WSC-16.5 / WSC-24: a valid signature over a declared UI origin alone
// cannot prove the origin supplied by the request transport.
func TestARVISSignedIntentMissingOriginNeverVerifies(t *testing.T) {
	now := time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC)
	input, signed := guardV3TestSignedIntent(t, now)

	for _, sourceOrigin := range []string{"", "null", "   ", "https://app.koschei.example/path", "javascript://app.koschei.example"} {
		t.Run(strings.ReplaceAll(sourceOrigin, "/", "_"), func(t *testing.T) {
			got, findings := evaluateTransactionGuardV3SignedIntent(input, &signed, "tx-fingerprint", sourceOrigin, now.Add(time.Minute), true)
			if !got.SignatureVerified || !got.FingerprintMatched || !got.PolicyMatched {
				t.Fatalf("fixture must preserve independent valid cryptographic checks: %+v", got)
			}
			if got.Verified || got.Complete || got.OriginMatched ||
				got.Status != "origin_evidence_unavailable" || got.OriginEvidenceStatus != "unavailable" {
				t.Fatalf("missing origin improperly promoted to verified signing safety: %+v", got)
			}
			if !guardV3TestHasFinding(findings, "signed_ui_intent_origin_unavailable") ||
				guardV3TestHasFinding(findings, "signed_ui_intent_verified") {
				t.Fatalf("missing signed-intent origin finding: %+v", findings)
			}
		})
	}
}

func TestARVISSignedIntentExplicitOriginMismatchIsNotVerified(t *testing.T) {
	now := time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC)
	input, signed := guardV3TestSignedIntent(t, now)
	got, findings := evaluateTransactionGuardV3SignedIntent(input, &signed, "tx-fingerprint", "https://app.koschei.example.evil.invalid", now.Add(time.Minute), true)
	if got.Verified || got.Complete || got.OriginMatched || got.OriginEvidenceStatus != "mismatch" || got.Status != "signed_ui_intent_mismatch" {
		t.Fatalf("phishing lookalike origin accepted: %+v", got)
	}
	if !guardV3TestHasFinding(findings, "signed_ui_intent_origin_mismatch") {
		t.Fatalf("origin mismatch finding absent: %+v", findings)
	}
}

func TestARVISSignedIntentCanonicalOriginStillVerifies(t *testing.T) {
	now := time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC)
	input, signed := guardV3TestSignedIntent(t, now)
	got, findings := evaluateTransactionGuardV3SignedIntent(input, &signed, "tx-fingerprint", "HTTPS://APP.KOSCHEI.EXAMPLE/", now.Add(time.Minute), true)
	if !got.Verified || !got.Complete || !got.OriginMatched ||
		got.OriginEvidenceStatus != "matched" || got.Status != "verified_signed_ui_intent" {
		t.Fatalf("matching valid signed origin incorrectly rejected: %+v %+v", got, findings)
	}
	if !guardV3TestHasFinding(findings, "signed_ui_intent_verified") {
		t.Fatalf("verified origin evidence finding absent: %+v", findings)
	}
}

func TestARVISOriginCanonicalizationRejectsAmbiguousClaimLinks(t *testing.T) {
	for _, tc := range []struct {
		name, supplied, canonical string
	}{
		{"valid_https", "HTTPS://APP.KOSCHEI.EXAMPLE/", "https://app.koschei.example"},
		{"valid_custom_port", "https://app.koschei.example:8443", "https://app.koschei.example:8443"},
		{"missing", "", ""},
		{"null", "null", ""},
		{"unsupported_protocol", "javascript://app.koschei.example", ""},
		{"no_scheme", "app.koschei.example", ""},
		{"userinfo", "https://app.koschei.example@attacker.invalid", ""},
		{"embedded_credentials", "https://user:pass@app.koschei.example", ""},
		{"path", "https://app.koschei.example/claim", ""},
		{"query", "https://app.koschei.example?claim=1", ""},
		{"empty_query", "https://app.koschei.example?", ""},
		{"fragment", "https://app.koschei.example#claim", ""},
		{"invalid_port", "https://app.koschei.example:99999", ""},
		{"bad_host_label", "https://app..koschei.example", ""},
		{"trailing_dns_dot", "https://app.koschei.example.", ""},
		{"whitespace", "https://app.koschei.example https://fake.invalid", ""},
		{"backslash", "https:\\\\app.koschei.example", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeTransactionGuardV3Origin(tc.supplied); got != tc.canonical {
				t.Fatalf("origin %q normalized to %q, want %q", tc.supplied, got, tc.canonical)
			}
		})
	}
}
