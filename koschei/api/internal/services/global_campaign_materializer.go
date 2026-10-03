package services

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

const GlobalCampaignMaterializerVersion = "koschei.global-campaign-materializer.v1"

// GlobalCampaignMaterializerInput is intentionally reference-first. It accepts
// canonical identities from existing subsystems and does not duplicate raw
// Global Radar evidence payloads or create a second graph model.
type GlobalCampaignMaterializerInput struct {
	ExistingCampaignRef string
	ExistingRevision    uint64
	ExistingState       GlobalCampaignState

	ObservedAt time.Time

	Networks  []string
	Subjects  []string
	Actors    []string
	Assets    []string
	Contracts []string
	Pools     []string
	Bridges   []string

	ObservationRefs       []string
	RelationRefs          []string
	BridgeLinkRefs        []string
	CampaignGenomeRefs    []string
	CampaignTempoRefs     []string
	BehaviorSignatureRefs []string
	IncidentRefs          []string
	VerdictRefs           []string
	AttackPathRefs        []string
	MissingEvidence       []string

	VerifiedAnchorCount int
	ObservedAnchorCount int
	RulesetVersion      string
}

// MaterializeGlobalCampaign produces a deterministic campaign snapshot from an
// unordered set of already-canonical evidence references. Arrival order does
// not participate in identity or evidence hashing.
func MaterializeGlobalCampaign(in GlobalCampaignMaterializerInput) GlobalCampaign {
	campaignRef := strings.TrimSpace(in.ExistingCampaignRef)
	if campaignRef == "" {
		campaignRef = globalCampaignRefFromMaterial(in)
	}
	revision := in.ExistingRevision
	if revision == 0 {
		revision = 1
	}
	state := in.ExistingState
	if !validGlobalCampaignState(state) {
		state = GlobalCampaignEmerging
	}

	campaign := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:           campaignRef,
		Revision:              revision,
		State:                 state,
		FirstObservedAt:       in.ObservedAt,
		LastObservedAt:        in.ObservedAt,
		Networks:              in.Networks,
		Subjects:              in.Subjects,
		Actors:                in.Actors,
		Assets:                in.Assets,
		Contracts:             in.Contracts,
		Pools:                 in.Pools,
		Bridges:               in.Bridges,
		ObservationRefs:       in.ObservationRefs,
		RelationRefs:          in.RelationRefs,
		BridgeLinkRefs:        in.BridgeLinkRefs,
		CampaignGenomeRefs:    in.CampaignGenomeRefs,
		CampaignTempoRefs:     in.CampaignTempoRefs,
		BehaviorSignatureRefs: in.BehaviorSignatureRefs,
		IncidentRefs:          in.IncidentRefs,
		VerdictRefs:           in.VerdictRefs,
		AttackPathRefs:        in.AttackPathRefs,
		MissingEvidence:       in.MissingEvidence,
		VerifiedAnchorCount:   in.VerifiedAnchorCount,
		ObservedAnchorCount:   in.ObservedAnchorCount,
		RulesetVersion:        in.RulesetVersion,
	})
	return campaign
}

func globalCampaignRefFromMaterial(in GlobalCampaignMaterializerInput) string {
	anchors := make([]string, 0)
	anchors = append(anchors, in.RelationRefs...)
	anchors = append(anchors, in.BridgeLinkRefs...)
	anchors = append(anchors, in.CampaignGenomeRefs...)
	anchors = append(anchors, in.IncidentRefs...)
	anchors = normalizeGlobalCampaignStrings(anchors)
	if len(anchors) == 0 {
		anchors = append(anchors, in.ObservationRefs...)
		anchors = append(anchors, in.BehaviorSignatureRefs...)
		anchors = append(anchors, in.CampaignTempoRefs...)
		anchors = append(anchors, in.AttackPathRefs...)
		anchors = normalizeGlobalCampaignStrings(anchors)
	}
	if len(anchors) == 0 {
		anchors = append(anchors, in.Subjects...)
		anchors = append(anchors, in.Actors...)
		anchors = append(anchors, in.Assets...)
		anchors = normalizeGlobalCampaignStrings(anchors)
	}
	if len(anchors) == 0 {
		return ""
	}
	sort.Strings(anchors)
	canonical := strings.Join(anchors, "\x1f")
	digest := sha256.Sum256([]byte(canonical))
	return "KCAM1-" + strings.ToUpper(hex.EncodeToString(digest[:8]))
}
