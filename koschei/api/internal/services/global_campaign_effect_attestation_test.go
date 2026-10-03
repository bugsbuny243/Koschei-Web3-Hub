package services

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"sort"
	"testing"
	"time"

	"koschei/api/internal/executioncontainment"
	"koschei/api/internal/securityevidence"
)

func signedEffectAttestationFixture(t *testing.T) (GlobalCampaign, GlobalCampaignResponseProposal, GlobalCampaignResponseAuthorization, executioncontainment.Receipt, GlobalCampaignResponseEffectProof, securityevidence.Event, GlobalCampaignEffectAttestationTrust, time.Time) {
	t.Helper()
	campaign, fabric, proposal, authorization, preflight, now := effectProofFixture(t)
	input := validEffectProofInput(now)
	input.VerifierRef = "collector:response-effect-production"
	proof, err := NewGlobalCampaignResponseEffectProof(campaign, fabric, proposal, authorization, preflight, input, now)
	if err != nil {
		t.Fatalf("build canonical effect proof: %v", err)
	}

	binding, err := BuildGlobalCampaignEffectAttestationBinding(campaign, proposal, authorization, proof)
	if err != nil {
		t.Fatalf("build attestation binding: %v", err)
	}
	bindingDigest, err := GlobalCampaignEffectAttestationBindingDigest(binding)
	if err != nil {
		t.Fatalf("hash attestation binding: %v", err)
	}

	privateKey := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	sources := []string{
		rawResponseDigest(proof.ProofHashSHA256),
		rawResponseDigest(authorization.AuthorizationHashSHA256),
		rawResponseDigest(proposal.ActionSHA256),
		rawResponseDigest(proof.ExecutionEvidenceSHA256),
		rawResponseDigest(proof.VerificationEvidenceSHA256),
	}
	sort.Strings(sources)
	event, err := (securityevidence.Event{
		Producer: "collector:response-effect-production",
		Subject: securityevidence.Subject{
			Chain: proposal.Network,
			Type:  GlobalCampaignEffectAttestationSubjectType,
			ID:    proof.ExecutionRef,
		},
		Window: securityevidence.ObservationWindow{
			FromUnixMS: proof.VerifiedAt.UnixMilli(),
			ToUnixMS:   proof.VerifiedAt.UnixMilli(),
		},
		SourceDigests: sources,
		Findings: []securityevidence.Finding{{
			ID:             GlobalCampaignEffectAttestationFindingID,
			Kind:           GlobalCampaignEffectAttestationFindingKind,
			State:          securityevidence.StateVerified,
			EvidenceSHA256: bindingDigest,
		}},
	}).SignEd25519(privateKey)
	if err != nil {
		t.Fatalf("sign effect evidence event: %v", err)
	}
	trust := GlobalCampaignEffectAttestationTrust{
		Producer:      "collector:response-effect-production",
		PublicKey:     base64.RawURLEncoding.EncodeToString(publicKey),
		MaxAge:        5 * time.Minute,
		MaxFutureSkew: 5 * time.Second,
	}
	return campaign, proposal, authorization, preflight, proof, event, trust, proof.VerifiedAt.Add(time.Second)
}

func TestGlobalCampaignEffectAttestationAuthenticatesIndependentCollector(t *testing.T) {
	campaign, proposal, authorization, preflight, proof, event, trust, now := signedEffectAttestationFixture(t)
	attestation, err := BuildGlobalCampaignEffectAttestation(campaign, proposal, authorization, preflight, proof, event, trust, now)
	if err != nil {
		t.Fatalf("build signed effect attestation: %v", err)
	}
	if !attestation.Authenticated || attestation.ExecutionAuthority || attestation.SubmissionAuthority || attestation.AttestationSHA256 == "" {
		t.Fatalf("unexpected signed effect attestation: %+v", attestation)
	}
	if err := ValidateGlobalCampaignEffectAttestation(campaign, proposal, authorization, preflight, proof, event, trust, attestation); err != nil {
		t.Fatalf("validate signed effect attestation: %v", err)
	}
}

func TestGlobalCampaignEffectAttestationRejectsWrongCollectorKey(t *testing.T) {
	campaign, proposal, authorization, preflight, proof, event, trust, now := signedEffectAttestationFixture(t)
	other := ed25519.NewKeyFromSeed(bytesOf(1, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	trust.PublicKey = base64.RawURLEncoding.EncodeToString(other)
	if _, err := BuildGlobalCampaignEffectAttestation(campaign, proposal, authorization, preflight, proof, event, trust, now); !errors.Is(err, ErrGlobalCampaignEffectAttestationUntrusted) {
		t.Fatalf("expected untrusted collector rejection, got %v", err)
	}
}

func TestGlobalCampaignEffectAttestationRejectsMismatchedEvidence(t *testing.T) {
	campaign, proposal, authorization, preflight, proof, event, trust, now := signedEffectAttestationFixture(t)
	privateKey := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	event.SourceDigests = append([]string(nil), event.SourceDigests...)
	event.SourceDigests[0] = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	event.Authentication = nil
	event.EventSHA256 = ""
	resigned, err := event.SignEd25519(privateKey)
	if err != nil {
		t.Fatalf("resign mismatched event: %v", err)
	}
	if _, err := BuildGlobalCampaignEffectAttestation(campaign, proposal, authorization, preflight, proof, resigned, trust, now); !errors.Is(err, ErrGlobalCampaignEffectAttestationMismatch) {
		t.Fatalf("expected binding mismatch, got %v", err)
	}
}

func TestGlobalCampaignEffectAttestationRejectsStaleEvidence(t *testing.T) {
	campaign, proposal, authorization, preflight, proof, event, trust, now := signedEffectAttestationFixture(t)
	if _, err := BuildGlobalCampaignEffectAttestation(campaign, proposal, authorization, preflight, proof, event, trust, now.Add(10*time.Minute)); !errors.Is(err, ErrGlobalCampaignEffectAttestationStale) {
		t.Fatalf("expected stale evidence rejection, got %v", err)
	}
}

func TestGlobalCampaignEffectAttestationRejectsSelfAttestingExecutor(t *testing.T) {
	campaign, proposal, authorization, preflight, proof, event, trust, now := signedEffectAttestationFixture(t)
	proof.ExecutorRef = trust.Producer
	proof.ProofHashSHA256 = hashGlobalCampaignResponseEffectProof(proof)
	if _, err := BuildGlobalCampaignEffectAttestation(campaign, proposal, authorization, preflight, proof, event, trust, now); err == nil {
		t.Fatal("expected self-attesting executor rejection")
	}
}

func bytesOf(value byte, count int) []byte {
	out := make([]byte, count)
	for index := range out {
		out[index] = value
	}
	return out
}
