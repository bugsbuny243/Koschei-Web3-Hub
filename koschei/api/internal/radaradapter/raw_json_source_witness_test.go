package radaradapter

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func rawJSONWitnessFixture() (SubjectIdentity, RawJSONObservation) {
	subject := SubjectIdentity{
		Network:       "solana-mainnet",
		IdentityKind:  "solana_mint",
		IdentityValue: "Mint1111111111111111111111111111111111",
		NativeID:      "Mint1111111111111111111111111111111111",
	}
	source := []byte(`{"result":{"context":{"slot":123},"value":"test"},"private_token":"fixture-internal-secret"}`)
	digest := sha256.Sum256(source)
	return subject, RawJSONObservation{
		Row: EvidenceRow{
			ID:         "rpc-mint-observation-1",
			Network:    subject.Network,
			Relation:   "mint_authority",
			SubjectID:  subject.NativeID,
			Status:     "observed",
			Source:     "https://private-endpoint.example/?token=fixture-internal-secret",
			ObservedAt: time.Date(2026, time.October, 17, 8, 0, 0, 123, time.UTC),
			Height:     "123",
			RawHash:    hex.EncodeToString(digest[:]),
			Attributes: map[string]any{"mint_authority": "candidate"},
		},
		ResponseJSON: source,
	}
}

func TestRawJSONSourceWitnessBindsExactBytesWithoutPromotingEvidence(t *testing.T) {
	subject, observation := rawJSONWitnessFixture()
	rows, err := BindRawJSONSourceWitnesses(subject, []RawJSONObservation{observation})
	if err != nil || len(rows) != 1 {
		t.Fatalf("valid observed source: len=%d err=%v", len(rows), err)
	}
	row := rows[0]
	if row.Network != subject.Network || row.SubjectID != subject.NativeID ||
		row.ReportedHeight != "123" || len(row.ResponseSHA256) != 64 ||
		len(row.DeclaredAttributesSHA256) != 64 || len(row.CommitmentSHA256) != 64 ||
		row.IntegrityStatus != "collector_raw_json_bytes_match_declared_digest_only" {
		t.Fatalf("witness identity or digest missing: %#v", row)
	}
	if row.ChainEvidenceVerified || row.VerdictBound ||
		row.HeightIndependentlyVerified || row.AttributesIndependentlyProven {
		t.Fatalf("transport integrity was misreported as independent proof: %#v", row)
	}
	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"private-endpoint.example", "fixture-internal-secret", "private_token", "mint_authority"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("source payload or provider secret escaped witness: %s", data)
		}
	}
	again, err := BindRawJSONSourceWitnesses(subject, []RawJSONObservation{observation})
	if err != nil || rows[0].CommitmentSHA256 != again[0].CommitmentSHA256 {
		t.Fatalf("same input did not reproduce commitment: %#v %#v %v", rows, again, err)
	}
	observation.Row.Attributes["mint_authority"] = "different"
	changed, err := BindRawJSONSourceWitnesses(subject, []RawJSONObservation{observation})
	if err != nil || changed[0].CommitmentSHA256 == rows[0].CommitmentSHA256 {
		t.Fatalf("changed declared attributes did not change commitment: %#v %v", changed, err)
	}
}

func TestRawJSONSourceWitnessRejectsMissingAndTamperedSources(t *testing.T) {
	subject, original := rawJSONWitnessFixture()
	for _, tc := range []struct {
		name string
		edit func(*SubjectIdentity, *RawJSONObservation)
	}{
		{"response tamper", func(_ *SubjectIdentity, o *RawJSONObservation) { o.ResponseJSON = []byte(`{"result":{"context":{"slot":124}}}`) }},
		{"no source", func(_ *SubjectIdentity, o *RawJSONObservation) { o.ResponseJSON = nil }},
		{"invalid JSON", func(_ *SubjectIdentity, o *RawJSONObservation) { o.ResponseJSON = []byte("{") }},
		{"too large", func(_ *SubjectIdentity, o *RawJSONObservation) { o.ResponseJSON = []byte(strings.Repeat(" ", RawJSONSourceWitnessMaxBytes+1)) }},
		{"raw hash incomplete", func(_ *SubjectIdentity, o *RawJSONObservation) { o.Row.RawHash = "not-sha256" }},
		{"raw hash uppercase", func(_ *SubjectIdentity, o *RawJSONObservation) { o.Row.RawHash = strings.ToUpper(o.Row.RawHash) }},
		{"cross network", func(_ *SubjectIdentity, o *RawJSONObservation) { o.Row.Network = "ethereum-mainnet" }},
		{"cross subject", func(_ *SubjectIdentity, o *RawJSONObservation) { o.Row.SubjectID = "other-mint" }},
		{"unverified identity", func(s *SubjectIdentity, _ *RawJSONObservation) { s.NativeID = "" }},
		{"invented verified status", func(_ *SubjectIdentity, o *RawJSONObservation) { o.Row.Status = "verified" }},
		{"inferred cannot be a raw observation", func(_ *SubjectIdentity, o *RawJSONObservation) { o.Row.Status = "inferred" }},
		{"no height", func(_ *SubjectIdentity, o *RawJSONObservation) { o.Row.Height = "" }},
		{"zero height", func(_ *SubjectIdentity, o *RawJSONObservation) { o.Row.Height = "0" }},
		{"noncanonical height", func(_ *SubjectIdentity, o *RawJSONObservation) { o.Row.Height = "00123" }},
		{"overflow height", func(_ *SubjectIdentity, o *RawJSONObservation) { o.Row.Height = "18446744073709551616" }},
		{"missing relation", func(_ *SubjectIdentity, o *RawJSONObservation) { o.Row.Relation = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := subject
			o := original
			tc.edit(&s, &o)
			seen, err := BindRawJSONSourceWitnesses(s, []RawJSONObservation{o})
			if err == nil || len(seen) != 0 {
				t.Fatalf("invalid source was accepted: %#v", seen)
			}
		})
	}
	if rows, err := BindRawJSONSourceWitnesses(subject, nil); err == nil || rows != nil {
		t.Fatalf("missing observations accepted: %#v", rows)
	}
	other := original
	if rows, err := BindRawJSONSourceWitnesses(subject, []RawJSONObservation{original, other}); err == nil || rows != nil {
		t.Fatalf("duplicate observation IDs accepted: %#v", rows)
	}
}

func TestRawJSONSourceWitnessRejectsMixedOneGoodOneBadWithoutPartialOutput(t *testing.T) {
	subject, valid := rawJSONWitnessFixture()
	invalid := valid
	invalid.Row.ID = "different-id"
	invalid.ResponseJSON = []byte("{}")
	result, err := BindRawJSONSourceWitnesses(subject, []RawJSONObservation{valid, invalid})
	if err == nil || result != nil {
		t.Fatalf("partial witness escaped after one bad source: %#v %v", result, err)
	}
}
