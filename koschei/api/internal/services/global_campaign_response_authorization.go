package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	GlobalCampaignResponseProposalSchemaVersion      = "koschei.response-proposal.v1"
	GlobalCampaignResponseAuthorizationSchemaVersion = "koschei.response-authorization.v1"
	GlobalCampaignResponseAuthorizerOperator         = "operator"
)

var (
	ErrGlobalCampaignResponseInvalid      = errors.New("global campaign response authorization is invalid")
	ErrGlobalCampaignResponseMismatch     = errors.New("global campaign response authorization binding mismatch")
	ErrGlobalCampaignResponseUnauthorized = errors.New("global campaign response is not explicitly authorized")
	ErrGlobalCampaignResponseExpired      = errors.New("global campaign response authorization is expired or not yet active")
)

// GlobalCampaignResponseProposal is a bounded, single-target defensive action proposal.
// It carries no execution authority. Fabric and Sentinel may contribute evidence or
// recommendations upstream, but neither can authorize or execute this proposal.
type GlobalCampaignResponseProposal struct {
	SchemaVersion              string    `json:"schema_version"`
	CampaignRef                string    `json:"campaign_ref"`
	CampaignRevision           uint64    `json:"campaign_revision"`
	CampaignEvidenceHashSHA256 string    `json:"campaign_evidence_hash_sha256"`
	FabricContractHashSHA256   string    `json:"fabric_contract_hash_sha256"`
	PolicyRef                  string    `json:"policy_ref"`
	PolicyHashSHA256           string    `json:"policy_hash_sha256"`
	Network                    string    `json:"network"`
	Target                     string    `json:"target"`
	ActionKind                 string    `json:"action_kind"`
	ActionSHA256               string    `json:"action_sha256"`
	RequestedAt                time.Time `json:"requested_at"`
	ExpiresAt                  time.Time `json:"expires_at"`
	DefensiveOnly              bool      `json:"defensive_only"`
	BoundedScope               bool      `json:"bounded_scope"`
	AuthorityExpansionAllowed  bool      `json:"authority_expansion_allowed"`
	AssetTransferAllowed       bool      `json:"asset_transfer_allowed"`
	ExecutionAuthority         bool      `json:"execution_authority"`
	ProposalHashSHA256         string    `json:"proposal_hash_sha256"`
}

// GlobalCampaignResponseAuthorization is an immutable operator authorization
// artifact pinned to one proposal. It is permission evidence, not proof that an
// action executed or had the intended effect.
type GlobalCampaignResponseAuthorization struct {
	SchemaVersion              string    `json:"schema_version"`
	ProposalHashSHA256         string    `json:"proposal_hash_sha256"`
	CampaignRef                string    `json:"campaign_ref"`
	CampaignRevision           uint64    `json:"campaign_revision"`
	CampaignEvidenceHashSHA256 string    `json:"campaign_evidence_hash_sha256"`
	FabricContractHashSHA256   string    `json:"fabric_contract_hash_sha256"`
	PolicyRef                  string    `json:"policy_ref"`
	PolicyHashSHA256           string    `json:"policy_hash_sha256"`
	ActionSHA256               string    `json:"action_sha256"`
	AuthorizerKind             string    `json:"authorizer_kind"`
	AuthorizerRef              string    `json:"authorizer_ref"`
	AuthorizationEvidenceRef   string    `json:"authorization_evidence_ref"`
	AuthorizedAt               time.Time `json:"authorized_at"`
	ExpiresAt                  time.Time `json:"expires_at"`
	ExplicitAuthorization      bool      `json:"explicit_authorization"`
	AuthorizationHashSHA256    string    `json:"authorization_hash_sha256"`
}

func NewGlobalCampaignResponseProposal(
	campaign GlobalCampaign,
	fabric FabricCampaignEvidenceContract,
	proposal GlobalCampaignResponseProposal,
) (GlobalCampaignResponseProposal, error) {
	if err := validateGlobalCampaignResponseBinding(campaign, fabric); err != nil {
		return GlobalCampaignResponseProposal{}, err
	}

	proposal.SchemaVersion = GlobalCampaignResponseProposalSchemaVersion
	proposal.CampaignRef = campaign.CampaignRef
	proposal.CampaignRevision = campaign.Revision
	proposal.CampaignEvidenceHashSHA256 = campaign.EvidenceHashSHA256
	proposal.FabricContractHashSHA256 = fabric.ContractHashSHA256
	proposal.PolicyRef = strings.TrimSpace(proposal.PolicyRef)
	proposal.PolicyHashSHA256 = normalizeResponseDigest(proposal.PolicyHashSHA256)
	proposal.Network = strings.TrimSpace(proposal.Network)
	proposal.Target = strings.TrimSpace(proposal.Target)
	proposal.ActionKind = strings.TrimSpace(proposal.ActionKind)
	proposal.ActionSHA256 = normalizeResponseDigest(proposal.ActionSHA256)
	proposal.RequestedAt = proposal.RequestedAt.UTC()
	proposal.ExpiresAt = proposal.ExpiresAt.UTC()
	proposal.ExecutionAuthority = false
	proposal.ProposalHashSHA256 = ""

	if err := ValidateGlobalCampaignResponseProposal(proposal); err != nil {
		return GlobalCampaignResponseProposal{}, err
	}
	proposal.ProposalHashSHA256 = hashGlobalCampaignResponseProposal(proposal)
	return proposal, nil
}

func ValidateGlobalCampaignResponseProposal(proposal GlobalCampaignResponseProposal) error {
	if proposal.SchemaVersion != GlobalCampaignResponseProposalSchemaVersion ||
		strings.TrimSpace(proposal.CampaignRef) == "" || proposal.CampaignRevision == 0 ||
		!validResponseDigest(proposal.CampaignEvidenceHashSHA256) ||
		!validResponseDigest(proposal.FabricContractHashSHA256) ||
		strings.TrimSpace(proposal.PolicyRef) == "" || !validResponseDigest(proposal.PolicyHashSHA256) ||
		strings.TrimSpace(proposal.Network) == "" || strings.TrimSpace(proposal.Target) == "" ||
		strings.TrimSpace(proposal.ActionKind) == "" || !validResponseDigest(proposal.ActionSHA256) ||
		proposal.RequestedAt.IsZero() || proposal.ExpiresAt.IsZero() || !proposal.ExpiresAt.After(proposal.RequestedAt) {
		return ErrGlobalCampaignResponseInvalid
	}
	if !proposal.DefensiveOnly || !proposal.BoundedScope || proposal.AuthorityExpansionAllowed || proposal.AssetTransferAllowed || proposal.ExecutionAuthority {
		return ErrGlobalCampaignResponseInvalid
	}
	if proposal.ProposalHashSHA256 != "" && hashGlobalCampaignResponseProposal(proposal) != proposal.ProposalHashSHA256 {
		return ErrGlobalCampaignResponseInvalid
	}
	return nil
}

func NewGlobalCampaignResponseAuthorization(
	campaign GlobalCampaign,
	fabric FabricCampaignEvidenceContract,
	proposal GlobalCampaignResponseProposal,
	authorization GlobalCampaignResponseAuthorization,
	now time.Time,
) (GlobalCampaignResponseAuthorization, error) {
	if err := validateGlobalCampaignResponseBinding(campaign, fabric); err != nil {
		return GlobalCampaignResponseAuthorization{}, err
	}
	if err := ValidateGlobalCampaignResponseProposal(proposal); err != nil {
		return GlobalCampaignResponseAuthorization{}, err
	}
	if proposal.ProposalHashSHA256 == "" || hashGlobalCampaignResponseProposal(proposal) != proposal.ProposalHashSHA256 {
		return GlobalCampaignResponseAuthorization{}, ErrGlobalCampaignResponseInvalid
	}
	if !proposalMatchesCampaignAndFabric(proposal, campaign, fabric) {
		return GlobalCampaignResponseAuthorization{}, ErrGlobalCampaignResponseMismatch
	}

	now = now.UTC()
	if now.IsZero() || now.Before(proposal.RequestedAt) || !now.Before(proposal.ExpiresAt) {
		return GlobalCampaignResponseAuthorization{}, ErrGlobalCampaignResponseExpired
	}

	authorization.SchemaVersion = GlobalCampaignResponseAuthorizationSchemaVersion
	authorization.ProposalHashSHA256 = proposal.ProposalHashSHA256
	authorization.CampaignRef = proposal.CampaignRef
	authorization.CampaignRevision = proposal.CampaignRevision
	authorization.CampaignEvidenceHashSHA256 = proposal.CampaignEvidenceHashSHA256
	authorization.FabricContractHashSHA256 = proposal.FabricContractHashSHA256
	authorization.PolicyRef = proposal.PolicyRef
	authorization.PolicyHashSHA256 = proposal.PolicyHashSHA256
	authorization.ActionSHA256 = proposal.ActionSHA256
	authorization.AuthorizerKind = strings.TrimSpace(strings.ToLower(authorization.AuthorizerKind))
	authorization.AuthorizerRef = strings.TrimSpace(authorization.AuthorizerRef)
	authorization.AuthorizationEvidenceRef = strings.TrimSpace(authorization.AuthorizationEvidenceRef)
	authorization.AuthorizedAt = authorization.AuthorizedAt.UTC()
	authorization.ExpiresAt = authorization.ExpiresAt.UTC()
	authorization.AuthorizationHashSHA256 = ""

	if authorization.AuthorizerKind != GlobalCampaignResponseAuthorizerOperator ||
		authorization.AuthorizerRef == "" || authorization.AuthorizationEvidenceRef == "" ||
		!authorization.ExplicitAuthorization {
		return GlobalCampaignResponseAuthorization{}, ErrGlobalCampaignResponseUnauthorized
	}
	if authorization.AuthorizedAt.IsZero() || authorization.ExpiresAt.IsZero() ||
		authorization.AuthorizedAt.Before(proposal.RequestedAt) || authorization.AuthorizedAt.After(now) ||
		!authorization.ExpiresAt.After(authorization.AuthorizedAt) || authorization.ExpiresAt.After(proposal.ExpiresAt) ||
		!now.Before(authorization.ExpiresAt) {
		return GlobalCampaignResponseAuthorization{}, ErrGlobalCampaignResponseExpired
	}

	authorization.AuthorizationHashSHA256 = hashGlobalCampaignResponseAuthorization(authorization)
	return authorization, nil
}

func ValidateGlobalCampaignResponseAuthorization(
	campaign GlobalCampaign,
	fabric FabricCampaignEvidenceContract,
	proposal GlobalCampaignResponseProposal,
	authorization GlobalCampaignResponseAuthorization,
	now time.Time,
) error {
	if err := validateGlobalCampaignResponseBinding(campaign, fabric); err != nil {
		return err
	}
	if err := ValidateGlobalCampaignResponseProposal(proposal); err != nil {
		return err
	}
	if proposal.ProposalHashSHA256 == "" || hashGlobalCampaignResponseProposal(proposal) != proposal.ProposalHashSHA256 ||
		!proposalMatchesCampaignAndFabric(proposal, campaign, fabric) {
		return ErrGlobalCampaignResponseMismatch
	}
	if authorization.SchemaVersion != GlobalCampaignResponseAuthorizationSchemaVersion ||
		authorization.ProposalHashSHA256 != proposal.ProposalHashSHA256 ||
		authorization.CampaignRef != proposal.CampaignRef || authorization.CampaignRevision != proposal.CampaignRevision ||
		authorization.CampaignEvidenceHashSHA256 != proposal.CampaignEvidenceHashSHA256 ||
		authorization.FabricContractHashSHA256 != proposal.FabricContractHashSHA256 ||
		authorization.PolicyRef != proposal.PolicyRef || authorization.PolicyHashSHA256 != proposal.PolicyHashSHA256 ||
		normalizeResponseDigest(authorization.ActionSHA256) != proposal.ActionSHA256 ||
		authorization.AuthorizerKind != GlobalCampaignResponseAuthorizerOperator ||
		strings.TrimSpace(authorization.AuthorizerRef) == "" || strings.TrimSpace(authorization.AuthorizationEvidenceRef) == "" ||
		!authorization.ExplicitAuthorization || !validResponseDigest(authorization.AuthorizationHashSHA256) {
		return ErrGlobalCampaignResponseUnauthorized
	}
	if hashGlobalCampaignResponseAuthorization(authorization) != authorization.AuthorizationHashSHA256 {
		return ErrGlobalCampaignResponseInvalid
	}

	now = now.UTC()
	if now.IsZero() || now.Before(proposal.RequestedAt) || !now.Before(proposal.ExpiresAt) ||
		authorization.AuthorizedAt.Before(proposal.RequestedAt) || authorization.AuthorizedAt.After(now) ||
		!authorization.ExpiresAt.After(authorization.AuthorizedAt) || authorization.ExpiresAt.After(proposal.ExpiresAt) ||
		!now.Before(authorization.ExpiresAt) {
		return ErrGlobalCampaignResponseExpired
	}
	return nil
}

func validateGlobalCampaignResponseBinding(campaign GlobalCampaign, fabric FabricCampaignEvidenceContract) error {
	if err := ValidateGlobalCampaign(campaign); err != nil {
		return fmt.Errorf("%w: campaign: %v", ErrGlobalCampaignResponseInvalid, err)
	}
	if err := ValidateFabricCampaignEvidenceContract(fabric); err != nil {
		return fmt.Errorf("%w: fabric: %v", ErrGlobalCampaignResponseInvalid, err)
	}
	if fabric.CampaignRef != campaign.CampaignRef || fabric.CampaignRevision != campaign.Revision ||
		fabric.CampaignEvidenceHashSHA256 != campaign.EvidenceHashSHA256 || fabric.Mode != FabricCampaignEvidenceMode ||
		fabric.ResponseExecutionAuthority || fabric.ContainmentAuthority {
		return ErrGlobalCampaignResponseMismatch
	}
	return nil
}

func proposalMatchesCampaignAndFabric(proposal GlobalCampaignResponseProposal, campaign GlobalCampaign, fabric FabricCampaignEvidenceContract) bool {
	return proposal.CampaignRef == campaign.CampaignRef &&
		proposal.CampaignRevision == campaign.Revision &&
		proposal.CampaignEvidenceHashSHA256 == campaign.EvidenceHashSHA256 &&
		proposal.FabricContractHashSHA256 == fabric.ContractHashSHA256
}

func hashGlobalCampaignResponseProposal(proposal GlobalCampaignResponseProposal) string {
	proposal.ProposalHashSHA256 = ""
	payload, err := json.Marshal(proposal)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func hashGlobalCampaignResponseAuthorization(authorization GlobalCampaignResponseAuthorization) string {
	authorization.AuthorizationHashSHA256 = ""
	payload, err := json.Marshal(authorization)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func normalizeResponseDigest(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return value
}

func validResponseDigest(value string) bool {
	value = normalizeResponseDigest(value)
	if strings.HasPrefix(value, "sha256:") {
		value = strings.TrimPrefix(value, "sha256:")
	}
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
