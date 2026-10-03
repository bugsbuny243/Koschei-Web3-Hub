package services

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"koschei/api/internal/executioncontainment"
	"koschei/api/internal/securityevidence"
)

func responseEffectFixture(t *testing.T) (GlobalCampaign, FabricCampaignEvidenceContract, GlobalCampaignResponseProposal, GlobalCampaignResponseAuthorization, executioncontainment.Receipt, string, time.Time) {
	t.Helper()
	campaign, fabric, proposal, now := responseAuthorizationFixture(t)
	authorization, err := NewGlobalCampaignResponseAuthorization(campaign, fabric, proposal, GlobalCampaignResponseAuthorization{
		AuthorizerKind:           GlobalCampaignResponseAuthorizerOperator,
		AuthorizerRef:            "operator:oncall-1",
		AuthorizationEvidenceRef: "approval:ticket-42",
		AuthorizedAt:             now.Add(-time.Minute),
		ExpiresAt:                now.Add(10 * time.Minute),
		ExplicitAuthorization:    true,
	}, now)
	if err != nil {
		t.Fatalf("authorize response: %v", err)
	}

	input := executioncontainment.CellInput{
		Version:                executioncontainment.Version,
		ChainID:                1,
		BlockNumber:            123456,
		BlockHash:              "1111111111111111111111111111111111111111111111111111111111111111",
		Target:                 proposal.Target,
		ApprovedIntentSHA256:   "2222222222222222222222222222222222222222222222222222222222222222",
		CandidateIntentSHA256:  "2222222222222222222222222222222222222222222222222222222222222222",
		ApprovedPayloadSHA256:  "3333333333333333333333333333333333333333333333333333333333333333",
		CandidatePayloadSHA256: "3333333333333333333333333333333333333333333333333333333333333333",
		ActionSHA256:           responseDigestHex(proposal.ActionSHA256),
		InvariantSetSHA256:     "4444444444444444444444444444444444444444444444444444444444444444",
		ApprovedRunnerSHA256:   "5555555555555555555555555555555555555555555555555555555555555555",
	}
	observation := executioncontainment.Observation{
		BackendAvailable:           true,
		ObservedChainID:            1,
		ObservedBlockNumber:        123456,
		ObservedBlockHash:          input.BlockHash,
		ObservedRunnerSHA256:       input.ApprovedRunnerSHA256,
		PreStateSHA256:             "6666666666666666666666666666666666666666666666666666666666666666",
		PostStateSHA256:            "7777777777777777777777777777777777777777777777777777777777777777",
		EffectSetSHA256:            "8888888888888888888888888888888888888888888888888888888888888888",
		AuthorityPreserved:         true,
		AssetBoundsPreserved:       true,
		CodeIntegrityPreserved:     true,
		ExecutionPathFullyObserved: true,
		InvariantsPass:             true,
	}
	receipt, err := executioncontainment.Evaluate(input, observation)
	if err != nil {
		t.Fatalf("evaluate containment receipt: %v", err)
	}
	if receipt.Decision != executioncontainment.DecisionRelease || !executioncontainment.Verify(receipt) {
		t.Fatalf("fixture receipt is not a verified RELEASE: decision=%s", receipt.Decision)
	}
	return campaign, fabric, proposal, authorization, receipt, "execution:response-42", now
}

func responseEffectTrust(seedByte byte, producer string) (GlobalCampaignResponseEffectTrust, ed25519.PrivateKey) {
	seed := bytes.Repeat([]byte{seedByte}, ed25519.SeedSize)
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return GlobalCampaignResponseEffectTrust{
		Producer:      producer,
		PublicKey:     base64.RawURLEncoding.EncodeToString(publicKey),
		MaxAge:        10 * time.Minute,
		MaxFutureSkew: 5 * time.Second,
	}, privateKey
}

func signedResponseEffectEvent(t *testing.T, binding GlobalCampaignResponseEffectBinding, producer string, privateKey ed25519.PrivateKey, from, to time.Time) securityevidence.Event {
	t.Helper()
	bindingDigest, err := GlobalCampaignResponseEffectBindingDigest(binding)
	if err != nil {
		t.Fatalf("binding digest: %v", err)
	}
	event := securityevidence.Event{
		Producer: producer,
		Subject: securityevidence.Subject{
			Chain: binding.Network,
			Type:  GlobalCampaignResponseEffectSubjectType,
			ID:    binding.ExecutionRef,
		},
		Window: securityevidence.ObservationWindow{
			FromUnixMS: from.UTC().UnixMilli(),
			ToUnixMS:   to.UTC().UnixMilli(),
		},
		SourceDigests: []string{
			binding.AuthorizationHashSHA256,
			binding.ProposalHashSHA256,
			binding.ActionSHA256,
			binding.ContainmentReceiptSHA256,
		},
		Findings: []securityevidence.Finding{{
			ID:             GlobalCampaignResponseEffectFindingID,
			Kind:           GlobalCampaignResponseEffectFindingKind,
			State:          securityevidence.StateVerified,
			EvidenceSHA256: bindingDigest,
		}},
	}
	signed, err := event.SignEd25519(privateKey)
	if err != nil {
		t.Fatalf("sign response effect event: %v", err)
	}
	return signed
}

func TestGlobalCampaignResponseEffectProofRequiresIndependentSignedEffect(t *testing.T) {
	campaign, fabric, proposal, authorization, receipt, executionRef, now := responseEffectFixture(t)
	from := now.Add(-30 * time.Second)
	to := now
	binding, err := BuildGlobalCampaignResponseEffectBinding(campaign, fabric, proposal, authorization, receipt, executionRef, from)
	if err != nil {
		t.Fatalf("build effect binding: %v", err)
	}
	trust, privateKey := responseEffectTrust(7, "collector:response-effect-1")
	event := signedResponseEffectEvent(t, binding, trust.Producer, privateKey, from, to)

	proof, err := BuildGlobalCampaignResponseEffectProof(campaign, fabric, proposal, authorization, receipt, executionRef, event, trust, now.Add(time.Second))
	if err != nil {
		t.Fatalf("build response effect proof: %v", err)
	}
	if !proof.ContainmentEffectVerified || proof.ExecutionAuthority || proof.ProductionForwardingClaim {
		t.Fatalf("unexpected authority/claim flags: %+v", proof)
	}
	if proof.ProofHashSHA256 == "" || proof.BindingHashSHA256 == "" || proof.EvidenceEventSHA256 == "" {
		t.Fatalf("missing immutable proof digests: %+v", proof)
	}
	if err := ValidateGlobalCampaignResponseEffectProof(campaign, fabric, proposal, authorization, receipt, event, trust, proof); err != nil {
		t.Fatalf("validate response effect proof: %v", err)
	}
}

func TestGlobalCampaignResponseEffectProofRejectsMismatchedAction(t *testing.T) {
	campaign, fabric, proposal, authorization, receipt, executionRef, now := responseEffectFixture(t)
	input := receipt.Input
	input.ActionSHA256 = "9999999999999999999999999999999999999999999999999999999999999999"
	mismatched, err := executioncontainment.Evaluate(input, receipt.Observation)
	if err != nil {
		t.Fatalf("rebuild mismatched receipt: %v", err)
	}
	_, err = BuildGlobalCampaignResponseEffectBinding(campaign, fabric, proposal, authorization, mismatched, executionRef, now)
	if !errors.Is(err, ErrGlobalCampaignResponseEffectMismatch) {
		t.Fatalf("expected action mismatch, got %v", err)
	}
}

func TestGlobalCampaignResponseEffectProofRejectsTamperedReceipt(t *testing.T) {
	campaign, fabric, proposal, authorization, receipt, executionRef, now := responseEffectFixture(t)
	receipt.Observation.PostStateSHA256 = "9999999999999999999999999999999999999999999999999999999999999999"
	_, err := BuildGlobalCampaignResponseEffectBinding(campaign, fabric, proposal, authorization, receipt, executionRef, now)
	if !errors.Is(err, ErrGlobalCampaignResponseEffectUnavailable) {
		t.Fatalf("expected unavailable effect for tampered receipt, got %v", err)
	}
}

func TestGlobalCampaignResponseEffectProofRejectsAuthorizerAsVerifier(t *testing.T) {
	campaign, fabric, proposal, authorization, receipt, executionRef, now := responseEffectFixture(t)
	from := now.Add(-30 * time.Second)
	binding, err := BuildGlobalCampaignResponseEffectBinding(campaign, fabric, proposal, authorization, receipt, executionRef, from)
	if err != nil {
		t.Fatalf("build effect binding: %v", err)
	}
	trust, privateKey := responseEffectTrust(8, authorization.AuthorizerRef)
	event := signedResponseEffectEvent(t, binding, trust.Producer, privateKey, from, now)
	_, err = BuildGlobalCampaignResponseEffectProof(campaign, fabric, proposal, authorization, receipt, executionRef, event, trust, now.Add(time.Second))
	if !errors.Is(err, ErrGlobalCampaignResponseEffectUntrusted) {
		t.Fatalf("expected independent verifier rejection, got %v", err)
	}
}

func TestGlobalCampaignResponseEffectProofRejectsStaleEvidence(t *testing.T) {
	campaign, fabric, proposal, authorization, receipt, executionRef, now := responseEffectFixture(t)
	from := now.Add(-30 * time.Second)
	binding, err := BuildGlobalCampaignResponseEffectBinding(campaign, fabric, proposal, authorization, receipt, executionRef, from)
	if err != nil {
		t.Fatalf("build effect binding: %v", err)
	}
	trust, privateKey := responseEffectTrust(9, "collector:response-effect-2")
	trust.MaxAge = 2 * time.Minute
	event := signedResponseEffectEvent(t, binding, trust.Producer, privateKey, from, now)
	_, err = BuildGlobalCampaignResponseEffectProof(campaign, fabric, proposal, authorization, receipt, executionRef, event, trust, now.Add(3*time.Minute))
	if !errors.Is(err, ErrGlobalCampaignResponseEffectStale) {
		t.Fatalf("expected stale evidence rejection, got %v", err)
	}
}

func TestGlobalCampaignResponseEffectProofRejectsWrongExecutionRef(t *testing.T) {
	campaign, fabric, proposal, authorization, receipt, executionRef, now := responseEffectFixture(t)
	from := now.Add(-30 * time.Second)
	binding, err := BuildGlobalCampaignResponseEffectBinding(campaign, fabric, proposal, authorization, receipt, executionRef, from)
	if err != nil {
		t.Fatalf("build effect binding: %v", err)
	}
	trust, privateKey := responseEffectTrust(10, "collector:response-effect-3")
	event := signedResponseEffectEvent(t, binding, trust.Producer, privateKey, from, now)
	_, err = BuildGlobalCampaignResponseEffectProof(campaign, fabric, proposal, authorization, receipt, "execution:other", event, trust, now.Add(time.Second))
	if !errors.Is(err, ErrGlobalCampaignResponseEffectMismatch) {
		t.Fatalf("expected execution ref mismatch, got %v", err)
	}
}

func TestGlobalCampaignResponseEffectProofRejectsProofTamper(t *testing.T) {
	campaign, fabric, proposal, authorization, receipt, executionRef, now := responseEffectFixture(t)
	from := now.Add(-30 * time.Second)
	binding, err := BuildGlobalCampaignResponseEffectBinding(campaign, fabric, proposal, authorization, receipt, executionRef, from)
	if err != nil {
		t.Fatalf("build effect binding: %v", err)
	}
	trust, privateKey := responseEffectTrust(11, "collector:response-effect-4")
	event := signedResponseEffectEvent(t, binding, trust.Producer, privateKey, from, now)
	proof, err := BuildGlobalCampaignResponseEffectProof(campaign, fabric, proposal, authorization, receipt, executionRef, event, trust, now.Add(time.Second))
	if err != nil {
		t.Fatalf("build response effect proof: %v", err)
	}
	proof.Binding.PostStateSHA256 = "9999999999999999999999999999999999999999999999999999999999999999"
	if err := ValidateGlobalCampaignResponseEffectProof(campaign, fabric, proposal, authorization, receipt, event, trust, proof); !errors.Is(err, ErrGlobalCampaignResponseEffectInvalid) {
		t.Fatalf("expected tampered proof rejection, got %v", err)
	}
}
