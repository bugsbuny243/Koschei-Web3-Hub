package services

import (
	"encoding/base64"
	"strings"
	"testing"

	"koschei/api/internal/securityevidence"
)

func TestUnifiedSignedSecurityEvidenceMapsBitcoinUTXOFamily(t *testing.T) {
	privateKey, publicKey := unifiedSignedEvidenceTestKey(0x49)
	subject := securityevidence.Subject{Chain: "bitcoin", Type: "address", ID: "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"}
	event := securityevidence.Event{
		Producer: "bitcoin-attestation-v1",
		Subject:  subject,
		Findings: []securityevidence.Finding{{
			ID:             "address-activity-binding",
			Kind:           "address_activity",
			State:          securityevidence.StateObserved,
			EvidenceSHA256: strings.Repeat("e", 64),
		}},
	}
	signed, err := event.SignEd25519(privateKey)
	if err != nil {
		t.Fatalf("sign event: %v", err)
	}

	projection, err := AdaptUnifiedSignedSecurityEvidence(signed, UnifiedSignedEvidenceBinding{
		Domain:           UnifiedSecurityDomainBlockchain,
		SubjectKind:      IntelligenceSubjectAddress,
		Network:          "bitcoin-mainnet",
		ExpectedProducer: signed.Producer,
		TrustedPublicKey: base64.RawURLEncoding.EncodeToString(publicKey),
		ExpectedSubject:  subject,
	})
	if err != nil {
		t.Fatalf("adapt signed evidence: %v", err)
	}
	if projection.Subject.ChainFamily != IntelligenceChainFamilyUTXO {
		t.Fatalf("expected UTXO chain family, got %q", projection.Subject.ChainFamily)
	}
	if projection.Subject.Chain != "bitcoin" {
		t.Fatalf("unexpected authenticated chain namespace %q", projection.Subject.Chain)
	}
	if len(projection.Evidence) != 1 || projection.Evidence[0].ChainFamily != IntelligenceChainFamilyUTXO {
		t.Fatalf("bitcoin evidence did not preserve UTXO family: %#v", projection.Evidence)
	}
	if projection.Evidence[0].Status != IntelligenceEvidenceObserved || projection.Evidence[0].Confidence != 0.8 {
		t.Fatalf("bitcoin observed evidence semantics changed: status=%q confidence=%v", projection.Evidence[0].Status, projection.Evidence[0].Confidence)
	}
}

func TestUnifiedSignedEvidenceChainFamilyRecognizesBitcoinNamespaces(t *testing.T) {
	for _, chain := range []string{"bitcoin", "bitcoin:mainnet", "btc"} {
		if got := unifiedSignedEvidenceChainFamily(chain); got != IntelligenceChainFamilyUTXO {
			t.Fatalf("chain %q mapped to %q", chain, got)
		}
	}
}
