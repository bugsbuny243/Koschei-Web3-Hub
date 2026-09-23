package services

import (
	"testing"
	"time"
)

func arvisVerdictReferenceFixture(subject IntelligenceSubject) (UnifiedRadarVerdict, IntelligenceDecision) {
	verdict := UnifiedRadarVerdict{
		Target:             subject.Raw,
		Network:            subject.Network,
		Grade:              "F",
		Verdict:            "hard_trigger",
		RulesetVersion:     "koschei-unified-radar-rules-v1.4.0",
		ActorRuleset:       "actor-rules-v1",
		Signed:             true,
		Signature:          "ZmFrZS1idXQtc3RydWN0dXJhbGx5LXZhbGlk",
		SignatureAlgorithm: UnifiedVerdictSignatureAlgorithmV1,
		KeyID:              "verdict-key-v1",
		PayloadHash:        "sha256:0123456789abcdef",
		GeneratedAt:        time.Date(2026, 9, 23, 18, 45, 0, 0, time.UTC),
	}
	decision := IntelligenceDecision{
		Status:       IntelligenceEvidenceVerified,
		Action:       "review_signed_verdict",
		Summary:      "Existing ARVIS decision is evidence-linked.",
		EvidenceRefs: []string{"evidence-1"},
		Confidence:   1,
	}
	return verdict, decision
}

func TestProjectARVISSignedVerdictToGlobalRadarPreservesAuthorityWithoutReverificationClaim(t *testing.T) {
	subject := ClassifyIntelligenceSubject("So11111111111111111111111111111111111111112", "solana-mainnet")
	subject.Kind = IntelligenceSubjectToken
	verdict, decision := arvisVerdictReferenceFixture(subject)

	got, err := ProjectARVISSignedVerdictToGlobalRadar(subject, verdict, decision)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != GlobalRadarVerdictReferenceSchemaVersion ||
		got.ReferenceID == "" ||
		got.AuthoritativeEngine != "arvis" ||
		got.TargetSubjectID != subject.ID {
		t.Fatalf("unexpected verdict reference identity: %#v", got)
	}
	if got.Grade != "F" || got.Verdict != "hard_trigger" ||
		!got.SourceMarkedSigned ||
		got.SignatureVerification != "not_reverified_by_global_radar" {
		t.Fatalf("verdict semantics were changed or verification overclaimed: %#v", got)
	}
	if got.ProjectionState != "authoritative_reference_only" ||
		got.TrustBoundary != "signature_requires_out_of_band_trusted_key_registry_for_independent_verification" {
		t.Fatalf("trust boundary missing: %#v", got)
	}
}

func TestProjectARVISSignedVerdictToGlobalRadarRequiresEvidenceLinkedDecision(t *testing.T) {
	subject := ClassifyIntelligenceSubject("So11111111111111111111111111111111111111112", "solana-mainnet")
	verdict, decision := arvisVerdictReferenceFixture(subject)

	decision.EvidenceRefs = nil
	if _, err := ProjectARVISSignedVerdictToGlobalRadar(subject, verdict, decision); err == nil {
		t.Fatal("verdict reference accepted decision without canonical evidence refs")
	}

	_, decision = arvisVerdictReferenceFixture(subject)
	decision.Action = "investigate"
	if _, err := ProjectARVISSignedVerdictToGlobalRadar(subject, verdict, decision); err == nil {
		t.Fatal("withheld intelligence decision was promoted into verdict reference")
	}
}

func TestProjectARVISSignedVerdictToGlobalRadarRejectsUnsignedOrCrossNetworkVerdict(t *testing.T) {
	subject := ClassifyIntelligenceSubject("So11111111111111111111111111111111111111112", "solana-mainnet")
	verdict, decision := arvisVerdictReferenceFixture(subject)

	verdict.Signed = false
	if _, err := ProjectARVISSignedVerdictToGlobalRadar(subject, verdict, decision); err == nil {
		t.Fatal("unsigned verdict was accepted")
	}

	verdict, decision = arvisVerdictReferenceFixture(subject)
	verdict.Network = "ethereum-mainnet"
	if _, err := ProjectARVISSignedVerdictToGlobalRadar(subject, verdict, decision); err == nil {
		t.Fatal("cross-network verdict identity mismatch was accepted")
	}
}
