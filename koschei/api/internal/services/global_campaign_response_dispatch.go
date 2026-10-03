package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	GlobalCampaignResponseDispatchSchemaVersion = "koschei.response-dispatch.v1"
	GlobalCampaignResponseDispatchMode          = "external_operator_forwarder"
	GlobalCampaignResponseDispatchRefPrefix     = "KRD1-"
)

var (
	ErrGlobalCampaignResponseDispatchInvalid  = errors.New("global campaign response dispatch envelope is invalid")
	ErrGlobalCampaignResponseDispatchMismatch = errors.New("global campaign response dispatch binding mismatch")
	ErrGlobalCampaignResponseDispatchExpired  = errors.New("global campaign response dispatch authorization is expired or not yet active")
)

// GlobalCampaignResponseDispatchRequest identifies the immutable external
// material an operator-controlled forwarder must resolve. Raw payload bytes,
// signing keys and custody material are intentionally excluded from this
// contract.
type GlobalCampaignResponseDispatchRequest struct {
	ExternalOperatorRef   string `json:"external_operator_ref"`
	ApprovedActionRef     string `json:"approved_action_ref"`
	ApprovedPayloadRef    string `json:"approved_payload_ref"`
	ApprovedPayloadSHA256 string `json:"approved_payload_sha256"`
}

// GlobalCampaignResponseDispatchEnvelope is an idempotent handoff artifact. It
// binds one exact C11 authorization to one approved payload digest and external
// operator boundary. The envelope is not a submission instruction and carries
// no signing, custody or execution authority.
type GlobalCampaignResponseDispatchEnvelope struct {
	SchemaVersion              string    `json:"schema_version"`
	Mode                       string    `json:"mode"`
	CampaignRef                string    `json:"campaign_ref"`
	CampaignRevision           uint64    `json:"campaign_revision"`
	CampaignEvidenceHashSHA256 string    `json:"campaign_evidence_hash_sha256"`
	FabricContractHashSHA256   string    `json:"fabric_contract_hash_sha256"`
	ProposalHashSHA256         string    `json:"proposal_hash_sha256"`
	AuthorizationHashSHA256    string    `json:"authorization_hash_sha256"`
	PolicyHashSHA256           string    `json:"policy_hash_sha256"`
	Network                    string    `json:"network"`
	Target                     string    `json:"target"`
	ActionSHA256               string    `json:"action_sha256"`
	ExternalOperatorRef        string    `json:"external_operator_ref"`
	ApprovedActionRef          string    `json:"approved_action_ref"`
	ApprovedPayloadRef         string    `json:"approved_payload_ref"`
	ApprovedPayloadSHA256      string    `json:"approved_payload_sha256"`
	AuthorizedAt               time.Time `json:"authorized_at"`
	ExpiresAt                  time.Time `json:"expires_at"`
	CreatedAt                  time.Time `json:"created_at"`
	DispatchRef                string    `json:"dispatch_ref"`
	ReplayKeySHA256            string    `json:"replay_key_sha256"`
	ExternalOperatorRequired   bool      `json:"external_operator_required"`
	AutonomousExecutionAllowed bool      `json:"autonomous_execution_allowed"`
	SubmissionAuthority        bool      `json:"submission_authority"`
	SigningAuthority           bool      `json:"signing_authority"`
	CustodyAuthority           bool      `json:"custody_authority"`
	SideEffectClaim            bool      `json:"side_effect_claim"`
	EnvelopeHashSHA256         string    `json:"envelope_hash_sha256"`
}

func BuildGlobalCampaignResponseDispatchEnvelope(
	campaign GlobalCampaign,
	fabric FabricCampaignEvidenceContract,
	proposal GlobalCampaignResponseProposal,
	authorization GlobalCampaignResponseAuthorization,
	request GlobalCampaignResponseDispatchRequest,
	now time.Time,
) (GlobalCampaignResponseDispatchEnvelope, error) {
	now = now.UTC()
	if now.IsZero() {
		return GlobalCampaignResponseDispatchEnvelope{}, ErrGlobalCampaignResponseDispatchInvalid
	}
	if err := ValidateGlobalCampaignResponseAuthorization(campaign, fabric, proposal, authorization, now); err != nil {
		if errors.Is(err, ErrGlobalCampaignResponseExpired) {
			return GlobalCampaignResponseDispatchEnvelope{}, ErrGlobalCampaignResponseDispatchExpired
		}
		return GlobalCampaignResponseDispatchEnvelope{}, ErrGlobalCampaignResponseDispatchMismatch
	}

	request = normalizeGlobalCampaignResponseDispatchRequest(request)
	if request.ExternalOperatorRef == "" || request.ApprovedActionRef == "" || request.ApprovedPayloadRef == "" ||
		responseDigestHex(request.ApprovedPayloadSHA256) == "" {
		return GlobalCampaignResponseDispatchEnvelope{}, ErrGlobalCampaignResponseDispatchInvalid
	}

	envelope := GlobalCampaignResponseDispatchEnvelope{
		SchemaVersion:              GlobalCampaignResponseDispatchSchemaVersion,
		Mode:                       GlobalCampaignResponseDispatchMode,
		CampaignRef:                campaign.CampaignRef,
		CampaignRevision:           campaign.Revision,
		CampaignEvidenceHashSHA256: responseDigestHex(campaign.EvidenceHashSHA256),
		FabricContractHashSHA256:   responseDigestHex(fabric.ContractHashSHA256),
		ProposalHashSHA256:         responseDigestHex(proposal.ProposalHashSHA256),
		AuthorizationHashSHA256:    responseDigestHex(authorization.AuthorizationHashSHA256),
		PolicyHashSHA256:           responseDigestHex(proposal.PolicyHashSHA256),
		Network:                    strings.ToLower(strings.TrimSpace(proposal.Network)),
		Target:                     strings.TrimSpace(proposal.Target),
		ActionSHA256:               responseDigestHex(proposal.ActionSHA256),
		ExternalOperatorRef:        request.ExternalOperatorRef,
		ApprovedActionRef:          request.ApprovedActionRef,
		ApprovedPayloadRef:         request.ApprovedPayloadRef,
		ApprovedPayloadSHA256:      responseDigestHex(request.ApprovedPayloadSHA256),
		AuthorizedAt:               authorization.AuthorizedAt.UTC(),
		ExpiresAt:                  authorization.ExpiresAt.UTC(),
		CreatedAt:                  now,
		ExternalOperatorRequired:   true,
		AutonomousExecutionAllowed: false,
		SubmissionAuthority:        false,
		SigningAuthority:           false,
		CustodyAuthority:           false,
		SideEffectClaim:            false,
	}
	if err := ValidateGlobalCampaignResponseDispatchEnvelopeShape(envelope); err != nil {
		return GlobalCampaignResponseDispatchEnvelope{}, err
	}
	replayKey := globalCampaignResponseDispatchReplayKey(envelope)
	if replayKey == "" {
		return GlobalCampaignResponseDispatchEnvelope{}, ErrGlobalCampaignResponseDispatchInvalid
	}
	envelope.ReplayKeySHA256 = replayKey
	envelope.DispatchRef = GlobalCampaignResponseDispatchRefPrefix + strings.ToUpper(strings.TrimPrefix(replayKey, "sha256:"))[:16]
	envelope.EnvelopeHashSHA256 = hashGlobalCampaignResponseDispatchEnvelope(envelope)
	return envelope, nil
}

func ValidateGlobalCampaignResponseDispatchEnvelopeShape(envelope GlobalCampaignResponseDispatchEnvelope) error {
	if envelope.SchemaVersion != GlobalCampaignResponseDispatchSchemaVersion ||
		envelope.Mode != GlobalCampaignResponseDispatchMode ||
		strings.TrimSpace(envelope.CampaignRef) == "" || envelope.CampaignRevision == 0 ||
		strings.TrimSpace(envelope.Network) == "" || strings.TrimSpace(envelope.Target) == "" ||
		strings.TrimSpace(envelope.ExternalOperatorRef) == "" || strings.TrimSpace(envelope.ApprovedActionRef) == "" ||
		strings.TrimSpace(envelope.ApprovedPayloadRef) == "" || envelope.AuthorizedAt.IsZero() || envelope.ExpiresAt.IsZero() || envelope.CreatedAt.IsZero() ||
		!envelope.ExpiresAt.After(envelope.AuthorizedAt) || envelope.CreatedAt.Before(envelope.AuthorizedAt) || !envelope.CreatedAt.Before(envelope.ExpiresAt) {
		return ErrGlobalCampaignResponseDispatchInvalid
	}
	for _, digest := range []string{
		envelope.CampaignEvidenceHashSHA256,
		envelope.FabricContractHashSHA256,
		envelope.ProposalHashSHA256,
		envelope.AuthorizationHashSHA256,
		envelope.PolicyHashSHA256,
		envelope.ActionSHA256,
		envelope.ApprovedPayloadSHA256,
	} {
		if responseDigestHex(digest) == "" {
			return ErrGlobalCampaignResponseDispatchInvalid
		}
	}
	if !envelope.ExternalOperatorRequired || envelope.AutonomousExecutionAllowed || envelope.SubmissionAuthority || envelope.SigningAuthority || envelope.CustodyAuthority || envelope.SideEffectClaim {
		return ErrGlobalCampaignResponseDispatchInvalid
	}
	return nil
}

func ValidateGlobalCampaignResponseDispatchEnvelope(
	campaign GlobalCampaign,
	fabric FabricCampaignEvidenceContract,
	proposal GlobalCampaignResponseProposal,
	authorization GlobalCampaignResponseAuthorization,
	envelope GlobalCampaignResponseDispatchEnvelope,
	now time.Time,
) error {
	if err := ValidateGlobalCampaignResponseDispatchEnvelopeShape(envelope); err != nil {
		return err
	}
	now = now.UTC()
	if now.IsZero() || now.Before(envelope.AuthorizedAt) || !now.Before(envelope.ExpiresAt) {
		return ErrGlobalCampaignResponseDispatchExpired
	}
	if err := ValidateGlobalCampaignResponseAuthorization(campaign, fabric, proposal, authorization, now); err != nil {
		if errors.Is(err, ErrGlobalCampaignResponseExpired) {
			return ErrGlobalCampaignResponseDispatchExpired
		}
		return ErrGlobalCampaignResponseDispatchMismatch
	}
	if envelope.CampaignRef != campaign.CampaignRef || envelope.CampaignRevision != campaign.Revision ||
		envelope.CampaignEvidenceHashSHA256 != responseDigestHex(campaign.EvidenceHashSHA256) ||
		envelope.FabricContractHashSHA256 != responseDigestHex(fabric.ContractHashSHA256) ||
		envelope.ProposalHashSHA256 != responseDigestHex(proposal.ProposalHashSHA256) ||
		envelope.AuthorizationHashSHA256 != responseDigestHex(authorization.AuthorizationHashSHA256) ||
		envelope.PolicyHashSHA256 != responseDigestHex(proposal.PolicyHashSHA256) ||
		envelope.ActionSHA256 != responseDigestHex(proposal.ActionSHA256) ||
		envelope.Network != strings.ToLower(strings.TrimSpace(proposal.Network)) || envelope.Target != strings.TrimSpace(proposal.Target) {
		return ErrGlobalCampaignResponseDispatchMismatch
	}
	replayKey := globalCampaignResponseDispatchReplayKey(envelope)
	if replayKey == "" || replayKey != envelope.ReplayKeySHA256 {
		return ErrGlobalCampaignResponseDispatchInvalid
	}
	expectedRef := GlobalCampaignResponseDispatchRefPrefix + strings.ToUpper(strings.TrimPrefix(replayKey, "sha256:"))[:16]
	if envelope.DispatchRef != expectedRef || responseDigestHex(envelope.EnvelopeHashSHA256) == "" || hashGlobalCampaignResponseDispatchEnvelope(envelope) != envelope.EnvelopeHashSHA256 {
		return ErrGlobalCampaignResponseDispatchInvalid
	}
	return nil
}

func normalizeGlobalCampaignResponseDispatchRequest(request GlobalCampaignResponseDispatchRequest) GlobalCampaignResponseDispatchRequest {
	request.ExternalOperatorRef = strings.TrimSpace(request.ExternalOperatorRef)
	request.ApprovedActionRef = strings.TrimSpace(request.ApprovedActionRef)
	request.ApprovedPayloadRef = strings.TrimSpace(request.ApprovedPayloadRef)
	request.ApprovedPayloadSHA256 = responseDigestHex(request.ApprovedPayloadSHA256)
	return request
}

func globalCampaignResponseDispatchReplayKey(envelope GlobalCampaignResponseDispatchEnvelope) string {
	identity := struct {
		AuthorizationHashSHA256 string `json:"authorization_hash_sha256"`
		ActionSHA256            string `json:"action_sha256"`
		ApprovedPayloadSHA256   string `json:"approved_payload_sha256"`
		ExternalOperatorRef     string `json:"external_operator_ref"`
		Network                 string `json:"network"`
		Target                  string `json:"target"`
	}{
		AuthorizationHashSHA256: responseDigestHex(envelope.AuthorizationHashSHA256),
		ActionSHA256:            responseDigestHex(envelope.ActionSHA256),
		ApprovedPayloadSHA256:   responseDigestHex(envelope.ApprovedPayloadSHA256),
		ExternalOperatorRef:     strings.TrimSpace(envelope.ExternalOperatorRef),
		Network:                 strings.ToLower(strings.TrimSpace(envelope.Network)),
		Target:                  strings.TrimSpace(envelope.Target),
	}
	if identity.AuthorizationHashSHA256 == "" || identity.ActionSHA256 == "" || identity.ApprovedPayloadSHA256 == "" || identity.ExternalOperatorRef == "" || identity.Network == "" || identity.Target == "" {
		return ""
	}
	payload, err := json.Marshal(identity)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func hashGlobalCampaignResponseDispatchEnvelope(envelope GlobalCampaignResponseDispatchEnvelope) string {
	envelope.EnvelopeHashSHA256 = ""
	payload, err := json.Marshal(envelope)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}
