package services

import (
	"strings"
	"testing"

	"koschei/api/internal/executioncontainment"
	"koschei/api/internal/executionproof"
)

func TestBuildAgentMaterialActionBindingV1(t *testing.T) {
	proof, receipt := agentMaterialActionFixture(t)
	binding, err := BuildAgentMaterialActionBindingV1(proof, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if binding.ContractVersion != AgentMaterialActionBindingContractVersionV1 || binding.ID == "" {
		t.Fatalf("binding identity incomplete: %#v", binding)
	}
	if binding.ActionSHA256 != receipt.Input.ActionSHA256 || binding.IntentSHA256 != receipt.Input.ApprovedIntentSHA256 {
		t.Fatalf("material action identity drifted: %#v", binding)
	}
	if binding.ExecutionProofSHA256 != proof.EnvelopeSHA256 || binding.ContainmentReceiptSHA256 != receipt.ReceiptSHA256 {
		t.Fatalf("native evidence digests drifted: %#v", binding)
	}
}

func TestBuildAgentMaterialActionBindingV1RejectsTargetSubstitution(t *testing.T) {
	proof, receipt := agentMaterialActionFixture(t)
	receipt.Input.Target = "0x2222222222222222222222222222222222222222"
	var err error
	receipt, err = executioncontainment.Evaluate(receipt.Input, receipt.Observation)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildAgentMaterialActionBindingV1(proof, receipt); err == nil {
		t.Fatal("authorization for one target was composed with execution evidence for another target")
	}
}

func TestBuildAgentMaterialActionBindingV1RejectsPayloadSubstitution(t *testing.T) {
	proof, receipt := agentMaterialActionFixture(t)
	receipt.Input.CandidatePayloadSHA256 = strings.Repeat("8", 64)
	var err error
	receipt, err = executioncontainment.Evaluate(receipt.Input, receipt.Observation)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildAgentMaterialActionBindingV1(proof, receipt); err == nil {
		t.Fatal("authorization and execution evidence with different payloads were composed")
	}
}

func TestBuildAgentMaterialActionBindingV1RejectsTamperedProof(t *testing.T) {
	proof, receipt := agentMaterialActionFixture(t)
	proof.Envelope.Authorization.SigningPolicySHA256 = strings.Repeat("8", 64)
	if _, err := BuildAgentMaterialActionBindingV1(proof, receipt); err == nil {
		t.Fatal("tampered execution proof was accepted")
	}
}

func agentMaterialActionFixture(t *testing.T) (executionproof.Proof, executioncontainment.Receipt) {
	t.Helper()
	target := "0x1111111111111111111111111111111111111111"
	intent := strings.Repeat("c", 64)
	payload := strings.Repeat("d", 64)
	invariants := strings.Repeat("f", 64)
	proof, err := executionproof.Evaluate(executionproof.Envelope{
		Source: executionproof.SourceEvidence{CommitID: strings.Repeat("1", 40), TreeID: strings.Repeat("2", 40)},
		Build: executionproof.BuildEvidence{
			ToolchainSHA256:        strings.Repeat("3", 64),
			ApprovedArtifactSHA256: strings.Repeat("4", 64),
			BuiltArtifactSHA256:    strings.Repeat("4", 64),
		},
		Runtime: executionproof.RuntimeEvidence{
			ObservedArtifactSHA256: strings.Repeat("4", 64),
			PolicySHA256:           strings.Repeat("5", 64),
		},
		Payload: executionproof.PayloadEvidence{
			ChainID:                 1,
			Target:                  target,
			ApprovedCalldataSHA256:  payload,
			GeneratedCalldataSHA256: payload,
			GeneratorSHA256:         strings.Repeat("6", 64),
		},
		Simulation: executionproof.SimulationEvidence{InvariantSetSHA256: invariants, Result: "PASS"},
		Authorization: executionproof.AuthorizationEvidence{
			SigningPolicySHA256:      strings.Repeat("7", 64),
			ApprovedSigningRequestID: intent,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := executioncontainment.Evaluate(executioncontainment.CellInput{
		ChainID:                1,
		BlockNumber:            100,
		BlockHash:              strings.Repeat("a", 64),
		Target:                 target,
		ApprovedIntentSHA256:   intent,
		CandidateIntentSHA256:  intent,
		ApprovedPayloadSHA256:  payload,
		CandidatePayloadSHA256: payload,
		ActionSHA256:           strings.Repeat("e", 64),
		InvariantSetSHA256:     invariants,
		ApprovedRunnerSHA256:   strings.Repeat("9", 64),
	}, executioncontainment.Observation{
		BackendAvailable:           true,
		ObservedChainID:            1,
		ObservedBlockNumber:        100,
		ObservedBlockHash:          strings.Repeat("a", 64),
		ObservedRunnerSHA256:       strings.Repeat("9", 64),
		PreStateSHA256:             strings.Repeat("1", 64),
		PostStateSHA256:            strings.Repeat("2", 64),
		EffectSetSHA256:            strings.Repeat("3", 64),
		AuthorityPreserved:         true,
		AssetBoundsPreserved:       true,
		CodeIntegrityPreserved:     true,
		ExecutionPathFullyObserved: true,
		InvariantsPass:             true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return proof, receipt
}
