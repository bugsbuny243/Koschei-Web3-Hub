package services

import (
	"errors"
	"strings"

	"koschei/api/internal/executioncontainment"
	"koschei/api/internal/executionproof"
)

const AgentMaterialActionBindingContractVersionV1 = "koschei-agent-material-action-binding-v1"

// AgentMaterialActionBindingV1 proves that independently valid authorization
// and execution artifacts refer to the same material action. It is evidence
// composition only: a valid binding does not grant authority, imply safety, or
// prove that an intended business result occurred.
type AgentMaterialActionBindingV1 struct {
	ContractVersion           string `json:"contract_version"`
	ID                        string `json:"id"`
	ChainID                   uint64 `json:"chain_id"`
	Target                    string `json:"target"`
	IntentSHA256              string `json:"intent_sha256"`
	ApprovedPayloadSHA256     string `json:"approved_payload_sha256"`
	CandidatePayloadSHA256    string `json:"candidate_payload_sha256"`
	ActionSHA256              string `json:"action_sha256"`
	InvariantSetSHA256        string `json:"invariant_set_sha256"`
	AuthorizationPolicySHA256 string `json:"authorization_policy_sha256"`
	RuntimePolicySHA256       string `json:"runtime_policy_sha256"`
	AuthorizationRef          string `json:"authorization_ref"`
	ExecutionProofSHA256      string `json:"execution_proof_sha256"`
	ContainmentReceiptSHA256  string `json:"containment_receipt_sha256"`
}

// BuildAgentMaterialActionBindingV1 re-verifies both native artifacts and then
// requires exact cross-artifact identity for chain, target, approved intent,
// approved/candidate payloads and invariant set. This prevents a valid permit
// for action A from being composed with a valid execution receipt for action B.
func BuildAgentMaterialActionBindingV1(proof executionproof.Proof, receipt executioncontainment.Receipt) (AgentMaterialActionBindingV1, error) {
	if !agentMaterialExecutionProofValid(proof) {
		return AgentMaterialActionBindingV1{}, errors.New("execution proof failed native recomputation")
	}
	if !executioncontainment.Verify(receipt) {
		return AgentMaterialActionBindingV1{}, errors.New("containment receipt failed native recomputation")
	}

	proofDigest := normalizeAgentSHA256(proof.EnvelopeSHA256)
	receiptDigest := normalizeAgentSHA256(receipt.ReceiptSHA256)
	approvedIntent := normalizeAgentSHA256(strings.TrimPrefix(strings.TrimSpace(proof.Envelope.Authorization.ApprovedSigningRequestID), "0x"))
	approvedPayload := normalizeAgentSHA256(proof.Envelope.Payload.ApprovedCalldataSHA256)
	candidatePayload := normalizeAgentSHA256(proof.Envelope.Payload.GeneratedCalldataSHA256)
	actionDigest := normalizeAgentSHA256(receipt.Input.ActionSHA256)
	invariantDigest := normalizeAgentSHA256(proof.Envelope.Simulation.InvariantSetSHA256)
	authorizationPolicy := normalizeAgentSHA256(proof.Envelope.Authorization.SigningPolicySHA256)
	runtimePolicy := normalizeAgentSHA256(proof.Envelope.Runtime.PolicySHA256)
	if proofDigest == "" || receiptDigest == "" || approvedIntent == "" || approvedPayload == "" || candidatePayload == "" || actionDigest == "" || invariantDigest == "" || authorizationPolicy == "" || runtimePolicy == "" {
		return AgentMaterialActionBindingV1{}, errors.New("material action binding requires complete sha256 evidence")
	}

	target := strings.TrimSpace(proof.Envelope.Payload.Target)
	if proof.Envelope.Payload.ChainID == 0 || target == "" ||
		receipt.Input.ChainID != proof.Envelope.Payload.ChainID ||
		!strings.EqualFold(strings.TrimSpace(receipt.Input.Target), target) ||
		normalizeAgentSHA256(receipt.Input.ApprovedIntentSHA256) != approvedIntent ||
		normalizeAgentSHA256(receipt.Input.ApprovedPayloadSHA256) != approvedPayload ||
		normalizeAgentSHA256(receipt.Input.CandidatePayloadSHA256) != candidatePayload ||
		normalizeAgentSHA256(receipt.Input.InvariantSetSHA256) != invariantDigest {
		return AgentMaterialActionBindingV1{}, errors.New("authorization and execution artifacts do not describe the same material action")
	}

	authorizationRef := strings.TrimSpace(proof.Envelope.Authorization.ApprovedSigningRequestID)
	canonical := strings.Join([]string{
		AgentMaterialActionBindingContractVersionV1,
		strings.TrimSpace(target),
		approvedIntent,
		approvedPayload,
		candidatePayload,
		actionDigest,
		invariantDigest,
		authorizationPolicy,
		runtimePolicy,
		proofDigest,
		receiptDigest,
	}, "|")
	return AgentMaterialActionBindingV1{
		ContractVersion:           AgentMaterialActionBindingContractVersionV1,
		ID:                        intelligenceTypedStableID("kamb_", canonical),
		ChainID:                   proof.Envelope.Payload.ChainID,
		Target:                    target,
		IntentSHA256:              approvedIntent,
		ApprovedPayloadSHA256:     approvedPayload,
		CandidatePayloadSHA256:    candidatePayload,
		ActionSHA256:              actionDigest,
		InvariantSetSHA256:        invariantDigest,
		AuthorizationPolicySHA256: authorizationPolicy,
		RuntimePolicySHA256:       runtimePolicy,
		AuthorizationRef:          authorizationRef,
		ExecutionProofSHA256:      proofDigest,
		ContainmentReceiptSHA256:  receiptDigest,
	}, nil
}

func agentMaterialExecutionProofValid(proof executionproof.Proof) bool {
	recomputed, err := executionproof.Evaluate(proof.Envelope)
	if err != nil || !strings.EqualFold(recomputed.EnvelopeSHA256, proof.EnvelopeSHA256) || recomputed.Evaluation.Decision != proof.Evaluation.Decision || len(recomputed.Evaluation.Reasons) != len(proof.Evaluation.Reasons) {
		return false
	}
	for index := range recomputed.Evaluation.Reasons {
		if recomputed.Evaluation.Reasons[index] != proof.Evaluation.Reasons[index] {
			return false
		}
	}
	return true
}
