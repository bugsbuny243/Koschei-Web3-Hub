package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"koschei/api/internal/executioncontainment"
)

const GlobalCampaignCommandCenterSchemaVersion = "koschei.global-campaign-command-center.v1"

type GlobalCampaignCommandCenterResponseState string

const GlobalCampaignCommandCenterMonitoring GlobalCampaignCommandCenterResponseState = "monitoring"
const GlobalCampaignCommandCenterProposed GlobalCampaignCommandCenterResponseState = "proposed"
const GlobalCampaignCommandCenterAuthorized GlobalCampaignCommandCenterResponseState = "authorized"
const GlobalCampaignCommandCenterExecuted GlobalCampaignCommandCenterResponseState = "executed"
const GlobalCampaignCommandCenterContainmentVerified GlobalCampaignCommandCenterResponseState = "containment_verified"

var (
	ErrGlobalCampaignCommandCenterInvalid  = errors.New("global campaign command center snapshot is invalid")
	ErrGlobalCampaignCommandCenterMismatch = errors.New("global campaign command center component mismatch")
)

type GlobalCampaignCommandCenterIncident struct {
	IncidentRef        string `json:"incident_ref"`
	CampaignRevision   int64  `json:"campaign_revision"`
	EvidenceHashSHA256 string `json:"evidence_hash_sha256"`
}

type GlobalCampaignCommandCenterInput struct {
	Campaign      GlobalCampaign
	Threat        GlobalCampaignThreatFamilyReport
	IncidentLinks []GlobalCampaignIncidentLink
	Fabric        FabricCampaignEvidenceContract
	Proposal      GlobalCampaignResponseProposal
	Authorization GlobalCampaignResponseAuthorization
	Preflight     executioncontainment.Receipt
	EffectProof   GlobalCampaignResponseEffectProof
	ObservedAt    time.Time
}

// GlobalCampaignCommandCenterSnapshot is the single truth-backed campaign view
// for operators and UI consumers. It projects existing evidence contracts and
// never creates verdict, grade, containment, or execution authority.
type GlobalCampaignCommandCenterSnapshot struct {
	SchemaVersion              string                                   `json:"schema_version"`
	CampaignRef                string                                   `json:"campaign_ref"`
	CampaignRevision           uint64                                   `json:"campaign_revision"`
	CampaignEvidenceHashSHA256 string                                   `json:"campaign_evidence_hash_sha256"`
	CampaignState              GlobalCampaignState                      `json:"campaign_state"`
	Networks                   []string                                 `json:"networks"`
	Subjects                   []string                                 `json:"subjects"`
	ThreatFingerprintSHA256    string                                   `json:"threat_fingerprint_sha256,omitempty"`
	ThreatFindingCount         int                                      `json:"threat_finding_count"`
	Incidents                  []GlobalCampaignCommandCenterIncident    `json:"incidents"`
	FabricMode                 string                                   `json:"fabric_mode"`
	FabricContractHashSHA256   string                                   `json:"fabric_contract_hash_sha256"`
	ProposalHashSHA256         string                                   `json:"proposal_hash_sha256,omitempty"`
	AuthorizationHashSHA256    string                                   `json:"authorization_hash_sha256,omitempty"`
	EffectProofHashSHA256      string                                   `json:"effect_proof_hash_sha256,omitempty"`
	ResponseState              GlobalCampaignCommandCenterResponseState `json:"response_state"`
	ContainmentVerified        bool                                     `json:"containment_verified"`
	MissingEvidence            []string                                 `json:"missing_evidence"`
	ObservedAt                 time.Time                                `json:"observed_at"`
	SnapshotHashSHA256         string                                   `json:"snapshot_hash_sha256"`
	VerdictAuthority           bool                                     `json:"verdict_authority"`
	GradeAuthority             bool                                     `json:"grade_authority"`
	ContainmentAuthority       bool                                     `json:"containment_authority"`
	ExecutionAuthority         bool                                     `json:"execution_authority"`
}

func BuildGlobalCampaignCommandCenterSnapshot(in GlobalCampaignCommandCenterInput) (GlobalCampaignCommandCenterSnapshot, error) {
	campaign := in.Campaign
	if err := ValidateGlobalCampaign(campaign); err != nil {
		return GlobalCampaignCommandCenterSnapshot{}, ErrGlobalCampaignCommandCenterInvalid
	}
	if err := ValidateFabricCampaignEvidenceContract(in.Fabric); err != nil {
		return GlobalCampaignCommandCenterSnapshot{}, ErrGlobalCampaignCommandCenterInvalid
	}
	if in.Fabric.CampaignRef != campaign.CampaignRef || in.Fabric.CampaignRevision != campaign.Revision || in.Fabric.CampaignEvidenceHashSHA256 != campaign.EvidenceHashSHA256 {
		return GlobalCampaignCommandCenterSnapshot{}, ErrGlobalCampaignCommandCenterMismatch
	}

	out := GlobalCampaignCommandCenterSnapshot{
		SchemaVersion:              GlobalCampaignCommandCenterSchemaVersion,
		CampaignRef:                campaign.CampaignRef,
		CampaignRevision:           campaign.Revision,
		CampaignEvidenceHashSHA256: campaign.EvidenceHashSHA256,
		CampaignState:              campaign.State,
		Networks:                   normalizeGlobalCampaignStrings(campaign.Networks),
		Subjects:                   normalizeGlobalCampaignStrings(campaign.Subjects),
		FabricMode:                 in.Fabric.Mode,
		FabricContractHashSHA256:   in.Fabric.ContractHashSHA256,
		ResponseState:              GlobalCampaignCommandCenterMonitoring,
		ContainmentVerified:        false,
		MissingEvidence:            normalizeGlobalCampaignStrings(campaign.MissingEvidence),
		ObservedAt:                 in.ObservedAt.UTC(),
		Incidents:                  []GlobalCampaignCommandCenterIncident{},
	}
	if out.ObservedAt.IsZero() {
		out.ObservedAt = campaign.LastObservedAt.UTC()
	}

	if in.Threat.Version != "" {
		if in.Threat.Version != GlobalCampaignThreatFamiliesVersion || strings.TrimSpace(in.Threat.CampaignRef) != campaign.CampaignRef || strings.TrimSpace(in.Threat.FingerprintSHA256) == "" ||
			in.Threat.VerdictAuthority || in.Threat.GradeAuthority || in.Threat.ContainmentAuthority || in.Threat.SameOperatorClaim || in.Threat.RealWorldIdentityClaim || in.Threat.WrongdoingClaim ||
			hashGlobalCampaignThreatFamilyReport(in.Threat) != in.Threat.FingerprintSHA256 {
			return GlobalCampaignCommandCenterSnapshot{}, ErrGlobalCampaignCommandCenterMismatch
		}
		out.ThreatFingerprintSHA256 = in.Threat.FingerprintSHA256
		out.ThreatFindingCount = in.Threat.FindingCount
	}

	for _, link := range in.IncidentLinks {
		if strings.TrimSpace(link.CampaignRef) != campaign.CampaignRef || link.CampaignRevision <= 0 || link.CampaignRevision > int64(campaign.Revision) || strings.TrimSpace(link.EvidenceHashSHA256) == "" || strings.TrimSpace(link.IncidentRef) == "" || link.VerdictAuthority || link.ContainmentAuthority {
			return GlobalCampaignCommandCenterSnapshot{}, ErrGlobalCampaignCommandCenterMismatch
		}
		out.Incidents = append(out.Incidents, GlobalCampaignCommandCenterIncident{
			IncidentRef:        strings.TrimSpace(link.IncidentRef),
			CampaignRevision:   link.CampaignRevision,
			EvidenceHashSHA256: strings.TrimSpace(link.EvidenceHashSHA256),
		})
	}
	sort.SliceStable(out.Incidents, func(i, j int) bool {
		if out.Incidents[i].IncidentRef != out.Incidents[j].IncidentRef {
			return out.Incidents[i].IncidentRef < out.Incidents[j].IncidentRef
		}
		if out.Incidents[i].CampaignRevision != out.Incidents[j].CampaignRevision {
			return out.Incidents[i].CampaignRevision < out.Incidents[j].CampaignRevision
		}
		return out.Incidents[i].EvidenceHashSHA256 < out.Incidents[j].EvidenceHashSHA256
	})

	hasProposal := strings.TrimSpace(in.Proposal.ProposalHashSHA256) != ""
	hasAuthorization := strings.TrimSpace(in.Authorization.AuthorizationHashSHA256) != ""
	hasEffectProof := strings.TrimSpace(in.EffectProof.ProofHashSHA256) != ""

	if hasProposal {
		if err := ValidateGlobalCampaignResponseProposal(in.Proposal); err != nil || !proposalMatchesCampaignAndFabric(in.Proposal, campaign, in.Fabric) {
			return GlobalCampaignCommandCenterSnapshot{}, ErrGlobalCampaignCommandCenterMismatch
		}
		out.ProposalHashSHA256 = in.Proposal.ProposalHashSHA256
		out.ResponseState = GlobalCampaignCommandCenterProposed
	}
	if hasAuthorization {
		if !hasProposal {
			return GlobalCampaignCommandCenterSnapshot{}, ErrGlobalCampaignCommandCenterMismatch
		}
		if err := ValidateGlobalCampaignResponseAuthorization(campaign, in.Fabric, in.Proposal, in.Authorization, out.ObservedAt); err != nil {
			return GlobalCampaignCommandCenterSnapshot{}, ErrGlobalCampaignCommandCenterMismatch
		}
		out.AuthorizationHashSHA256 = in.Authorization.AuthorizationHashSHA256
		out.ResponseState = GlobalCampaignCommandCenterAuthorized
	}
	if hasEffectProof {
		if !hasAuthorization {
			return GlobalCampaignCommandCenterSnapshot{}, ErrGlobalCampaignCommandCenterMismatch
		}
		if err := ValidateGlobalCampaignResponseEffectProof(campaign, in.Proposal, in.Authorization, in.Preflight, in.EffectProof); err != nil {
			return GlobalCampaignCommandCenterSnapshot{}, ErrGlobalCampaignCommandCenterMismatch
		}
		out.EffectProofHashSHA256 = in.EffectProof.ProofHashSHA256
		if in.EffectProof.ContainmentProven {
			out.ResponseState = GlobalCampaignCommandCenterContainmentVerified
			out.ContainmentVerified = true
		} else {
			out.ResponseState = GlobalCampaignCommandCenterExecuted
		}
	}

	out.SnapshotHashSHA256 = hashGlobalCampaignCommandCenterSnapshot(out)
	if err := ValidateGlobalCampaignCommandCenterSnapshot(out); err != nil {
		return GlobalCampaignCommandCenterSnapshot{}, err
	}
	return out, nil
}

func ValidateGlobalCampaignCommandCenterSnapshot(snapshot GlobalCampaignCommandCenterSnapshot) error {
	if snapshot.SchemaVersion != GlobalCampaignCommandCenterSchemaVersion || strings.TrimSpace(snapshot.CampaignRef) == "" || snapshot.CampaignRevision == 0 || !validResponseDigest(snapshot.CampaignEvidenceHashSHA256) || strings.TrimSpace(snapshot.FabricMode) == "" || !validResponseDigest(snapshot.FabricContractHashSHA256) || snapshot.ObservedAt.IsZero() || !validResponseDigest(snapshot.SnapshotHashSHA256) {
		return ErrGlobalCampaignCommandCenterInvalid
	}
	if snapshot.VerdictAuthority || snapshot.GradeAuthority || snapshot.ContainmentAuthority || snapshot.ExecutionAuthority {
		return ErrGlobalCampaignCommandCenterInvalid
	}
	if snapshot.ContainmentVerified != (snapshot.ResponseState == GlobalCampaignCommandCenterContainmentVerified) {
		return ErrGlobalCampaignCommandCenterInvalid
	}
	if snapshot.ResponseState == GlobalCampaignCommandCenterContainmentVerified && !validResponseDigest(snapshot.EffectProofHashSHA256) {
		return ErrGlobalCampaignCommandCenterInvalid
	}
	if hashGlobalCampaignCommandCenterSnapshot(snapshot) != snapshot.SnapshotHashSHA256 {
		return ErrGlobalCampaignCommandCenterInvalid
	}
	return nil
}

func hashGlobalCampaignCommandCenterSnapshot(snapshot GlobalCampaignCommandCenterSnapshot) string {
	snapshot.SnapshotHashSHA256 = ""
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}
