package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"koschei/api/internal/executioncontainment"
	"koschei/api/internal/securityevidence"
)

const (
	GlobalCampaignResponseEffectProofSchemaVersion = "koschei.response-effect-proof.v1"
	GlobalCampaignResponseEffectSubjectType        = "campaign_response_effect"
	GlobalCampaignResponseEffectFindingID          = "campaign-response-effect"
	GlobalCampaignResponseEffectFindingKind        = "campaign_response_effect"
)

var (
	ErrGlobalCampaignResponseEffectInvalid     = errors.New("global campaign response effect proof is invalid")
	ErrGlobalCampaignResponseEffectMismatch    = errors.New("global campaign response effect proof binding mismatch")
	ErrGlobalCampaignResponseEffectUntrusted   = errors.New("global campaign response effect evidence is untrusted")
	ErrGlobalCampaignResponseEffectStale       = errors.New("global campaign response effect evidence is stale")
	ErrGlobalCampaignResponseEffectUnavailable = errors.New("global campaign response effect is not independently verified")
)

// GlobalCampaignResponseEffectTrust is supplied by the consuming control plane.
// Producer key material is never accepted from the evidence event itself.
type GlobalCampaignResponseEffectTrust struct {
	Producer      string
	PublicKey     string
	MaxAge        time.Duration
	MaxFutureSkew time.Duration
}

// GlobalCampaignResponseEffectBinding pins the exact authorization, action and
// recomputable containment receipt to an independently observed execution ref
// and its pre/post/effect state digests.
type GlobalCampaignResponseEffectBinding struct {
	SchemaVersion              string `json:"schema_version"`
	CampaignRef                string `json:"campaign_ref"`
	CampaignRevision           uint64 `json:"campaign_revision"`
	CampaignEvidenceHashSHA256 string `json:"campaign_evidence_hash_sha256"`
	FabricContractHashSHA256   string `json:"fabric_contract_hash_sha256"`
	ProposalHashSHA256         string `json:"proposal_hash_sha256"`
	AuthorizationHashSHA256    string `json:"authorization_hash_sha256"`
	PolicyHashSHA256           string `json:"policy_hash_sha256"`
	Network                    string `json:"network"`
	Target                     string `json:"target"`
	ActionSHA256               string `json:"action_sha256"`
	ContainmentReceiptSHA256   string `json:"containment_receipt_sha256"`
	ContainmentInputSHA256     string `json:"containment_input_sha256"`
	ExecutionRef               string `json:"execution_ref"`
	PreStateSHA256             string `json:"pre_state_sha256"`
	PostStateSHA256            string `json:"post_state_sha256"`
	EffectSetSHA256            string `json:"effect_set_sha256"`
}

// GlobalCampaignResponseEffectProof records that a trusted independent producer
// authenticated evidence for one exact response effect. It does not grant
// execution authority and does not claim that Koschei forwarded a production
// transaction; production forwarding remains a separate deployment fact.
type GlobalCampaignResponseEffectProof struct {
	SchemaVersion             string                              `json:"schema_version"`
	Binding                   GlobalCampaignResponseEffectBinding `json:"binding"`
	BindingHashSHA256         string                              `json:"binding_hash_sha256"`
	EvidenceEventSHA256       string                              `json:"evidence_event_sha256"`
	VerifierProducer          string                              `json:"verifier_producer"`
	VerifiedAt                time.Time                           `json:"verified_at"`
	ContainmentEffectVerified bool                                `json:"containment_effect_verified"`
	ExecutionAuthority        bool                                `json:"execution_authority"`
	ProductionForwardingClaim bool                                `json:"production_forwarding_claim"`
	ProofHashSHA256           string                              `json:"proof_hash_sha256"`
}

func BuildGlobalCampaignResponseEffectBinding(
	campaign GlobalCampaign,
	fabric FabricCampaignEvidenceContract,
	proposal GlobalCampaignResponseProposal,
	authorization GlobalCampaignResponseAuthorization,
	receipt executioncontainment.Receipt,
	executionRef string,
	authorizationCheckAt time.Time,
) (GlobalCampaignResponseEffectBinding, error) {
	if err := ValidateGlobalCampaignResponseAuthorization(campaign, fabric, proposal, authorization, authorizationCheckAt); err != nil {
		return GlobalCampaignResponseEffectBinding{}, fmt.Errorf("%w: authorization: %v", ErrGlobalCampaignResponseEffectInvalid, err)
	}
	if !executioncontainment.Verify(receipt) || receipt.Decision != executioncontainment.DecisionRelease || !receipt.Observation.BackendAvailable {
		return GlobalCampaignResponseEffectBinding{}, ErrGlobalCampaignResponseEffectUnavailable
	}

	proposalAction := responseDigestHex(proposal.ActionSHA256)
	if proposalAction == "" || !strings.EqualFold(proposalAction, strings.TrimSpace(receipt.Input.ActionSHA256)) ||
		strings.TrimSpace(proposal.Target) != strings.TrimSpace(receipt.Input.Target) {
		return GlobalCampaignResponseEffectBinding{}, ErrGlobalCampaignResponseEffectMismatch
	}

	binding := GlobalCampaignResponseEffectBinding{
		SchemaVersion:              GlobalCampaignResponseEffectProofSchemaVersion,
		CampaignRef:                campaign.CampaignRef,
		CampaignRevision:           campaign.Revision,
		CampaignEvidenceHashSHA256: responseDigestHex(campaign.EvidenceHashSHA256),
		FabricContractHashSHA256:   responseDigestHex(fabric.ContractHashSHA256),
		ProposalHashSHA256:         responseDigestHex(proposal.ProposalHashSHA256),
		AuthorizationHashSHA256:    responseDigestHex(authorization.AuthorizationHashSHA256),
		PolicyHashSHA256:           responseDigestHex(proposal.PolicyHashSHA256),
		Network:                    strings.ToLower(strings.TrimSpace(proposal.Network)),
		Target:                     strings.TrimSpace(proposal.Target),
		ActionSHA256:               proposalAction,
		ContainmentReceiptSHA256:   responseDigestHex(receipt.ReceiptSHA256),
		ContainmentInputSHA256:     responseDigestHex(receipt.InputSHA256),
		ExecutionRef:               strings.TrimSpace(executionRef),
		PreStateSHA256:             responseDigestHex(receipt.Observation.PreStateSHA256),
		PostStateSHA256:            responseDigestHex(receipt.Observation.PostStateSHA256),
		EffectSetSHA256:            responseDigestHex(receipt.Observation.EffectSetSHA256),
	}
	if err := ValidateGlobalCampaignResponseEffectBinding(binding); err != nil {
		return GlobalCampaignResponseEffectBinding{}, err
	}
	return binding, nil
}

func ValidateGlobalCampaignResponseEffectBinding(binding GlobalCampaignResponseEffectBinding) error {
	if binding.SchemaVersion != GlobalCampaignResponseEffectProofSchemaVersion ||
		strings.TrimSpace(binding.CampaignRef) == "" || binding.CampaignRevision == 0 ||
		strings.TrimSpace(binding.Network) == "" || strings.TrimSpace(binding.Target) == "" ||
		strings.TrimSpace(binding.ExecutionRef) == "" {
		return ErrGlobalCampaignResponseEffectInvalid
	}
	for _, digest := range []string{
		binding.CampaignEvidenceHashSHA256,
		binding.FabricContractHashSHA256,
		binding.ProposalHashSHA256,
		binding.AuthorizationHashSHA256,
		binding.PolicyHashSHA256,
		binding.ActionSHA256,
		binding.ContainmentReceiptSHA256,
		binding.ContainmentInputSHA256,
		binding.PreStateSHA256,
		binding.PostStateSHA256,
		binding.EffectSetSHA256,
	} {
		if responseDigestHex(digest) == "" {
			return ErrGlobalCampaignResponseEffectInvalid
		}
	}
	return nil
}

func GlobalCampaignResponseEffectBindingDigest(binding GlobalCampaignResponseEffectBinding) (string, error) {
	if err := ValidateGlobalCampaignResponseEffectBinding(binding); err != nil {
		return "", err
	}
	payload, err := json.Marshal(binding)
	if err != nil {
		return "", fmt.Errorf("marshal response effect binding: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func BuildGlobalCampaignResponseEffectProof(
	campaign GlobalCampaign,
	fabric FabricCampaignEvidenceContract,
	proposal GlobalCampaignResponseProposal,
	authorization GlobalCampaignResponseAuthorization,
	receipt executioncontainment.Receipt,
	executionRef string,
	event securityevidence.Event,
	trust GlobalCampaignResponseEffectTrust,
	verifiedAt time.Time,
) (GlobalCampaignResponseEffectProof, error) {
	verifiedAt = verifiedAt.UTC()
	if verifiedAt.IsZero() {
		return GlobalCampaignResponseEffectProof{}, ErrGlobalCampaignResponseEffectInvalid
	}
	trust.Producer = strings.TrimSpace(trust.Producer)
	trust.PublicKey = strings.TrimSpace(trust.PublicKey)
	if trust.Producer == "" || trust.PublicKey == "" || trust.MaxAge <= 0 || trust.MaxFutureSkew < 0 ||
		strings.EqualFold(trust.Producer, strings.TrimSpace(authorization.AuthorizerRef)) {
		return GlobalCampaignResponseEffectProof{}, ErrGlobalCampaignResponseEffectUntrusted
	}
	if err := event.VerifyEd25519(trust.Producer, trust.PublicKey); err != nil {
		return GlobalCampaignResponseEffectProof{}, fmt.Errorf("%w: %v", ErrGlobalCampaignResponseEffectUntrusted, err)
	}
	canonicalEvent, err := event.Canonical()
	if err != nil {
		return GlobalCampaignResponseEffectProof{}, fmt.Errorf("%w: %v", ErrGlobalCampaignResponseEffectUntrusted, err)
	}

	if err := validateResponseEffectWindow(canonicalEvent, authorization, trust, verifiedAt); err != nil {
		return GlobalCampaignResponseEffectProof{}, err
	}
	authorizationCheckAt := time.UnixMilli(canonicalEvent.Window.FromUnixMS).UTC()
	binding, err := BuildGlobalCampaignResponseEffectBinding(campaign, fabric, proposal, authorization, receipt, executionRef, authorizationCheckAt)
	if err != nil {
		return GlobalCampaignResponseEffectProof{}, err
	}
	bindingDigest, err := GlobalCampaignResponseEffectBindingDigest(binding)
	if err != nil {
		return GlobalCampaignResponseEffectProof{}, err
	}
	if !responseEffectEventMatches(canonicalEvent, binding, bindingDigest) {
		return GlobalCampaignResponseEffectProof{}, ErrGlobalCampaignResponseEffectMismatch
	}

	proof := GlobalCampaignResponseEffectProof{
		SchemaVersion:             GlobalCampaignResponseEffectProofSchemaVersion,
		Binding:                   binding,
		BindingHashSHA256:         bindingDigest,
		EvidenceEventSHA256:       responseDigestHex(event.EventSHA256),
		VerifierProducer:          trust.Producer,
		VerifiedAt:                verifiedAt,
		ContainmentEffectVerified: true,
		ExecutionAuthority:        false,
		ProductionForwardingClaim: false,
	}
	if proof.EvidenceEventSHA256 == "" {
		return GlobalCampaignResponseEffectProof{}, ErrGlobalCampaignResponseEffectInvalid
	}
	proof.ProofHashSHA256 = hashGlobalCampaignResponseEffectProof(proof)
	return proof, nil
}

func ValidateGlobalCampaignResponseEffectProof(
	campaign GlobalCampaign,
	fabric FabricCampaignEvidenceContract,
	proposal GlobalCampaignResponseProposal,
	authorization GlobalCampaignResponseAuthorization,
	receipt executioncontainment.Receipt,
	event securityevidence.Event,
	trust GlobalCampaignResponseEffectTrust,
	proof GlobalCampaignResponseEffectProof,
) error {
	if proof.SchemaVersion != GlobalCampaignResponseEffectProofSchemaVersion ||
		!proof.ContainmentEffectVerified || proof.ExecutionAuthority || proof.ProductionForwardingClaim ||
		responseDigestHex(proof.BindingHashSHA256) == "" || responseDigestHex(proof.EvidenceEventSHA256) == "" ||
		responseDigestHex(proof.ProofHashSHA256) == "" || hashGlobalCampaignResponseEffectProof(proof) != proof.ProofHashSHA256 {
		return ErrGlobalCampaignResponseEffectInvalid
	}

	rebuilt, err := BuildGlobalCampaignResponseEffectProof(
		campaign,
		fabric,
		proposal,
		authorization,
		receipt,
		proof.Binding.ExecutionRef,
		event,
		trust,
		proof.VerifiedAt,
	)
	if err != nil {
		return err
	}
	if rebuilt.ProofHashSHA256 != proof.ProofHashSHA256 || rebuilt.BindingHashSHA256 != proof.BindingHashSHA256 || rebuilt.EvidenceEventSHA256 != proof.EvidenceEventSHA256 {
		return ErrGlobalCampaignResponseEffectMismatch
	}
	return nil
}

func validateResponseEffectWindow(event securityevidence.Event, authorization GlobalCampaignResponseAuthorization, trust GlobalCampaignResponseEffectTrust, verifiedAt time.Time) error {
	windowStart := time.UnixMilli(event.Window.FromUnixMS).UTC()
	windowEnd := time.UnixMilli(event.Window.ToUnixMS).UTC()
	if windowStart.Before(authorization.AuthorizedAt.UTC()) || !windowStart.Before(authorization.ExpiresAt.UTC()) || windowEnd.After(authorization.ExpiresAt.UTC()) {
		return ErrGlobalCampaignResponseEffectMismatch
	}
	if windowEnd.After(verifiedAt.Add(trust.MaxFutureSkew)) || verifiedAt.Sub(windowStart) > trust.MaxAge {
		return ErrGlobalCampaignResponseEffectStale
	}
	return nil
}

func responseEffectEventMatches(event securityevidence.Event, binding GlobalCampaignResponseEffectBinding, bindingDigest string) bool {
	if event.Subject.Chain != binding.Network || event.Subject.Type != GlobalCampaignResponseEffectSubjectType || event.Subject.ID != binding.ExecutionRef {
		return false
	}
	expectedSources := []string{
		binding.ActionSHA256,
		binding.AuthorizationHashSHA256,
		binding.ContainmentReceiptSHA256,
		binding.ProposalHashSHA256,
	}
	sort.Strings(expectedSources)
	if len(event.SourceDigests) != len(expectedSources) {
		return false
	}
	for index := range expectedSources {
		if event.SourceDigests[index] != expectedSources[index] {
			return false
		}
	}
	for _, finding := range event.Findings {
		if finding.ID == GlobalCampaignResponseEffectFindingID &&
			finding.Kind == GlobalCampaignResponseEffectFindingKind &&
			finding.State == securityevidence.StateVerified &&
			strings.EqualFold(finding.EvidenceSHA256, bindingDigest) {
			return true
		}
	}
	return false
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

func responseDigestHex(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "sha256:")
	if len(value) != sha256.Size*2 {
		return ""
	}
	if _, err := hex.DecodeString(value); err != nil {
		return ""
	}
	return value
}
