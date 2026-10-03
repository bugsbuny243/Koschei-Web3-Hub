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
	GlobalCampaignEffectAttestationSchemaVersion = "koschei.response-effect-attestation.v1"
	GlobalCampaignEffectAttestationSubjectType   = "campaign_response_effect"
	GlobalCampaignEffectAttestationFindingID     = "campaign-response-effect-authenticated"
	GlobalCampaignEffectAttestationFindingKind   = "campaign_response_effect_authenticated"
)

var (
	ErrGlobalCampaignEffectAttestationInvalid   = errors.New("global campaign effect attestation is invalid")
	ErrGlobalCampaignEffectAttestationUntrusted = errors.New("global campaign effect attestation producer is untrusted")
	ErrGlobalCampaignEffectAttestationMismatch  = errors.New("global campaign effect attestation binding mismatch")
	ErrGlobalCampaignEffectAttestationStale     = errors.New("global campaign effect attestation is stale")
)

type GlobalCampaignEffectAttestationTrust struct {
	Producer      string
	PublicKey     string
	MaxAge        time.Duration
	MaxFutureSkew time.Duration
}

type GlobalCampaignEffectAttestationBinding struct {
	SchemaVersion              string `json:"schema_version"`
	CampaignRef                string `json:"campaign_ref"`
	CampaignRevision           uint64 `json:"campaign_revision"`
	CampaignEvidenceHashSHA256 string `json:"campaign_evidence_hash_sha256"`
	ProposalHashSHA256         string `json:"proposal_hash_sha256"`
	AuthorizationHashSHA256    string `json:"authorization_hash_sha256"`
	ActionSHA256               string `json:"action_sha256"`
	EffectProofHashSHA256      string `json:"effect_proof_hash_sha256"`
	ExecutionRef               string `json:"execution_ref"`
	ExecutionEvidenceSHA256    string `json:"execution_evidence_sha256"`
	VerificationEvidenceSHA256 string `json:"verification_evidence_sha256"`
}

type GlobalCampaignEffectAttestation struct {
	SchemaVersion       string    `json:"schema_version"`
	Binding             GlobalCampaignEffectAttestationBinding `json:"binding"`
	BindingHashSHA256   string    `json:"binding_hash_sha256"`
	EvidenceEventSHA256 string    `json:"evidence_event_sha256"`
	Producer            string    `json:"producer"`
	AuthenticatedAt     time.Time `json:"authenticated_at"`
	Authenticated       bool      `json:"authenticated"`
	ExecutionAuthority  bool      `json:"execution_authority"`
	SubmissionAuthority bool      `json:"submission_authority"`
	AttestationSHA256   string    `json:"attestation_sha256"`
}

func BuildGlobalCampaignEffectAttestationBinding(
	campaign GlobalCampaign,
	proposal GlobalCampaignResponseProposal,
	authorization GlobalCampaignResponseAuthorization,
	proof GlobalCampaignResponseEffectProof,
) (GlobalCampaignEffectAttestationBinding, error) {
	binding := GlobalCampaignEffectAttestationBinding{
		SchemaVersion:              GlobalCampaignEffectAttestationSchemaVersion,
		CampaignRef:                campaign.CampaignRef,
		CampaignRevision:           campaign.Revision,
		CampaignEvidenceHashSHA256: normalizeResponseDigest(campaign.EvidenceHashSHA256),
		ProposalHashSHA256:         normalizeResponseDigest(proposal.ProposalHashSHA256),
		AuthorizationHashSHA256:    normalizeResponseDigest(authorization.AuthorizationHashSHA256),
		ActionSHA256:               normalizeResponseDigest(proposal.ActionSHA256),
		EffectProofHashSHA256:      normalizeResponseDigest(proof.ProofHashSHA256),
		ExecutionRef:               strings.TrimSpace(proof.ExecutionRef),
		ExecutionEvidenceSHA256:    normalizeResponseDigest(proof.ExecutionEvidenceSHA256),
		VerificationEvidenceSHA256: normalizeResponseDigest(proof.VerificationEvidenceSHA256),
	}
	if binding.CampaignRef == "" || binding.CampaignRevision == 0 || binding.ExecutionRef == "" {
		return GlobalCampaignEffectAttestationBinding{}, ErrGlobalCampaignEffectAttestationInvalid
	}
	for _, digest := range []string{binding.CampaignEvidenceHashSHA256, binding.ProposalHashSHA256, binding.AuthorizationHashSHA256, binding.ActionSHA256, binding.EffectProofHashSHA256, binding.ExecutionEvidenceSHA256, binding.VerificationEvidenceSHA256} {
		if !validResponseDigest(digest) {
			return GlobalCampaignEffectAttestationBinding{}, ErrGlobalCampaignEffectAttestationInvalid
		}
	}
	return binding, nil
}

func GlobalCampaignEffectAttestationBindingDigest(binding GlobalCampaignEffectAttestationBinding) (string, error) {
	if binding.SchemaVersion != GlobalCampaignEffectAttestationSchemaVersion {
		return "", ErrGlobalCampaignEffectAttestationInvalid
	}
	payload, err := json.Marshal(binding)
	if err != nil {
		return "", fmt.Errorf("marshal effect attestation binding: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func BuildGlobalCampaignEffectAttestation(
	campaign GlobalCampaign,
	proposal GlobalCampaignResponseProposal,
	authorization GlobalCampaignResponseAuthorization,
	preflight executioncontainment.Receipt,
	proof GlobalCampaignResponseEffectProof,
	event securityevidence.Event,
	trust GlobalCampaignEffectAttestationTrust,
	authenticatedAt time.Time,
) (GlobalCampaignEffectAttestation, error) {
	if err := ValidateGlobalCampaignResponseEffectProof(campaign, proposal, authorization, preflight, proof); err != nil {
		return GlobalCampaignEffectAttestation{}, fmt.Errorf("%w: canonical effect proof: %v", ErrGlobalCampaignEffectAttestationInvalid, err)
	}
	if !proof.ContainmentProven || proof.State != GlobalCampaignEffectStateVerified {
		return GlobalCampaignEffectAttestation{}, ErrGlobalCampaignEffectAttestationInvalid
	}

	trust.Producer = strings.TrimSpace(trust.Producer)
	trust.PublicKey = strings.TrimSpace(trust.PublicKey)
	authenticatedAt = authenticatedAt.UTC()
	if trust.Producer == "" || trust.PublicKey == "" || trust.MaxAge <= 0 || trust.MaxFutureSkew < 0 || authenticatedAt.IsZero() ||
		!strings.EqualFold(strings.TrimSpace(proof.VerifierRef), trust.Producer) ||
		strings.EqualFold(strings.TrimSpace(proof.ExecutorRef), trust.Producer) ||
		strings.EqualFold(strings.TrimSpace(authorization.AuthorizerRef), trust.Producer) {
		return GlobalCampaignEffectAttestation{}, ErrGlobalCampaignEffectAttestationUntrusted
	}
	if err := event.VerifyEd25519(trust.Producer, trust.PublicKey); err != nil {
		return GlobalCampaignEffectAttestation{}, fmt.Errorf("%w: %v", ErrGlobalCampaignEffectAttestationUntrusted, err)
	}
	canonicalEvent, err := event.Canonical()
	if err != nil {
		return GlobalCampaignEffectAttestation{}, fmt.Errorf("%w: %v", ErrGlobalCampaignEffectAttestationInvalid, err)
	}

	windowStart := time.UnixMilli(canonicalEvent.Window.FromUnixMS).UTC()
	windowEnd := time.UnixMilli(canonicalEvent.Window.ToUnixMS).UTC()
	if windowStart.Before(proof.ExecutedAt.UTC()) || windowEnd.Before(windowStart) || windowEnd.After(authenticatedAt.Add(trust.MaxFutureSkew)) || authenticatedAt.Sub(windowStart) > trust.MaxAge || proof.VerifiedAt.UTC().Before(windowStart) || proof.VerifiedAt.UTC().After(windowEnd) {
		return GlobalCampaignEffectAttestation{}, ErrGlobalCampaignEffectAttestationStale
	}

	binding, err := BuildGlobalCampaignEffectAttestationBinding(campaign, proposal, authorization, proof)
	if err != nil {
		return GlobalCampaignEffectAttestation{}, err
	}
	bindingDigest, err := GlobalCampaignEffectAttestationBindingDigest(binding)
	if err != nil {
		return GlobalCampaignEffectAttestation{}, err
	}
	if !globalCampaignEffectAttestationEventMatches(canonicalEvent, proposal, proof, authorization, bindingDigest) {
		return GlobalCampaignEffectAttestation{}, ErrGlobalCampaignEffectAttestationMismatch
	}

	attestation := GlobalCampaignEffectAttestation{
		SchemaVersion:       GlobalCampaignEffectAttestationSchemaVersion,
		Binding:             binding,
		BindingHashSHA256:   "sha256:" + bindingDigest,
		EvidenceEventSHA256: "sha256:" + strings.ToLower(strings.TrimSpace(event.EventSHA256)),
		Producer:            trust.Producer,
		AuthenticatedAt:     authenticatedAt,
		Authenticated:       true,
		ExecutionAuthority:  false,
		SubmissionAuthority: false,
	}
	attestation.AttestationSHA256 = hashGlobalCampaignEffectAttestation(attestation)
	return attestation, nil
}

func ValidateGlobalCampaignEffectAttestation(
	campaign GlobalCampaign,
	proposal GlobalCampaignResponseProposal,
	authorization GlobalCampaignResponseAuthorization,
	preflight executioncontainment.Receipt,
	proof GlobalCampaignResponseEffectProof,
	event securityevidence.Event,
	trust GlobalCampaignEffectAttestationTrust,
	attestation GlobalCampaignEffectAttestation,
) error {
	if attestation.SchemaVersion != GlobalCampaignEffectAttestationSchemaVersion || !attestation.Authenticated || attestation.ExecutionAuthority || attestation.SubmissionAuthority || !validResponseDigest(attestation.BindingHashSHA256) || !validResponseDigest(attestation.EvidenceEventSHA256) || !validResponseDigest(attestation.AttestationSHA256) {
		return ErrGlobalCampaignEffectAttestationInvalid
	}
	rebuilt, err := BuildGlobalCampaignEffectAttestation(campaign, proposal, authorization, preflight, proof, event, trust, attestation.AuthenticatedAt)
	if err != nil {
		return err
	}
	if rebuilt.AttestationSHA256 != attestation.AttestationSHA256 || rebuilt.BindingHashSHA256 != attestation.BindingHashSHA256 || rebuilt.EvidenceEventSHA256 != attestation.EvidenceEventSHA256 {
		return ErrGlobalCampaignEffectAttestationMismatch
	}
	return nil
}

func globalCampaignEffectAttestationEventMatches(event securityevidence.Event, proposal GlobalCampaignResponseProposal, proof GlobalCampaignResponseEffectProof, authorization GlobalCampaignResponseAuthorization, bindingDigest string) bool {
	if event.Subject.Chain != strings.ToLower(strings.TrimSpace(proposal.Network)) || event.Subject.Type != GlobalCampaignEffectAttestationSubjectType || event.Subject.ID != strings.TrimSpace(proof.ExecutionRef) {
		return false
	}
	expectedSources := []string{
		rawResponseDigest(proof.ProofHashSHA256),
		rawResponseDigest(authorization.AuthorizationHashSHA256),
		rawResponseDigest(proposal.ActionSHA256),
		rawResponseDigest(proof.ExecutionEvidenceSHA256),
		rawResponseDigest(proof.VerificationEvidenceSHA256),
	}
	for _, digest := range expectedSources {
		if digest == "" {
			return false
		}
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
		if finding.ID == GlobalCampaignEffectAttestationFindingID && finding.Kind == GlobalCampaignEffectAttestationFindingKind && finding.State == securityevidence.StateVerified && strings.EqualFold(finding.EvidenceSHA256, bindingDigest) {
			return true
		}
	}
	return false
}

func rawResponseDigest(value string) string {
	value = normalizeResponseDigest(value)
	value = strings.TrimPrefix(value, "sha256:")
	if len(value) != sha256.Size*2 {
		return ""
	}
	if _, err := hex.DecodeString(value); err != nil {
		return ""
	}
	return value
}

func hashGlobalCampaignEffectAttestation(attestation GlobalCampaignEffectAttestation) string {
	attestation.AttestationSHA256 = ""
	payload, err := json.Marshal(attestation)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}
