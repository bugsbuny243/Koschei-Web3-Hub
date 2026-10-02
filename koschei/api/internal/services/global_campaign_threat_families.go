package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

const GlobalCampaignThreatFamiliesVersion = "koschei.global-campaign-threat-families.v1"

const (
	GlobalCampaignThreatFamilyCrossChain = "KCAM-TF-001"
	GlobalCampaignThreatFamilyTempo      = "KCAM-TF-002"
	GlobalCampaignThreatFamilyExit       = "KCAM-TF-003"
	GlobalCampaignThreatFamilyAuthority  = "KCAM-TF-004"
	GlobalCampaignThreatFamilyLiquidity  = "KCAM-TF-005"
	GlobalCampaignThreatFamilyCoordExit  = "KCAM-TF-006"
	GlobalCampaignThreatFamilyCreator    = "KCAM-TF-007"
)

var (
	ErrGlobalCampaignThreatCampaignRefRequired = errors.New("global campaign threat families require campaign reference")
	ErrGlobalCampaignThreatCampaignMismatch    = errors.New("global campaign threat family input belongs to another campaign")
	ErrGlobalCampaignThreatRadarInvalid        = errors.New("global campaign threat family radar projection is invalid")
)

type GlobalCampaignThreatFamilyInput struct {
	CampaignRef   string                           `json:"campaign_ref"`
	Radar         GlobalCampaignRadarProjection    `json:"radar"`
	TempoReports  []CampaignTempoFingerprintReport `json:"tempo_reports,omitempty"`
	ThreatReports []ThreatAnticipationReport       `json:"threat_reports,omitempty"`
}

type GlobalCampaignThreatFamilyFinding struct {
	RuleID          string   `json:"rule_id"`
	Family          string   `json:"family"`
	Status          string   `json:"status"`
	EvidenceState   string   `json:"evidence_state"`
	WatchOnly       bool     `json:"watch_only"`
	Targets         []string `json:"targets,omitempty"`
	Networks        []string `json:"networks,omitempty"`
	SourceRefs      []string `json:"source_refs,omitempty"`
	EvidenceKeys    []string `json:"evidence_keys,omitempty"`
	PathwayIDs      []string `json:"pathway_ids,omitempty"`
	PathwayStatuses []string `json:"pathway_statuses,omitempty"`
	Basis           string   `json:"basis"`
	Limitations     []string `json:"limitations,omitempty"`
}

type GlobalCampaignThreatFamilyReport struct {
	Version                string                              `json:"version"`
	CampaignRef            string                              `json:"campaign_ref"`
	Status                 string                              `json:"status"`
	Complete               bool                                `json:"complete"`
	FindingCount           int                                 `json:"finding_count"`
	Findings               []GlobalCampaignThreatFamilyFinding `json:"findings"`
	FingerprintSHA256      string                              `json:"fingerprint_sha256"`
	VerdictAuthority       bool                                `json:"verdict_authority"`
	GradeAuthority         bool                                `json:"grade_authority"`
	ContainmentAuthority   bool                                `json:"containment_authority"`
	SameOperatorClaim      bool                                `json:"same_operator_claim"`
	RealWorldIdentityClaim bool                                `json:"real_world_identity_claim"`
	WrongdoingClaim        bool                                `json:"wrongdoing_claim"`
	Limitations            []string                            `json:"limitations"`
}

// BuildGlobalCampaignThreatFamilies deterministically projects existing
// evidence-backed Radar, tempo, and threat-pathway contracts into campaign-level
// technical families. It creates no new verdict, grade, identity, intent,
// wrongdoing, or containment authority.
func BuildGlobalCampaignThreatFamilies(in GlobalCampaignThreatFamilyInput) (GlobalCampaignThreatFamilyReport, error) {
	campaignRef := strings.TrimSpace(in.CampaignRef)
	if campaignRef == "" {
		return GlobalCampaignThreatFamilyReport{}, ErrGlobalCampaignThreatCampaignRefRequired
	}

	out := GlobalCampaignThreatFamilyReport{
		Version:     GlobalCampaignThreatFamiliesVersion,
		CampaignRef: campaignRef,
		Status:      "no_supported_campaign_threat_family",
		Complete:    true,
		Findings:    []GlobalCampaignThreatFamilyFinding{},
		Limitations: []string{
			"Campaign threat families are deterministic evidence projections, not predictions of intent or future wrongdoing.",
			"Cross-chain continuity proves an explicit transfer link only and does not identify a common real-world operator.",
			"Recurring tempo remains watch context even when its underlying path evidence is verified.",
			"Existing ARVIS deterministic verdict, grade and signature authority remain unchanged.",
			"No finding authorizes containment or any response action.",
		},
	}

	if in.Radar.Version != "" {
		if in.Radar.Version != GlobalCampaignRadarProjectionVersion || strings.TrimSpace(in.Radar.CampaignRef) != campaignRef {
			return GlobalCampaignThreatFamilyReport{}, ErrGlobalCampaignThreatCampaignMismatch
		}
		if strings.TrimSpace(in.Radar.FingerprintSHA256) == "" {
			return GlobalCampaignThreatFamilyReport{}, ErrGlobalCampaignThreatRadarInvalid
		}
		if finding, ok := globalCampaignCrossChainThreatFamily(in.Radar); ok {
			out.Findings = append(out.Findings, finding)
		}
	}

	if finding, ok, complete := globalCampaignTempoThreatFamily(in.TempoReports); ok {
		out.Findings = append(out.Findings, finding)
		if !complete {
			out.Complete = false
		}
	} else if !complete {
		out.Complete = false
	}

	pathFindings, pathComplete := globalCampaignPathwayThreatFamilies(in.ThreatReports)
	out.Findings = append(out.Findings, pathFindings...)
	if !pathComplete {
		out.Complete = false
	}

	sort.SliceStable(out.Findings, func(i, j int) bool {
		if out.Findings[i].RuleID != out.Findings[j].RuleID {
			return out.Findings[i].RuleID < out.Findings[j].RuleID
		}
		return out.Findings[i].Family < out.Findings[j].Family
	})
	out.FindingCount = len(out.Findings)
	if out.FindingCount > 0 {
		out.Status = "campaign_threat_families_observed"
	}
	if in.Radar.Version == "" && len(in.TempoReports) == 0 && len(in.ThreatReports) == 0 {
		out.Complete = false
		out.Limitations = append(out.Limitations, "No campaign evidence inputs were supplied; absence of a family is inconclusive.")
	}
	out.FingerprintSHA256 = hashGlobalCampaignThreatFamilyReport(out)
	return out, nil
}

func globalCampaignCrossChainThreatFamily(radar GlobalCampaignRadarProjection) (GlobalCampaignThreatFamilyFinding, bool) {
	if len(radar.CrossNetworkContinuity) == 0 {
		return GlobalCampaignThreatFamilyFinding{}, false
	}
	finding := GlobalCampaignThreatFamilyFinding{
		RuleID:        GlobalCampaignThreatFamilyCrossChain,
		Family:        "cross_chain_transfer_continuity",
		Status:        "verified_technical_continuity",
		EvidenceState: IntelligenceEvidenceVerified,
		WatchOnly:     true,
		Basis:         "Canonical Global Radar bridge-link evidence connects campaign observations across distinct networks.",
		Limitations: []string{
			"A verified bridge transfer is a technical continuity fact only; it does not prove common control, malicious intent or wrongdoing.",
		},
	}
	for _, continuity := range radar.CrossNetworkContinuity {
		if continuity.EvidenceState != IntelligenceEvidenceVerified || strings.TrimSpace(continuity.BridgeLinkRef) == "" {
			continue
		}
		finding.Targets = append(finding.Targets, continuity.SourceSubjectID, continuity.DestinationSubjectID)
		finding.Networks = append(finding.Networks, continuity.SourceNetwork, continuity.DestinationNetwork)
		finding.SourceRefs = append(finding.SourceRefs, continuity.BridgeLinkRef, continuity.LinkEvidenceRef, continuity.RelationRef)
	}
	finding.Targets = normalizeGlobalCampaignStrings(finding.Targets)
	finding.Networks = normalizeGlobalCampaignStrings(finding.Networks)
	finding.SourceRefs = normalizeGlobalCampaignStrings(finding.SourceRefs)
	if len(finding.SourceRefs) == 0 {
		return GlobalCampaignThreatFamilyFinding{}, false
	}
	return finding, true
}

func globalCampaignTempoThreatFamily(reports []CampaignTempoFingerprintReport) (GlobalCampaignThreatFamilyFinding, bool, bool) {
	complete := true
	finding := GlobalCampaignThreatFamilyFinding{
		RuleID:        GlobalCampaignThreatFamilyTempo,
		Family:        "recurring_verified_campaign_tempo",
		Status:        "observed_watch",
		EvidenceState: IntelligenceEvidenceObserved,
		WatchOnly:     true,
		Basis:         "Distinct funded on-chain actors repeat the same complete verified campaign-tempo profile.",
		Limitations: []string{
			"Matching time buckets are investigation context only and do not prove shared control, identity, intent or wrongdoing.",
		},
	}
	triggered := false
	for _, report := range reports {
		if !report.Complete {
			complete = false
		}
		match := behaviorSignatureCampaignTempoRecurrence(report)
		if !match.Triggered {
			continue
		}
		triggered = true
		finding.Targets = append(finding.Targets, match.Targets...)
		finding.Networks = append(finding.Networks, report.Network)
		finding.SourceRefs = append(finding.SourceRefs, match.EvidenceRefs...)
	}
	if !triggered {
		return GlobalCampaignThreatFamilyFinding{}, false, complete
	}
	finding.Targets = normalizeGlobalCampaignStrings(finding.Targets)
	finding.Networks = normalizeGlobalCampaignStrings(finding.Networks)
	finding.SourceRefs = normalizeGlobalCampaignStrings(finding.SourceRefs)
	return finding, true, complete
}

type globalCampaignThreatFamilyAccumulator struct {
	finding GlobalCampaignThreatFamilyFinding
	states  []string
}

func globalCampaignPathwayThreatFamilies(reports []ThreatAnticipationReport) ([]GlobalCampaignThreatFamilyFinding, bool) {
	complete := true
	byFamily := map[string]*globalCampaignThreatFamilyAccumulator{}
	for _, report := range reports {
		if len(report.MissingEvidence) > 0 || report.Status == "insufficient_evidence" {
			complete = false
		}
		target := strings.TrimSpace(report.Target)
		for _, path := range report.Pathways {
			ruleID, family, ok := globalCampaignThreatPathwayFamily(path.ID)
			if !ok || !globalCampaignThreatPathwayActive(path) {
				continue
			}
			acc := byFamily[family]
			if acc == nil {
				acc = &globalCampaignThreatFamilyAccumulator{finding: GlobalCampaignThreatFamilyFinding{
					RuleID:      ruleID,
					Family:      family,
					Status:      "evidence_backed_pathway_context",
					WatchOnly:   true,
					Basis:       "Existing ThreatAnticipation pathway evidence is present on one or more campaign targets.",
					Limitations: []string{"A technically open or observed pathway is capacity/context evidence and does not prove intent to use it."},
				}}
				byFamily[family] = acc
			}
			if target != "" {
				acc.finding.Targets = append(acc.finding.Targets, target)
			}
			acc.finding.EvidenceKeys = append(acc.finding.EvidenceKeys, path.EvidenceKeys...)
			acc.finding.PathwayIDs = append(acc.finding.PathwayIDs, path.ID)
			acc.finding.PathwayStatuses = append(acc.finding.PathwayStatuses, path.Status)
			acc.finding.SourceRefs = append(acc.finding.SourceRefs, "threat:"+target+":"+strings.TrimSpace(path.ID))
			acc.states = append(acc.states, strings.ToLower(strings.TrimSpace(path.EvidenceStatus)))
		}
	}

	families := make([]string, 0, len(byFamily))
	for family := range byFamily {
		families = append(families, family)
	}
	sort.Strings(families)
	out := make([]GlobalCampaignThreatFamilyFinding, 0, len(families))
	for _, family := range families {
		acc := byFamily[family]
		acc.finding.Targets = normalizeGlobalCampaignStrings(acc.finding.Targets)
		acc.finding.SourceRefs = normalizeGlobalCampaignStrings(acc.finding.SourceRefs)
		acc.finding.EvidenceKeys = normalizeGlobalCampaignStrings(acc.finding.EvidenceKeys)
		acc.finding.PathwayIDs = normalizeGlobalCampaignStrings(acc.finding.PathwayIDs)
		acc.finding.PathwayStatuses = normalizeGlobalCampaignStrings(acc.finding.PathwayStatuses)
		acc.finding.EvidenceState = globalCampaignStrongestThreatEvidenceState(acc.states)
		out = append(out, acc.finding)
	}
	return out, complete
}

func globalCampaignThreatPathwayFamily(pathID string) (string, string, bool) {
	switch strings.TrimSpace(pathID) {
	case "dominant_holder_exit":
		return GlobalCampaignThreatFamilyExit, "market_exit_capacity", true
	case "mint_inflation", "freeze_abuse":
		return GlobalCampaignThreatFamilyAuthority, "authority_control_exposure", true
	case "liquidity_removal":
		return GlobalCampaignThreatFamilyLiquidity, "liquidity_control_exposure", true
	case "coordinated_holder_exit":
		return GlobalCampaignThreatFamilyCoordExit, "coordinated_exit_pattern", true
	case "creator_sell_acceleration":
		return GlobalCampaignThreatFamilyCreator, "creator_exit_acceleration", true
	default:
		return "", "", false
	}
}

func globalCampaignThreatPathwayActive(path ThreatPathway) bool {
	state := strings.ToLower(strings.TrimSpace(path.EvidenceStatus))
	if state != IntelligenceEvidenceVerified && state != IntelligenceEvidenceObserved {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(path.Status)) {
	case "open", "observed", "watch", "limited":
		return true
	default:
		return false
	}
}

func globalCampaignStrongestThreatEvidenceState(states []string) string {
	best := "unknown"
	for _, state := range states {
		switch strings.ToLower(strings.TrimSpace(state)) {
		case IntelligenceEvidenceVerified:
			return IntelligenceEvidenceVerified
		case IntelligenceEvidenceObserved:
			best = IntelligenceEvidenceObserved
		}
	}
	return best
}

func hashGlobalCampaignThreatFamilyReport(report GlobalCampaignThreatFamilyReport) string {
	report.FingerprintSHA256 = ""
	payload, err := json.Marshal(report)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}
