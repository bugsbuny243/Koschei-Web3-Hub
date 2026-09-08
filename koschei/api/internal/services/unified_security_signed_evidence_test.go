package services

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"

	"koschei/api/internal/securityevidence"
)

func TestUnifiedSignedSecurityEvidenceAuthenticatesAndProjectsFindings(t *testing.T) {
	privateKey, publicKey := unifiedSignedEvidenceTestKey(0x41)
	subject := securityevidence.Subject{Chain: "agent-runtime", Type: "agent", ID: "Agent-CaseSensitive-01"}
	event := securityevidence.Event{
		Producer: "external-agent-evidence-v1",
		Subject:  subject,
		Window: securityevidence.ObservationWindow{
			FromUnixMS: 1_000,
			ToUnixMS:   2_000,
		},
		SourceDigests: []string{strings.Repeat("b", 64)},
		Findings: []securityevidence.Finding{
			{
				ID:             "finding-verified",
				Kind:           "tool_use_grant",
				State:          securityevidence.StateVerified,
				EvidenceSHA256: strings.Repeat("a", 64),
				Summary:        "Agent grant was verified by the configured evidence producer.",
			},
			{
				ID:      "finding-observed",
				Kind:    "tool_call",
				State:   securityevidence.StateObserved,
				Summary: "A tool call was observed in the signed evidence window.",
			},
		},
	}
	signed, err := event.SignEd25519(privateKey)
	if err != nil {
		t.Fatalf("sign event: %v", err)
	}

	projection, err := AdaptUnifiedSignedSecurityEvidence(signed, UnifiedSignedEvidenceBinding{
		Domain:           UnifiedSecurityDomainAIAgent,
		SubjectKind:      IntelligenceSubjectAgent,
		Network:          "agent-control-plane",
		ExpectedProducer: signed.Producer,
		TrustedPublicKey: base64.RawURLEncoding.EncodeToString(publicKey),
		ExpectedSubject:  subject,
	})
	if err != nil {
		t.Fatalf("adapt signed evidence: %v", err)
	}
	if projection.Subject.Raw != subject.ID {
		t.Fatalf("subject id changed: got %q want %q", projection.Subject.Raw, subject.ID)
	}
	if projection.Subject.Kind != IntelligenceSubjectAgent {
		t.Fatalf("unexpected subject kind %q", projection.Subject.Kind)
	}
	if projection.Subject.ClassificationBasis != "authenticated_security_evidence_subject_binding" {
		t.Fatalf("unexpected classification basis %q", projection.Subject.ClassificationBasis)
	}
	if len(projection.Evidence) != 2 {
		t.Fatalf("expected 2 evidence rows, got %d", len(projection.Evidence))
	}
	if projection.Evidence[0].Status != IntelligenceEvidenceObserved || projection.Evidence[1].Status != IntelligenceEvidenceVerified {
		t.Fatalf("unexpected canonical finding statuses: %#v", []string{projection.Evidence[0].Status, projection.Evidence[1].Status})
	}
	for _, evidence := range projection.Evidence {
		if evidence.SubjectID != projection.Subject.ID {
			t.Fatalf("evidence subject mismatch: %q != %q", evidence.SubjectID, projection.Subject.ID)
		}
		if evidence.Provenance != unifiedSignedEvidenceProvenance {
			t.Fatalf("unexpected provenance %q", evidence.Provenance)
		}
		if evidence.Attributes["event_sha256"] != signed.EventSHA256 {
			t.Fatalf("event digest not preserved in attributes: %#v", evidence.Attributes["event_sha256"])
		}
	}
	if projection.Evidence[1].Confidence != 1 {
		t.Fatalf("verified evidence confidence metadata changed: %v", projection.Evidence[1].Confidence)
	}
}

func TestUnifiedSignedSecurityEvidenceRejectsProducerMismatch(t *testing.T) {
	privateKey, publicKey := unifiedSignedEvidenceTestKey(0x42)
	signed := unifiedSignedEvidenceTestEvent(t, privateKey)

	_, err := AdaptUnifiedSignedSecurityEvidence(signed, UnifiedSignedEvidenceBinding{
		Domain:           UnifiedSecurityDomainProtocol,
		SubjectKind:      IntelligenceSubjectAsset,
		ExpectedProducer: "different-producer",
		TrustedPublicKey: base64.RawURLEncoding.EncodeToString(publicKey),
		ExpectedSubject:  signed.Subject,
	})
	if err == nil {
		t.Fatal("producer mismatch unexpectedly accepted")
	}
}

func TestUnifiedSignedSecurityEvidenceRejectsSubjectSubstitution(t *testing.T) {
	privateKey, publicKey := unifiedSignedEvidenceTestKey(0x43)
	signed := unifiedSignedEvidenceTestEvent(t, privateKey)
	expected := signed.Subject
	expected.ID = strings.ToLower(expected.ID)

	_, err := AdaptUnifiedSignedSecurityEvidence(signed, UnifiedSignedEvidenceBinding{
		Domain:           UnifiedSecurityDomainIdentity,
		SubjectKind:      IntelligenceSubjectIdentity,
		ExpectedProducer: signed.Producer,
		TrustedPublicKey: base64.RawURLEncoding.EncodeToString(publicKey),
		ExpectedSubject:  expected,
	})
	if err == nil {
		t.Fatal("case-changing subject substitution unexpectedly accepted")
	}
}

func TestUnifiedSignedSecurityEvidenceRejectsTampering(t *testing.T) {
	privateKey, publicKey := unifiedSignedEvidenceTestKey(0x44)
	signed := unifiedSignedEvidenceTestEvent(t, privateKey)
	signed.Findings[0].Summary = "tampered after signing"

	_, err := AdaptUnifiedSignedSecurityEvidence(signed, UnifiedSignedEvidenceBinding{
		Domain:           UnifiedSecurityDomainProtocol,
		SubjectKind:      IntelligenceSubjectAsset,
		ExpectedProducer: signed.Producer,
		TrustedPublicKey: base64.RawURLEncoding.EncodeToString(publicKey),
		ExpectedSubject:  signed.Subject,
	})
	if err == nil {
		t.Fatal("tampered signed event unexpectedly accepted")
	}
}

func TestUnifiedSignedSecurityEvidenceUnavailableFindingStaysUnverified(t *testing.T) {
	privateKey, publicKey := unifiedSignedEvidenceTestKey(0x45)
	subject := securityevidence.Subject{Chain: "identity-control", Type: "credential", ID: "Credential-ABC"}
	event := securityevidence.Event{
		Producer: "identity-evidence-v1",
		Subject:  subject,
		Findings: []securityevidence.Finding{{
			ID:      "credential-state",
			Kind:    "credential_status",
			State:   securityevidence.StateUnavailable,
			Summary: "Credential status source was unavailable.",
		}},
	}
	signed, err := event.SignEd25519(privateKey)
	if err != nil {
		t.Fatalf("sign event: %v", err)
	}

	projection, err := AdaptUnifiedSignedSecurityEvidence(signed, UnifiedSignedEvidenceBinding{
		Domain:           UnifiedSecurityDomainIdentity,
		SubjectKind:      IntelligenceSubjectCredential,
		ExpectedProducer: signed.Producer,
		TrustedPublicKey: base64.RawURLEncoding.EncodeToString(publicKey),
		ExpectedSubject:  subject,
	})
	if err != nil {
		t.Fatalf("adapt signed evidence: %v", err)
	}
	if len(projection.Evidence) != 1 || projection.Evidence[0].Status != IntelligenceEvidenceUnverified {
		t.Fatalf("unavailable finding was upgraded: %#v", projection.Evidence)
	}
}

func TestUnifiedSignedSecurityEvidenceUnknownDomainFailsClosed(t *testing.T) {
	privateKey, publicKey := unifiedSignedEvidenceTestKey(0x46)
	signed := unifiedSignedEvidenceTestEvent(t, privateKey)

	_, err := AdaptUnifiedSignedSecurityEvidence(signed, UnifiedSignedEvidenceBinding{
		Domain:           "future-unregistered-domain",
		SubjectKind:      IntelligenceSubjectAsset,
		ExpectedProducer: signed.Producer,
		TrustedPublicKey: base64.RawURLEncoding.EncodeToString(publicKey),
		ExpectedSubject:  signed.Subject,
	})
	if err == nil {
		t.Fatal("unknown domain unexpectedly accepted")
	}
}

func unifiedSignedEvidenceTestEvent(t *testing.T, privateKey ed25519.PrivateKey) securityevidence.Event {
	t.Helper()
	event := securityevidence.Event{
		Producer: "test-evidence-producer-v1",
		Subject:  securityevidence.Subject{Chain: "protocol", Type: "asset", ID: "Asset-CaseSensitive-01"},
		Findings: []securityevidence.Finding{{
			ID:             "asset-control",
			Kind:           "control_state",
			State:          securityevidence.StateVerified,
			EvidenceSHA256: strings.Repeat("c", 64),
			Summary:        "Control state was verified.",
		}},
	}
	signed, err := event.SignEd25519(privateKey)
	if err != nil {
		t.Fatalf("sign test event: %v", err)
	}
	return signed
}

func unifiedSignedEvidenceTestKey(fill byte) (ed25519.PrivateKey, ed25519.PublicKey) {
	seed := bytes.Repeat([]byte{fill}, ed25519.SeedSize)
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return privateKey, publicKey
}

func TestUnifiedSignedSecurityEvidenceRejectsEmptyFindingSet(t *testing.T) {
	privateKey, publicKey := unifiedSignedEvidenceTestKey(0x47)
	subject := securityevidence.Subject{Chain: "protocol", Type: "asset", ID: "Asset-01"}
	signed, err := (securityevidence.Event{
		Producer: "empty-evidence-producer-v1",
		Subject:  subject,
	}).SignEd25519(privateKey)
	if err != nil {
		t.Fatalf("sign event: %v", err)
	}

	_, err = AdaptUnifiedSignedSecurityEvidence(signed, UnifiedSignedEvidenceBinding{
		Domain:           UnifiedSecurityDomainProtocol,
		SubjectKind:      IntelligenceSubjectAsset,
		ExpectedProducer: signed.Producer,
		TrustedPublicKey: base64.RawURLEncoding.EncodeToString(publicKey),
		ExpectedSubject:  subject,
	})
	if err == nil {
		t.Fatal("empty signed event unexpectedly accepted as evidence")
	}
}

func TestUnifiedSignedSecurityEvidenceMapsAuthenticatedBlockchainNamespace(t *testing.T) {
	privateKey, publicKey := unifiedSignedEvidenceTestKey(0x48)
	subject := securityevidence.Subject{Chain: "eip155:1", Type: "safe-transaction", ID: "0xABCDEF"}
	event := securityevidence.Event{
		Producer: "safe-attestation-v1",
		Subject:  subject,
		Findings: []securityevidence.Finding{{
			ID:             "safe-hash-binding",
			Kind:           "transaction_binding",
			State:          securityevidence.StateVerified,
			EvidenceSHA256: strings.Repeat("d", 64),
		}},
	}
	signed, err := event.SignEd25519(privateKey)
	if err != nil {
		t.Fatalf("sign event: %v", err)
	}

	projection, err := AdaptUnifiedSignedSecurityEvidence(signed, UnifiedSignedEvidenceBinding{
		Domain:           UnifiedSecurityDomainBlockchain,
		SubjectKind:      IntelligenceSubjectAsset,
		Network:          "ethereum-mainnet",
		ExpectedProducer: signed.Producer,
		TrustedPublicKey: base64.RawURLEncoding.EncodeToString(publicKey),
		ExpectedSubject:  subject,
	})
	if err != nil {
		t.Fatalf("adapt signed evidence: %v", err)
	}
	if projection.Subject.ChainFamily != IntelligenceChainFamilyEVM {
		t.Fatalf("expected EVM chain family, got %q", projection.Subject.ChainFamily)
	}
	if projection.Subject.Chain != "eip155:1" {
		t.Fatalf("unexpected authenticated chain namespace %q", projection.Subject.Chain)
	}
}
