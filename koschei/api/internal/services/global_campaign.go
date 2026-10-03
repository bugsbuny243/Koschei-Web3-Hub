package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

const GlobalCampaignSchemaVersion = "koschei.global-campaign.v1"

// GlobalCampaignState is descriptive campaign lifecycle state. It has no ARVIS
// verdict, grade, signing, containment, or real-world identity authority.
type GlobalCampaignState string

const (
	GlobalCampaignEmerging   GlobalCampaignState = "emerging"
	GlobalCampaignActive     GlobalCampaignState = "active"
	GlobalCampaignEscalating GlobalCampaignState = "escalating"
	GlobalCampaignDegrading  GlobalCampaignState = "degrading"
	GlobalCampaignDormant    GlobalCampaignState = "dormant"
	GlobalCampaignClosed     GlobalCampaignState = "closed"
	GlobalCampaignReopened   GlobalCampaignState = "reopened"
)

// GlobalCampaign is a canonical, evidence-referenced projection over existing
// Global Radar, Actor Defense, campaign-genome, tempo, incident, and verdict
// artifacts. It deliberately does not create a second actor/entity graph.
type GlobalCampaign struct {
	SchemaVersion string              `json:"schema_version"`
	CampaignRef   string              `json:"campaign_ref"`
	Revision      uint64              `json:"revision"`
	State         GlobalCampaignState `json:"state"`

	FirstObservedAt time.Time `json:"first_observed_at"`
	LastObservedAt  time.Time `json:"last_observed_at"`

	Networks  []string `json:"networks"`
	Subjects  []string `json:"subjects"`
	Actors    []string `json:"actors"`
	Assets    []string `json:"assets"`
	Contracts []string `json:"contracts"`
	Pools     []string `json:"pools"`
	Bridges   []string `json:"bridges"`

	ObservationRefs        []string `json:"observation_refs"`
	RelationRefs           []string `json:"relation_refs"`
	BridgeLinkRefs         []string `json:"bridge_link_refs"`
	CampaignGenomeRefs     []string `json:"campaign_genome_refs"`
	CampaignTempoRefs      []string `json:"campaign_tempo_refs"`
	BehaviorSignatureRefs  []string `json:"behavior_signature_refs"`
	IncidentRefs           []string `json:"incident_refs"`
	VerdictRefs            []string `json:"verdict_refs"`
	AttackPathRefs         []string `json:"attack_path_refs"`
	MissingEvidence        []string `json:"missing_evidence"`
	TransitionReasonCodes  []string `json:"transition_reason_codes"`
	TransitionEvidenceRefs []string `json:"transition_evidence_refs"`

	VerifiedAnchorCount int `json:"verified_anchor_count"`
	ObservedAnchorCount int `json:"observed_anchor_count"`

	RulesetVersion     string `json:"ruleset_version"`
	EvidenceHashSHA256 string `json:"evidence_hash_sha256"`

	VerdictAuthority       bool `json:"verdict_authority"`
	GradeAuthority         bool `json:"grade_authority"`
	ContainmentAuthority   bool `json:"containment_authority"`
	SameOperatorClaim      bool `json:"same_operator_claim"`
	RealWorldIdentityClaim bool `json:"real_world_identity_claim"`
	WrongdoingClaim        bool `json:"wrongdoing_claim"`
}

// NewGlobalCampaign normalizes an evidence-referenced campaign snapshot and
// computes its immutable revision evidence hash. campaignRef is stable across
// revisions; EvidenceHashSHA256 changes when canonical evidence content changes.
func NewGlobalCampaign(c GlobalCampaign) GlobalCampaign {
	c.SchemaVersion = GlobalCampaignSchemaVersion
	if c.Revision == 0 {
		c.Revision = 1
	}
	if !validGlobalCampaignState(c.State) {
		c.State = GlobalCampaignEmerging
	}
	c.CampaignRef = strings.TrimSpace(c.CampaignRef)
	c.RulesetVersion = strings.TrimSpace(c.RulesetVersion)
	c.FirstObservedAt = c.FirstObservedAt.UTC()
	c.LastObservedAt = c.LastObservedAt.UTC()
	if c.FirstObservedAt.IsZero() && !c.LastObservedAt.IsZero() {
		c.FirstObservedAt = c.LastObservedAt
	}
	if c.LastObservedAt.IsZero() && !c.FirstObservedAt.IsZero() {
		c.LastObservedAt = c.FirstObservedAt
	}
	if !c.FirstObservedAt.IsZero() && !c.LastObservedAt.IsZero() && c.LastObservedAt.Before(c.FirstObservedAt) {
		c.FirstObservedAt, c.LastObservedAt = c.LastObservedAt, c.FirstObservedAt
	}

	c.Networks = normalizeGlobalCampaignStrings(c.Networks)
	c.Subjects = normalizeGlobalCampaignStrings(c.Subjects)
	c.Actors = normalizeGlobalCampaignStrings(c.Actors)
	c.Assets = normalizeGlobalCampaignStrings(c.Assets)
	c.Contracts = normalizeGlobalCampaignStrings(c.Contracts)
	c.Pools = normalizeGlobalCampaignStrings(c.Pools)
	c.Bridges = normalizeGlobalCampaignStrings(c.Bridges)
	c.ObservationRefs = normalizeGlobalCampaignStrings(c.ObservationRefs)
	c.RelationRefs = normalizeGlobalCampaignStrings(c.RelationRefs)
	c.BridgeLinkRefs = normalizeGlobalCampaignStrings(c.BridgeLinkRefs)
	c.CampaignGenomeRefs = normalizeGlobalCampaignStrings(c.CampaignGenomeRefs)
	c.CampaignTempoRefs = normalizeGlobalCampaignStrings(c.CampaignTempoRefs)
	c.BehaviorSignatureRefs = normalizeGlobalCampaignStrings(c.BehaviorSignatureRefs)
	c.IncidentRefs = normalizeGlobalCampaignStrings(c.IncidentRefs)
	c.VerdictRefs = normalizeGlobalCampaignStrings(c.VerdictRefs)
	c.AttackPathRefs = normalizeGlobalCampaignStrings(c.AttackPathRefs)
	c.MissingEvidence = normalizeGlobalCampaignStrings(c.MissingEvidence)
	c.TransitionReasonCodes = normalizeGlobalCampaignStrings(c.TransitionReasonCodes)
	c.TransitionEvidenceRefs = normalizeGlobalCampaignStrings(c.TransitionEvidenceRefs)

	if c.VerifiedAnchorCount < 0 {
		c.VerifiedAnchorCount = 0
	}
	if c.ObservedAnchorCount < 0 {
		c.ObservedAnchorCount = 0
	}

	// Hard authority boundary: this contract is correlation/evidence context only.
	c.VerdictAuthority = false
	c.GradeAuthority = false
	c.ContainmentAuthority = false
	c.SameOperatorClaim = false
	c.RealWorldIdentityClaim = false
	c.WrongdoingClaim = false
	c.EvidenceHashSHA256 = hashGlobalCampaign(c)
	return c
}

func validGlobalCampaignState(state GlobalCampaignState) bool {
	switch state {
	case GlobalCampaignEmerging, GlobalCampaignActive, GlobalCampaignEscalating,
		GlobalCampaignDegrading, GlobalCampaignDormant, GlobalCampaignClosed, GlobalCampaignReopened:
		return true
	default:
		return false
	}
}

func normalizeGlobalCampaignStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func hashGlobalCampaign(c GlobalCampaign) string {
	c.EvidenceHashSHA256 = ""
	payload, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}
