package handlers

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"koschei/api/internal/services"
)

const globalCampaignRuntimeRulesetVersion = "koschei.global-campaign-runtime.v1"

type globalCampaignRuntimeStatus struct {
	Status                string                                       `json:"status"`
	CampaignRef           string                                       `json:"campaign_ref,omitempty"`
	Revision              uint64                                       `json:"revision,omitempty"`
	Inserted              bool                                         `json:"inserted,omitempty"`
	Idempotent            bool                                         `json:"idempotent,omitempty"`
	EvidenceHash          string                                       `json:"evidence_hash_sha256,omitempty"`
	RadarProjectionStatus string                                       `json:"radar_projection_status,omitempty"`
	RadarFingerprint      string                                       `json:"radar_fingerprint_sha256,omitempty"`
	ThreatFingerprint     string                                       `json:"threat_fingerprint_sha256,omitempty"`
	FabricContractHash    string                                       `json:"fabric_contract_hash_sha256,omitempty"`
	CommandCenter         services.GlobalCampaignCommandCenterSnapshot `json:"command_center,omitempty"`
	ProjectionError       string                                       `json:"projection_error,omitempty"`
	Error                 string                                       `json:"error,omitempty"`
}

func persistGlobalCampaignFromAssembly(ctx context.Context, db *sql.DB, target, network string, assembly unifiedInvestigationAssembly) globalCampaignRuntimeStatus {
	if db == nil {
		return globalCampaignRuntimeStatus{Status: "database_unavailable"}
	}
	input, eligible := globalCampaignMaterializerInputFromAssembly(target, network, assembly)
	if !eligible {
		return globalCampaignRuntimeStatus{Status: "not_eligible"}
	}
	result, err := services.MaterializeAndPersistGlobalCampaign(ctx, db, input)
	if err != nil {
		return globalCampaignRuntimeStatus{Status: "failed", Error: compactCanonicalWorkerError(err)}
	}
	status := "persisted"
	if result.Idempotent {
		status = "idempotent"
	}
	out := globalCampaignRuntimeStatus{
		Status:       status,
		CampaignRef:  result.Campaign.CampaignRef,
		Revision:     result.Campaign.Revision,
		Inserted:     result.Inserted,
		Idempotent:   result.Idempotent,
		EvidenceHash: result.Campaign.EvidenceHashSHA256,
	}
	wireGlobalCampaignOperatorProjection(&out, result.Campaign, assembly, input.ObservedAt)
	return out
}

// wireGlobalCampaignOperatorProjection activates the read-only C6/C8/C9/C17
// chain in the canonical runtime: ARVIS verified evidence -> canonical Global
// Radar snapshot -> campaign Radar projection -> threat-family projection ->
// Fabric observe contract -> command-center monitoring snapshot. None of these
// projections creates verdict, grade, containment or execution authority.
func wireGlobalCampaignOperatorProjection(out *globalCampaignRuntimeStatus, campaign services.GlobalCampaign, assembly unifiedInvestigationAssembly, observedAt time.Time) {
	if out == nil {
		return
	}
	var radar services.GlobalCampaignRadarProjection
	if services.SecurityRadarHasLiveEvidence(assembly.Core.Bundle) {
		snapshot, err := services.ProjectSecurityRadarBundleToGlobalRadar(assembly.Core.Bundle, observedAt)
		if err != nil {
			out.RadarProjectionStatus = "unavailable"
			out.ProjectionError = compactCanonicalWorkerError(err)
		} else {
			radar, err = services.BuildGlobalCampaignRadarProjection(campaign.CampaignRef, snapshot)
			if err != nil {
				out.RadarProjectionStatus = "failed"
				out.ProjectionError = compactCanonicalWorkerError(err)
			} else {
				out.RadarProjectionStatus = "projected"
				out.RadarFingerprint = radar.FingerprintSHA256
			}
		}
	} else {
		out.RadarProjectionStatus = "no_live_verified_evidence"
	}

	threat, err := services.BuildGlobalCampaignThreatFamilies(services.GlobalCampaignThreatFamilyInput{
		CampaignRef:   campaign.CampaignRef,
		Radar:         radar,
		TempoReports:  []services.CampaignTempoFingerprintReport{assembly.CampaignTempo},
		ThreatReports: []services.ThreatAnticipationReport{assembly.Threat},
	})
	if err != nil {
		out.ProjectionError = firstNonEmptyString(out.ProjectionError, compactCanonicalWorkerError(err))
		return
	}
	out.ThreatFingerprint = threat.FingerprintSHA256

	temporal := services.GlobalCampaignTemporalCorrelationReport{}
	if radar.Version != "" {
		temporal = radar.Temporal
	}
	fabric, err := services.BuildFabricCampaignEvidenceContract(campaign, temporal, radar, threat, nil)
	if err != nil {
		out.ProjectionError = firstNonEmptyString(out.ProjectionError, compactCanonicalWorkerError(err))
		return
	}
	out.FabricContractHash = fabric.ContractHashSHA256

	commandCenter, err := services.BuildGlobalCampaignCommandCenterSnapshot(services.GlobalCampaignCommandCenterInput{
		Campaign:   campaign,
		Threat:     threat,
		Fabric:     fabric,
		ObservedAt: observedAt,
	})
	if err != nil {
		out.ProjectionError = firstNonEmptyString(out.ProjectionError, compactCanonicalWorkerError(err))
		return
	}
	out.CommandCenter = commandCenter
}

// globalCampaignMaterializerInputFromAssembly projects already-persisted ARVIS,
// Actor Defense, incident-memory, campaign-genome and tempo evidence into the
// reference-first Global Campaign contract. It deliberately withholds generic
// target-only scans so a single token observation is not mislabeled a campaign.
func globalCampaignMaterializerInputFromAssembly(target, network string, assembly unifiedInvestigationAssembly) (services.GlobalCampaignMaterializerInput, bool) {
	target = strings.TrimSpace(target)
	network = strings.TrimSpace(network)
	if network == "" {
		network = "solana-mainnet"
	}

	input := services.GlobalCampaignMaterializerInput{
		ObservedAt:     time.Now().UTC(),
		Networks:       []string{network},
		Subjects:       []string{target},
		Assets:         []string{target},
		RulesetVersion: globalCampaignRuntimeRulesetVersion,
	}
	if !assembly.UnifiedVerdict.GeneratedAt.IsZero() {
		input.ObservedAt = assembly.UnifiedVerdict.GeneratedAt.UTC()
	}
	creator := strings.TrimSpace(assembly.Creator)
	if creator != "" {
		input.Subjects = append(input.Subjects, creator)
		input.Actors = append(input.Actors, creator)
	}

	verifiedObservationRefs := map[string]bool{}
	observedObservationRefs := map[string]bool{}
	for _, evidence := range assembly.CombinedEvidence {
		ref := strings.TrimSpace(evidence.EvidenceKey)
		if ref == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(evidence.VerificationStatus)) {
		case "verified":
			verifiedObservationRefs[ref] = true
		case "observed":
			observedObservationRefs[ref] = true
		default:
			continue
		}
		input.ObservationRefs = append(input.ObservationRefs, ref)
		if actor := strings.TrimSpace(evidence.ActorWallet); actor != "" {
			input.Actors = append(input.Actors, actor)
			input.Subjects = append(input.Subjects, actor)
		}
		if counterpart := strings.TrimSpace(evidence.CounterpartID); counterpart != "" {
			input.Subjects = append(input.Subjects, counterpart)
		}
		if mint := strings.TrimSpace(evidence.TokenMint); mint != "" {
			input.Assets = append(input.Assets, mint)
		}
	}

	if assembly.CampaignGenome.Complete {
		if ref := strings.TrimSpace(assembly.CampaignGenome.GenomeID); ref != "" {
			// GenomeID is derived from the normalized technical pattern and is the
			// intentional cross-wallet correlation anchor. It is not an identity claim.
			input.CampaignGenomeRefs = append(input.CampaignGenomeRefs, ref)
		}
		input.VerifiedAnchorCount += assembly.CampaignGenome.VerifiedDescriptorCount
		input.ObservedAnchorCount += assembly.CampaignGenome.ObservedDescriptorCount
	}
	for _, match := range assembly.CampaignGenomeMatches.Matches {
		if actor := strings.TrimSpace(match.ActorWallet); actor != "" {
			input.Actors = append(input.Actors, actor)
			input.Subjects = append(input.Subjects, actor)
		}
	}

	if assembly.CampaignTempo.Complete {
		if ref := strings.TrimSpace(assembly.CampaignTempo.FingerprintSHA256); ref != "" {
			input.CampaignTempoRefs = append(input.CampaignTempoRefs, ref)
		}
		for _, path := range assembly.CampaignTempo.Paths {
			input.Actors = append(input.Actors, path.ActorWallet, path.FundingSourceWallet)
			input.Assets = append(input.Assets, path.TokenMint)
			input.ObservationRefs = append(input.ObservationRefs, path.EvidenceRefs...)
		}
	}

	for _, match := range assembly.BehavioralSignatures.Matches {
		if !match.Triggered || len(match.EvidenceRefs) == 0 {
			continue
		}
		if ref := behaviorSignatureCampaignRef(match); ref != "" {
			input.BehaviorSignatureRefs = append(input.BehaviorSignatureRefs, ref)
		}
		input.Actors = append(input.Actors, match.ActorWallets...)
		input.Assets = append(input.Assets, match.Targets...)
		input.IncidentRefs = append(input.IncidentRefs, match.IncidentKeys...)
		input.ObservationRefs = append(input.ObservationRefs, match.EvidenceRefs...)
	}

	appendIncidentView := func(view services.SecurityIncidentCorpusView) {
		for _, record := range view.Records {
			if ref := strings.TrimSpace(record.IncidentKey); ref != "" {
				input.IncidentRefs = append(input.IncidentRefs, ref)
			}
			if record.Network != "" {
				input.Networks = append(input.Networks, record.Network)
			}
			if record.Target != "" {
				input.Subjects = append(input.Subjects, record.Target)
				input.Assets = append(input.Assets, record.Target)
			}
			if record.ActorWallet != "" {
				input.Subjects = append(input.Subjects, record.ActorWallet)
				input.Actors = append(input.Actors, record.ActorWallet)
			}
			for _, ref := range record.Evidence {
				if strings.TrimSpace(ref) != "" {
					input.ObservationRefs = append(input.ObservationRefs, ref)
				}
			}
		}
	}
	appendIncidentView(assembly.IncidentCorpus)
	appendIncidentView(assembly.ActorIncidentHistory)

	if assembly.UnifiedVerdict.Signed {
		if ref := strings.TrimSpace(assembly.UnifiedVerdict.PayloadHash); ref != "" {
			input.VerdictRefs = append(input.VerdictRefs, ref)
		} else if ref := strings.TrimSpace(assembly.UnifiedVerdict.Digest); ref != "" {
			input.VerdictRefs = append(input.VerdictRefs, ref)
		}
	}

	for _, pathway := range assembly.Threat.Pathways {
		if ref := threatPathCampaignRef(pathway); ref != "" {
			input.AttackPathRefs = append(input.AttackPathRefs, ref)
		}
	}
	input.MissingEvidence = append(input.MissingEvidence, assembly.Threat.MissingEvidence...)

	input.VerifiedAnchorCount += len(verifiedObservationRefs)
	input.ObservedAnchorCount += len(observedObservationRefs)

	// At least one campaign-specific correlation anchor is required. Generic
	// ARVIS observations alone remain Global Radar observations and do not mint a
	// campaign identity.
	eligible := len(nonEmptyHandlerStrings(input.CampaignGenomeRefs)) > 0 ||
		len(nonEmptyHandlerStrings(input.IncidentRefs)) > 0 ||
		len(nonEmptyHandlerStrings(input.BehaviorSignatureRefs)) > 0 ||
		len(nonEmptyHandlerStrings(input.CampaignTempoRefs)) > 0
	return input, eligible
}

func behaviorSignatureCampaignRef(match services.BehavioralSignatureMatch) string {
	id := strings.TrimSpace(match.SignatureID)
	refs := nonEmptyHandlerStrings(match.EvidenceRefs)
	if id == "" || len(refs) == 0 {
		return ""
	}
	sort.Strings(refs)
	digest := sha256.Sum256([]byte(id + "\x1f" + strings.Join(refs, "\x1f")))
	return "KBEH1-" + strings.ToUpper(hex.EncodeToString(digest[:8]))
}

func threatPathCampaignRef(pathway services.ThreatPathway) string {
	id := strings.TrimSpace(pathway.ID)
	refs := nonEmptyHandlerStrings(pathway.EvidenceKeys)
	status := strings.ToLower(strings.TrimSpace(pathway.EvidenceStatus))
	if id == "" || len(refs) == 0 || (status != "verified" && status != "observed") {
		return ""
	}
	sort.Strings(refs)
	digest := sha256.Sum256([]byte(id + "\x1f" + strings.Join(refs, "\x1f")))
	return "KPATH1-" + strings.ToUpper(hex.EncodeToString(digest[:8]))
}

func nonEmptyHandlerStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
