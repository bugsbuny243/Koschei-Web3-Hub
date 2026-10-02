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
)

const (
	FabricCampaignEvidenceSchemaVersion = "koschei.fabric.campaign-evidence.v1"
	FabricCampaignEvidenceMode          = "observe"
)

var (
	ErrFabricCampaignEvidenceInvalid  = errors.New("fabric campaign evidence contract is invalid")
	ErrFabricCampaignEvidenceMismatch = errors.New("fabric campaign evidence input campaign mismatch")
)

type FabricCampaignIncidentRef struct {
	IncidentRef        string `json:"incident_ref"`
	CampaignRevision   int64  `json:"campaign_revision"`
	EvidenceHashSHA256 string `json:"evidence_hash_sha256"`
}

type FabricCampaignEvidenceContract struct {
	SchemaVersion              string                      `json:"schema_version"`
	Mode                       string                      `json:"mode"`
	CampaignRef                string                      `json:"campaign_ref"`
	CampaignRevision           uint64                      `json:"campaign_revision"`
	CampaignState              GlobalCampaignState         `json:"campaign_state"`
	CampaignEvidenceHashSHA256 string                      `json:"campaign_evidence_hash_sha256"`
	RulesetVersion             string                      `json:"ruleset_version"`
	FirstObservedAt            time.Time                   `json:"first_observed_at"`
	LastObservedAt             time.Time                   `json:"last_observed_at"`
	Networks                   []string                    `json:"networks"`
	Subjects                   []string                    `json:"subjects"`
	EvidenceRefs               []string                    `json:"evidence_refs"`
	VerdictRefs                []string                    `json:"verdict_refs"`
	AttackPathRefs             []string                    `json:"attack_path_refs"`
	MissingEvidence            []string                    `json:"missing_evidence"`
	TemporalFingerprintSHA256  string                      `json:"temporal_fingerprint_sha256,omitempty"`
	RadarFingerprintSHA256     string                      `json:"radar_fingerprint_sha256,omitempty"`
	ThreatFingerprintSHA256    string                      `json:"threat_fingerprint_sha256,omitempty"`
	IncidentRefs               []FabricCampaignIncidentRef `json:"incident_refs"`
	ContractHashSHA256         string                      `json:"contract_hash_sha256"`
	VerdictAuthority           bool                        `json:"verdict_authority"`
	GradeAuthority             bool                        `json:"grade_authority"`
	ContainmentAuthority       bool                        `json:"containment_authority"`
	ResponseExecutionAuthority bool                        `json:"response_execution_authority"`
	SameOperatorClaim          bool                        `json:"same_operator_claim"`
	RealWorldIdentityClaim     bool                        `json:"real_world_identity_claim"`
	WrongdoingClaim            bool                        `json:"wrongdoing_claim"`
}

func BuildFabricCampaignEvidenceContract(
	campaign GlobalCampaign,
	temporal GlobalCampaignTemporalCorrelationReport,
	radar GlobalCampaignRadarProjection,
	threat GlobalCampaignThreatFamilyReport,
	incidentLinks []GlobalCampaignIncidentLink,
) (FabricCampaignEvidenceContract, error) {
	if err := ValidateGlobalCampaign(campaign); err != nil {
		return FabricCampaignEvidenceContract{}, fmt.Errorf("%w: campaign: %v", ErrFabricCampaignEvidenceInvalid, err)
	}
	if temporal.Version != "" {
		if temporal.Version != GlobalCampaignTemporalCorrelatorVersion || strings.TrimSpace(temporal.CampaignRef) != campaign.CampaignRef || strings.TrimSpace(temporal.FingerprintSHA256) == "" ||
			temporal.VerdictAuthority || temporal.GradeAuthority || temporal.ContainmentAuthority || temporal.SameOperatorClaim || temporal.RealWorldIdentityClaim || temporal.WrongdoingClaim {
			return FabricCampaignEvidenceContract{}, ErrFabricCampaignEvidenceMismatch
		}
	}
	if radar.Version != "" {
		if radar.Version != GlobalCampaignRadarProjectionVersion || strings.TrimSpace(radar.CampaignRef) != campaign.CampaignRef || strings.TrimSpace(radar.FingerprintSHA256) == "" ||
			radar.VerdictAuthority || radar.GradeAuthority || radar.ContainmentAuthority || radar.SameOperatorClaim || radar.RealWorldIdentityClaim || radar.WrongdoingClaim {
			return FabricCampaignEvidenceContract{}, ErrFabricCampaignEvidenceMismatch
		}
	}
	if threat.Version != "" {
		if threat.Version != GlobalCampaignThreatFamiliesVersion || strings.TrimSpace(threat.CampaignRef) != campaign.CampaignRef || strings.TrimSpace(threat.FingerprintSHA256) == "" ||
			threat.VerdictAuthority || threat.GradeAuthority || threat.ContainmentAuthority || threat.SameOperatorClaim || threat.RealWorldIdentityClaim || threat.WrongdoingClaim {
			return FabricCampaignEvidenceContract{}, ErrFabricCampaignEvidenceMismatch
		}
	}

	evidenceRefs := append([]string{}, campaign.ObservationRefs...)
	evidenceRefs = append(evidenceRefs, campaign.RelationRefs...)
	evidenceRefs = append(evidenceRefs, campaign.BridgeLinkRefs...)
	evidenceRefs = append(evidenceRefs, campaign.CampaignGenomeRefs...)
	evidenceRefs = append(evidenceRefs, campaign.CampaignTempoRefs...)
	evidenceRefs = append(evidenceRefs, campaign.BehaviorSignatureRefs...)
	evidenceRefs = append(evidenceRefs, campaign.TransitionEvidenceRefs...)

	incidentRefs := make([]FabricCampaignIncidentRef, 0, len(incidentLinks))
	for _, link := range incidentLinks {
		if strings.TrimSpace(link.CampaignRef) != campaign.CampaignRef || link.CampaignRevision <= 0 || link.CampaignRevision > int64(campaign.Revision) || strings.TrimSpace(link.EvidenceHashSHA256) == "" {
			return FabricCampaignEvidenceContract{}, ErrFabricCampaignEvidenceMismatch
		}
		incidentRefs = append(incidentRefs, FabricCampaignIncidentRef{
			IncidentRef:        strings.TrimSpace(link.IncidentRef),
			CampaignRevision:   link.CampaignRevision,
			EvidenceHashSHA256: strings.TrimSpace(link.EvidenceHashSHA256),
		})
	}
	sort.SliceStable(incidentRefs, func(i, j int) bool {
		if incidentRefs[i].IncidentRef != incidentRefs[j].IncidentRef {
			return incidentRefs[i].IncidentRef < incidentRefs[j].IncidentRef
		}
		if incidentRefs[i].CampaignRevision != incidentRefs[j].CampaignRevision {
			return incidentRefs[i].CampaignRevision < incidentRefs[j].CampaignRevision
		}
		return incidentRefs[i].EvidenceHashSHA256 < incidentRefs[j].EvidenceHashSHA256
	})

	out := FabricCampaignEvidenceContract{
		SchemaVersion:              FabricCampaignEvidenceSchemaVersion,
		Mode:                       FabricCampaignEvidenceMode,
		CampaignRef:                campaign.CampaignRef,
		CampaignRevision:           campaign.Revision,
		CampaignState:              campaign.State,
		CampaignEvidenceHashSHA256: campaign.EvidenceHashSHA256,
		RulesetVersion:             campaign.RulesetVersion,
		FirstObservedAt:            campaign.FirstObservedAt.UTC(),
		LastObservedAt:             campaign.LastObservedAt.UTC(),
		Networks:                   normalizeGlobalCampaignStrings(campaign.Networks),
		Subjects:                   normalizeGlobalCampaignStrings(campaign.Subjects),
		EvidenceRefs:               normalizeGlobalCampaignStrings(evidenceRefs),
		VerdictRefs:                normalizeGlobalCampaignStrings(campaign.VerdictRefs),
		AttackPathRefs:             normalizeGlobalCampaignStrings(campaign.AttackPathRefs),
		MissingEvidence:            normalizeGlobalCampaignStrings(campaign.MissingEvidence),
		IncidentRefs:               incidentRefs,
	}
	if temporal.Version != "" {
		out.TemporalFingerprintSHA256 = temporal.FingerprintSHA256
	}
	if radar.Version != "" {
		out.RadarFingerprintSHA256 = radar.FingerprintSHA256
	}
	if threat.Version != "" {
		out.ThreatFingerprintSHA256 = threat.FingerprintSHA256
	}
	out.ContractHashSHA256 = hashFabricCampaignEvidenceContract(out)
	return out, nil
}

func ValidateFabricCampaignEvidenceContract(contract FabricCampaignEvidenceContract) error {
	if contract.SchemaVersion != FabricCampaignEvidenceSchemaVersion || contract.Mode != FabricCampaignEvidenceMode || strings.TrimSpace(contract.CampaignRef) == "" || contract.CampaignRevision == 0 || strings.TrimSpace(contract.CampaignEvidenceHashSHA256) == "" || strings.TrimSpace(contract.ContractHashSHA256) == "" {
		return ErrFabricCampaignEvidenceInvalid
	}
	if contract.VerdictAuthority || contract.GradeAuthority || contract.ContainmentAuthority || contract.ResponseExecutionAuthority || contract.SameOperatorClaim || contract.RealWorldIdentityClaim || contract.WrongdoingClaim {
		return ErrFabricCampaignEvidenceInvalid
	}
	if contract.LastObservedAt.Before(contract.FirstObservedAt) {
		return ErrFabricCampaignEvidenceInvalid
	}
	if hashFabricCampaignEvidenceContract(contract) != contract.ContractHashSHA256 {
		return ErrFabricCampaignEvidenceInvalid
	}
	return nil
}

func hashFabricCampaignEvidenceContract(contract FabricCampaignEvidenceContract) string {
	contract.ContractHashSHA256 = ""
	payload, err := json.Marshal(contract)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}
