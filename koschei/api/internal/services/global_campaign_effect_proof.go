package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"koschei/api/internal/executioncontainment"
)

const GlobalCampaignEffectProofSchemaVersion = "koschei.response-effect-proof.v1"

type GlobalCampaignEffectState string

const (
	GlobalCampaignEffectStateExecuted GlobalCampaignEffectState = "executed"
	GlobalCampaignEffectStateVerified GlobalCampaignEffectState = "verified"
	GlobalCampaignEffectStateFailed   GlobalCampaignEffectState = "failed"
)

var (
	ErrGlobalCampaignEffectProofInvalid  = errors.New("global campaign response effect proof is invalid")
	ErrGlobalCampaignEffectProofMismatch = errors.New("global campaign response effect proof binding mismatch")
)

// GlobalCampaignResponseEffectProof binds explicit authorization, safe preflight,
// concrete execution evidence, and independent post-effect verification. It never
// grants authority and cannot be constructed from campaign/model opinion alone.
type GlobalCampaignResponseEffectProof struct {
	SchemaVersion              string                    `json:"schema_version"`
	CampaignRef                string                    `json:"campaign_ref"`
	CampaignRevision           uint64                    `json:"campaign_revision"`
	CampaignEvidenceHashSHA256 string                    `json:"campaign_evidence_hash_sha256"`
	ProposalHashSHA256         string                    `json:"proposal_hash_sha256"`
	AuthorizationHashSHA256    string                    `json:"authorization_hash_sha256"`
	ActionSHA256               string                    `json:"action_sha256"`
	PreflightReceiptSHA256     string                    `json:"preflight_receipt_sha256"`
	ExecutionRef               string                    `json:"execution_ref"`
	ExecutionEvidenceSHA256    string                    `json:"execution_evidence_sha256"`
	ExecutorRef                string                    `json:"executor_ref"`
	ExecutedAt                 time.Time                 `json:"executed_at"`
	PreStateSHA256             string                    `json:"pre_state_sha256"`
	PostStateSHA256            string                    `json:"post_state_sha256"`
	EffectSetSHA256            string                    `json:"effect_set_sha256"`
	VerifierRef                string                    `json:"verifier_ref"`
	VerificationEvidenceSHA256 string                    `json:"verification_evidence_sha256"`
	VerifiedAt                 time.Time                 `json:"verified_at"`
	State                      GlobalCampaignEffectState `json:"state"`
	EffectVerified             bool                      `json:"effect_verified"`
	AuthorityPreserved         bool                      `json:"authority_preserved"`
	AssetBoundsPreserved       bool                      `json:"asset_bounds_preserved"`
	CodeIntegrityPreserved     bool                      `json:"code_integrity_preserved"`
	ExecutionPathObserved      bool                      `json:"execution_path_observed"`
	IndependentVerifier        bool                      `json:"independent_verifier"`
	ContainmentProven          bool                      `json:"containment_proven"`
	ProofHashSHA256            string                    `json:"proof_hash_sha256"`
}

func NewGlobalCampaignResponseEffectProof(
	campaign GlobalCampaign,
	fabric FabricCampaignEvidenceContract,
	proposal GlobalCampaignResponseProposal,
	authorization GlobalCampaignResponseAuthorization,
	preflight executioncontainment.Receipt,
	proof GlobalCampaignResponseEffectProof,
	now time.Time,
) (GlobalCampaignResponseEffectProof, error) {
	if err := ValidateGlobalCampaignResponseAuthorization(campaign, fabric, proposal, authorization, now); err != nil {
		return GlobalCampaignResponseEffectProof{}, err
	}
	if !executioncontainment.Verify(preflight) || preflight.Decision != executioncontainment.DecisionRelease {
		return GlobalCampaignResponseEffectProof{}, ErrGlobalCampaignEffectProofInvalid
	}
	if normalizeResponseDigest(preflight.Input.ActionSHA256) != proposal.ActionSHA256 {
		return GlobalCampaignResponseEffectProof{}, ErrGlobalCampaignEffectProofMismatch
	}

	proof.SchemaVersion = GlobalCampaignEffectProofSchemaVersion
	proof.CampaignRef = campaign.CampaignRef
	proof.CampaignRevision = campaign.Revision
	proof.CampaignEvidenceHashSHA256 = campaign.EvidenceHashSHA256
	proof.ProposalHashSHA256 = proposal.ProposalHashSHA256
	proof.AuthorizationHashSHA256 = authorization.AuthorizationHashSHA256
	proof.ActionSHA256 = proposal.ActionSHA256
	proof.PreflightReceiptSHA256 = normalizeResponseDigest(preflight.ReceiptSHA256)
	proof.ExecutionRef = strings.TrimSpace(proof.ExecutionRef)
	proof.ExecutionEvidenceSHA256 = normalizeResponseDigest(proof.ExecutionEvidenceSHA256)
	proof.ExecutorRef = strings.TrimSpace(proof.ExecutorRef)
	proof.ExecutedAt = proof.ExecutedAt.UTC()
	proof.PreStateSHA256 = normalizeResponseDigest(proof.PreStateSHA256)
	proof.PostStateSHA256 = normalizeResponseDigest(proof.PostStateSHA256)
	proof.EffectSetSHA256 = normalizeResponseDigest(proof.EffectSetSHA256)
	proof.VerifierRef = strings.TrimSpace(proof.VerifierRef)
	proof.VerificationEvidenceSHA256 = normalizeResponseDigest(proof.VerificationEvidenceSHA256)
	proof.VerifiedAt = proof.VerifiedAt.UTC()
	proof.ContainmentProven = false
	proof.ProofHashSHA256 = ""

	if proof.EffectVerified && proof.IndependentVerifier && proof.AuthorityPreserved && proof.AssetBoundsPreserved && proof.CodeIntegrityPreserved && proof.ExecutionPathObserved {
		proof.State = GlobalCampaignEffectStateVerified
		proof.ContainmentProven = true
	} else if proof.ExecutedAt.IsZero() || proof.ExecutionRef == "" || !validResponseDigest(proof.ExecutionEvidenceSHA256) {
		proof.State = GlobalCampaignEffectStateFailed
	} else {
		proof.State = GlobalCampaignEffectStateExecuted
	}

	if err := ValidateGlobalCampaignResponseEffectProof(campaign, proposal, authorization, preflight, proof); err != nil {
		return GlobalCampaignResponseEffectProof{}, err
	}
	proof.ProofHashSHA256 = hashGlobalCampaignResponseEffectProof(proof)
	return proof, nil
}

func ValidateGlobalCampaignResponseEffectProof(
	campaign GlobalCampaign,
	proposal GlobalCampaignResponseProposal,
	authorization GlobalCampaignResponseAuthorization,
	preflight executioncontainment.Receipt,
	proof GlobalCampaignResponseEffectProof,
) error {
	if proof.SchemaVersion != GlobalCampaignEffectProofSchemaVersion ||
		proof.CampaignRef != campaign.CampaignRef || proof.CampaignRevision != campaign.Revision ||
		proof.CampaignEvidenceHashSHA256 != campaign.EvidenceHashSHA256 ||
		proof.ProposalHashSHA256 != proposal.ProposalHashSHA256 ||
		proof.AuthorizationHashSHA256 != authorization.AuthorizationHashSHA256 ||
		normalizeResponseDigest(proof.ActionSHA256) != proposal.ActionSHA256 ||
		!executioncontainment.Verify(preflight) || preflight.Decision != executioncontainment.DecisionRelease ||
		normalizeResponseDigest(preflight.Input.ActionSHA256) != proposal.ActionSHA256 ||
		normalizeResponseDigest(proof.PreflightReceiptSHA256) != normalizeResponseDigest(preflight.ReceiptSHA256) {
		return ErrGlobalCampaignEffectProofMismatch
	}

	if strings.TrimSpace(proof.ExecutionRef) == "" || strings.TrimSpace(proof.ExecutorRef) == "" ||
		!validResponseDigest(proof.ExecutionEvidenceSHA256) || proof.ExecutedAt.IsZero() ||
		!validResponseDigest(proof.PreStateSHA256) || !validResponseDigest(proof.PostStateSHA256) ||
		!validResponseDigest(proof.EffectSetSHA256) || strings.TrimSpace(proof.VerifierRef) == "" ||
		!validResponseDigest(proof.VerificationEvidenceSHA256) || proof.VerifiedAt.IsZero() || proof.VerifiedAt.Before(proof.ExecutedAt) ||
		strings.EqualFold(strings.TrimSpace(proof.ExecutorRef), strings.TrimSpace(proof.VerifierRef)) {
		return ErrGlobalCampaignEffectProofInvalid
	}

	expectedVerified := proof.EffectVerified && proof.IndependentVerifier && proof.AuthorityPreserved && proof.AssetBoundsPreserved && proof.CodeIntegrityPreserved && proof.ExecutionPathObserved
	if proof.ContainmentProven != expectedVerified {
		return ErrGlobalCampaignEffectProofInvalid
	}
	if expectedVerified && proof.State != GlobalCampaignEffectStateVerified {
		return ErrGlobalCampaignEffectProofInvalid
	}
	if !expectedVerified && proof.State == GlobalCampaignEffectStateVerified {
		return ErrGlobalCampaignEffectProofInvalid
	}
	if proof.ProofHashSHA256 != "" && hashGlobalCampaignResponseEffectProof(proof) != proof.ProofHashSHA256 {
		return ErrGlobalCampaignEffectProofInvalid
	}
	return nil
}

func hashGlobalCampaignResponseEffectProof(proof GlobalCampaignResponseEffectProof) string {
	proof.ProofHashSHA256 = ""
	payload, err := json.Marshal(proof)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}
