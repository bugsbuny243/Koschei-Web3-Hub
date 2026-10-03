package services

import (
	"errors"
	"testing"
	"time"

	"koschei/api/internal/executioncontainment"
)

func effectProofFixture(t *testing.T) (GlobalCampaign, FabricCampaignEvidenceContract, GlobalCampaignResponseProposal, GlobalCampaignResponseAuthorization, executioncontainment.Receipt, time.Time) {
	t.Helper()
	campaign, fabric, proposal, now := responseAuthorizationFixture(t)
	auth, err := NewGlobalCampaignResponseAuthorization(campaign, fabric, proposal, GlobalCampaignResponseAuthorization{
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

	preflight, err := executioncontainment.Evaluate(executioncontainment.CellInput{
		Version:                executioncontainment.Version,
		ChainID:                1,
		BlockNumber:            100,
		BlockHash:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Target:                 proposal.Target,
		ApprovedIntentSHA256:   "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		CandidateIntentSHA256:  "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		ApprovedPayloadSHA256:  "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		CandidatePayloadSHA256: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		ActionSHA256:           proposal.ActionSHA256,
		InvariantSetSHA256:     "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		ApprovedRunnerSHA256:   "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	}, executioncontainment.Observation{
		BackendAvailable:           true,
		ObservedChainID:            1,
		ObservedBlockNumber:        100,
		ObservedBlockHash:          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ObservedRunnerSHA256:       "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		PreStateSHA256:             "1111111111111111111111111111111111111111111111111111111111111111",
		PostStateSHA256:            "2222222222222222222222222222222222222222222222222222222222222222",
		EffectSetSHA256:            "3333333333333333333333333333333333333333333333333333333333333333",
		AuthorityPreserved:         true,
		AssetBoundsPreserved:       true,
		CodeIntegrityPreserved:     true,
		ExecutionPathFullyObserved: true,
		InvariantsPass:             true,
	})
	if err != nil {
		t.Fatalf("build preflight: %v", err)
	}
	if preflight.Decision != executioncontainment.DecisionRelease {
		t.Fatalf("expected RELEASE preflight, got %s", preflight.Decision)
	}
	return campaign, fabric, proposal, auth, preflight, now
}

func validEffectProofInput(now time.Time) GlobalCampaignResponseEffectProof {
	return GlobalCampaignResponseEffectProof{
		ExecutionRef:               "execution:tx:abc",
		ExecutionEvidenceSHA256:    "4444444444444444444444444444444444444444444444444444444444444444",
		ExecutorRef:                "executor:prod-1",
		ExecutedAt:                 now.Add(time.Minute),
		PreStateSHA256:             "5555555555555555555555555555555555555555555555555555555555555555",
		PostStateSHA256:            "6666666666666666666666666666666666666666666666666666666666666666",
		EffectSetSHA256:            "7777777777777777777777777777777777777777777777777777777777777777",
		VerifierRef:                "verifier:independent-1",
		VerificationEvidenceSHA256: "8888888888888888888888888888888888888888888888888888888888888888",
		VerifiedAt:                 now.Add(2 * time.Minute),
		EffectVerified:             true,
		AuthorityPreserved:         true,
		AssetBoundsPreserved:       true,
		CodeIntegrityPreserved:     true,
		ExecutionPathObserved:      true,
		IndependentVerifier:        true,
	}
}

func TestGlobalCampaignResponseEffectProofVerified(t *testing.T) {
	campaign, fabric, proposal, auth, preflight, now := effectProofFixture(t)
	proof, err := NewGlobalCampaignResponseEffectProof(campaign, fabric, proposal, auth, preflight, validEffectProofInput(now), now)
	if err != nil {
		t.Fatalf("build effect proof: %v", err)
	}
	if proof.State != GlobalCampaignEffectStateVerified || !proof.ContainmentProven || proof.ProofHashSHA256 == "" {
		t.Fatalf("expected verified containment proof, got state=%s proven=%v hash=%q", proof.State, proof.ContainmentProven, proof.ProofHashSHA256)
	}
	if err := ValidateGlobalCampaignResponseEffectProof(campaign, proposal, auth, preflight, proof); err != nil {
		t.Fatalf("validate effect proof: %v", err)
	}
}

func TestGlobalCampaignResponseEffectProofRequiresIndependentVerifier(t *testing.T) {
	campaign, fabric, proposal, auth, preflight, now := effectProofFixture(t)
	input := validEffectProofInput(now)
	input.VerifierRef = input.ExecutorRef
	if _, err := NewGlobalCampaignResponseEffectProof(campaign, fabric, proposal, auth, preflight, input, now); !errors.Is(err, ErrGlobalCampaignEffectProofInvalid) {
		t.Fatalf("expected independent verifier rejection, got %v", err)
	}
}

func TestGlobalCampaignResponseEffectProofRejectsUnsafePreflight(t *testing.T) {
	campaign, fabric, proposal, auth, preflight, now := effectProofFixture(t)
	preflight.Observation.InvariantsPass = false
	preflight, _ = executioncontainment.Evaluate(preflight.Input, preflight.Observation)
	if preflight.Decision != executioncontainment.DecisionContain {
		t.Fatalf("expected CONTAIN preflight, got %s", preflight.Decision)
	}
	if _, err := NewGlobalCampaignResponseEffectProof(campaign, fabric, proposal, auth, preflight, validEffectProofInput(now), now); !errors.Is(err, ErrGlobalCampaignEffectProofInvalid) {
		t.Fatalf("expected unsafe preflight rejection, got %v", err)
	}
}

func TestGlobalCampaignResponseEffectProofDoesNotOverclaimPartialEffect(t *testing.T) {
	campaign, fabric, proposal, auth, preflight, now := effectProofFixture(t)
	input := validEffectProofInput(now)
	input.EffectVerified = false
	proof, err := NewGlobalCampaignResponseEffectProof(campaign, fabric, proposal, auth, preflight, input, now)
	if err != nil {
		t.Fatalf("build partial effect proof: %v", err)
	}
	if proof.State != GlobalCampaignEffectStateExecuted || proof.ContainmentProven {
		t.Fatalf("partial effect must remain executed/not proven, got state=%s proven=%v", proof.State, proof.ContainmentProven)
	}
}

func TestGlobalCampaignResponseEffectProofRejectsTamper(t *testing.T) {
	campaign, fabric, proposal, auth, preflight, now := effectProofFixture(t)
	proof, err := NewGlobalCampaignResponseEffectProof(campaign, fabric, proposal, auth, preflight, validEffectProofInput(now), now)
	if err != nil {
		t.Fatalf("build effect proof: %v", err)
	}
	proof.PostStateSHA256 = "9999999999999999999999999999999999999999999999999999999999999999"
	if err := ValidateGlobalCampaignResponseEffectProof(campaign, proposal, auth, preflight, proof); !errors.Is(err, ErrGlobalCampaignEffectProofInvalid) {
		t.Fatalf("expected tamper rejection, got %v", err)
	}
}
