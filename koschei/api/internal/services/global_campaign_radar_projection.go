package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

const GlobalCampaignRadarProjectionVersion = "koschei.global-campaign-radar-projection.v1"

var (
	ErrGlobalCampaignRadarCampaignRefRequired = errors.New("global campaign radar projection requires campaign reference")
	ErrGlobalCampaignRadarSnapshotInvalid     = errors.New("global campaign radar projection requires canonical Global Radar snapshot")
	ErrGlobalCampaignRadarEvidenceInvalid     = errors.New("global campaign radar projection contains non-canonical evidence binding")
)

// GlobalCampaignCrossNetworkContinuity is a projection of one canonical
// GlobalRadarBridgeLink. It does not infer actor identity, common control,
// malicious intent, a campaign verdict, or containment authority.
type GlobalCampaignCrossNetworkContinuity struct {
	BridgeLinkRef              string `json:"bridge_link_ref"`
	BridgeProtocol             string `json:"bridge_protocol"`
	BridgeTransferID           string `json:"bridge_transfer_id"`
	SourceObservationRef       string `json:"source_observation_ref"`
	DestinationObservationRef  string `json:"destination_observation_ref"`
	SourceSubjectID            string `json:"source_subject_id"`
	DestinationSubjectID       string `json:"destination_subject_id"`
	SourceNetwork              string `json:"source_network"`
	DestinationNetwork         string `json:"destination_network"`
	SourceTransactionHash      string `json:"source_transaction_hash"`
	DestinationTransactionHash string `json:"destination_transaction_hash"`
	LinkEvidenceRef            string `json:"link_evidence_ref"`
	RelationRef                string `json:"relation_ref"`
	EvidenceState              string `json:"evidence_state"`
	VerificationBoundary       string `json:"verification_boundary"`
	TemporalAvailable          bool   `json:"temporal_available"`
	TemporalWindow             string `json:"temporal_window,omitempty"`
	TemporalDeltaSeconds       int64  `json:"temporal_delta_seconds"`
	TemporalEvidenceState      string `json:"temporal_evidence_state,omitempty"`
}

// GlobalCampaignRadarProjection is a read-only campaign-centered projection of
// canonical Global Radar observations, relations, and bridge links. It reuses
// the existing Radar graph instead of creating a second relationship authority.
type GlobalCampaignRadarProjection struct {
	Version                string                                  `json:"version"`
	CampaignRef            string                                  `json:"campaign_ref"`
	SnapshotGeneratedAt    time.Time                               `json:"snapshot_generated_at"`
	Events                 []GlobalCampaignTemporalEvent           `json:"events"`
	CrossNetworkContinuity []GlobalCampaignCrossNetworkContinuity  `json:"cross_network_continuity"`
	Temporal               GlobalCampaignTemporalCorrelationReport `json:"temporal"`
	FingerprintSHA256      string                                  `json:"fingerprint_sha256"`
	VerdictAuthority       bool                                    `json:"verdict_authority"`
	GradeAuthority         bool                                    `json:"grade_authority"`
	ContainmentAuthority   bool                                    `json:"containment_authority"`
	SameOperatorClaim      bool                                    `json:"same_operator_claim"`
	RealWorldIdentityClaim bool                                    `json:"real_world_identity_claim"`
	WrongdoingClaim        bool                                    `json:"wrongdoing_claim"`
	Limitations            []string                                `json:"limitations"`
}

// BuildGlobalCampaignRadarProjection validates the supplied snapshot back
// through the canonical Global Radar builders before projecting it into C6
// temporal events. Verified relation refs can only originate from canonical
// VERIFIED relation edges. Verified bridge refs can only originate from a
// canonical GlobalRadarBridgeLink rebuilt from its exact source/destination
// observations and explicit bridge protocol + transfer identity.
func BuildGlobalCampaignRadarProjection(campaignRef string, snapshot GlobalRadarSnapshot) (GlobalCampaignRadarProjection, error) {
	campaignRef = strings.TrimSpace(campaignRef)
	if campaignRef == "" {
		return GlobalCampaignRadarProjection{}, ErrGlobalCampaignRadarCampaignRefRequired
	}
	canonical, err := validateCanonicalGlobalCampaignRadarSnapshot(snapshot)
	if err != nil {
		return GlobalCampaignRadarProjection{}, err
	}

	relationRefsBySubject := make(map[string][]string)
	verifiedRelationRefsBySubject := make(map[string][]string)
	for _, relation := range canonical.Relations {
		for _, subjectID := range []string{relation.Source.ID, relation.Target.ID} {
			relationRefsBySubject[subjectID] = append(relationRefsBySubject[subjectID], relation.EdgeID)
			if relation.Status == IntelligenceEvidenceVerified {
				verifiedRelationRefsBySubject[subjectID] = append(verifiedRelationRefsBySubject[subjectID], relation.EdgeID)
			}
		}
	}

	bridgeRefsByObservation := make(map[string][]string)
	verifiedBridgeRefsByObservation := make(map[string][]string)
	for _, link := range canonical.BridgeLinks {
		for _, observationID := range []string{link.SourceObservation.ObservationID, link.DestinationObservation.ObservationID} {
			bridgeRefsByObservation[observationID] = append(bridgeRefsByObservation[observationID], link.LinkID)
			verifiedBridgeRefsByObservation[observationID] = append(verifiedBridgeRefsByObservation[observationID], link.LinkID)
		}
	}

	events := make([]GlobalCampaignTemporalEvent, 0, len(canonical.Observations))
	for _, observation := range canonical.Observations {
		events = append(events, GlobalCampaignTemporalEvent{
			Ref:                    observation.ObservationID,
			SubjectID:              observation.Subject.ID,
			Network:                observation.Subject.Network,
			Kind:                   observation.ObservationKind,
			ObservedAt:             observation.ObservedAt,
			EvidenceState:          observation.Evidence.Status,
			RelationRefs:           normalizeGlobalCampaignStrings(relationRefsBySubject[observation.Subject.ID]),
			VerifiedRelationRefs:   normalizeGlobalCampaignStrings(verifiedRelationRefsBySubject[observation.Subject.ID]),
			BridgeLinkRefs:         normalizeGlobalCampaignStrings(bridgeRefsByObservation[observation.ObservationID]),
			VerifiedBridgeLinkRefs: normalizeGlobalCampaignStrings(verifiedBridgeRefsByObservation[observation.ObservationID]),
		})
	}
	normalizedEvents, invalidated, err := normalizeGlobalCampaignTemporalEvents(events)
	if err != nil {
		return GlobalCampaignRadarProjection{}, err
	}
	if len(invalidated) != 0 {
		return GlobalCampaignRadarProjection{}, fmt.Errorf("%w: canonical snapshot produced invalidated events", ErrGlobalCampaignRadarEvidenceInvalid)
	}

	temporal, err := BuildGlobalCampaignTemporalCorrelation(campaignRef, normalizedEvents)
	if err != nil {
		return GlobalCampaignRadarProjection{}, err
	}

	continuity := make([]GlobalCampaignCrossNetworkContinuity, 0, len(canonical.BridgeLinks))
	for _, link := range canonical.BridgeLinks {
		item := GlobalCampaignCrossNetworkContinuity{
			BridgeLinkRef:              link.LinkID,
			BridgeProtocol:             link.BridgeProtocol,
			BridgeTransferID:           link.BridgeTransferID,
			SourceObservationRef:       link.SourceObservation.ObservationID,
			DestinationObservationRef:  link.DestinationObservation.ObservationID,
			SourceSubjectID:            link.SourceObservation.Subject.ID,
			DestinationSubjectID:       link.DestinationObservation.Subject.ID,
			SourceNetwork:              normalizeRadarNetwork(link.SourceObservation.Subject.Network),
			DestinationNetwork:         normalizeRadarNetwork(link.DestinationObservation.Subject.Network),
			SourceTransactionHash:      strings.TrimSpace(link.SourceObservation.Evidence.TransactionHash),
			DestinationTransactionHash: strings.TrimSpace(link.DestinationObservation.Evidence.TransactionHash),
			LinkEvidenceRef:            link.LinkEvidence.ID,
			RelationRef:                link.Relation.EdgeID,
			EvidenceState:              IntelligenceEvidenceVerified,
			VerificationBoundary:       link.VerificationBoundary,
		}
		if correlation, ok := findGlobalCampaignTemporalCorrelation(temporal.Correlations, item.SourceObservationRef, item.DestinationObservationRef); ok {
			item.TemporalAvailable = true
			item.TemporalWindow = correlation.Window
			item.TemporalDeltaSeconds = correlation.DeltaSeconds
			item.TemporalEvidenceState = correlation.EvidenceState
		}
		continuity = append(continuity, item)
	}
	sort.SliceStable(continuity, func(i, j int) bool {
		if continuity[i].BridgeLinkRef != continuity[j].BridgeLinkRef {
			return continuity[i].BridgeLinkRef < continuity[j].BridgeLinkRef
		}
		if continuity[i].SourceObservationRef != continuity[j].SourceObservationRef {
			return continuity[i].SourceObservationRef < continuity[j].SourceObservationRef
		}
		return continuity[i].DestinationObservationRef < continuity[j].DestinationObservationRef
	})

	out := GlobalCampaignRadarProjection{
		Version:                GlobalCampaignRadarProjectionVersion,
		CampaignRef:            campaignRef,
		SnapshotGeneratedAt:    canonical.GeneratedAt.UTC(),
		Events:                 normalizedEvents,
		CrossNetworkContinuity: continuity,
		Temporal:               temporal,
		Limitations: []string{
			"This projection reuses canonical Global Radar graph evidence and does not create a second relationship authority.",
			"A verified bridge continuity proves the explicit transfer link only; it does not prove common actor identity, control, intent or wrongdoing.",
			"Temporal correlation remains separately bounded by C6 evidence-state and event-time rules; a verified bridge link does not upgrade unrelated observations.",
			"ARVIS verdict, grade and signing authority remain outside this projection; containment authority is never created here.",
		},
	}
	out.FingerprintSHA256 = hashGlobalCampaignRadarProjection(out)
	return out, nil
}

func validateCanonicalGlobalCampaignRadarSnapshot(snapshot GlobalRadarSnapshot) (GlobalRadarSnapshot, error) {
	if snapshot.SchemaVersion != GlobalRadarSnapshotSchemaVersion || snapshot.GeneratedAt.IsZero() {
		return GlobalRadarSnapshot{}, fmt.Errorf("%w: schema or generated_at", ErrGlobalCampaignRadarSnapshotInvalid)
	}

	observations := make([]GlobalRadarObservation, 0, len(snapshot.Observations))
	observationByID := make(map[string]GlobalRadarObservation, len(snapshot.Observations))
	evidenceByID := make(map[string]IntelligenceEvidence)
	for _, observation := range snapshot.Observations {
		rebuilt, err := BuildGlobalRadarObservation(observation.ObservationKind, observation.Subject, observation.Evidence)
		if err != nil || !reflect.DeepEqual(rebuilt, observation) {
			return GlobalRadarSnapshot{}, fmt.Errorf("%w: observation=%s", ErrGlobalCampaignRadarEvidenceInvalid, observation.ObservationID)
		}
		if _, exists := observationByID[observation.ObservationID]; exists {
			return GlobalRadarSnapshot{}, fmt.Errorf("%w: duplicate observation=%s", ErrGlobalCampaignRadarEvidenceInvalid, observation.ObservationID)
		}
		observationByID[observation.ObservationID] = observation
		if err := registerGlobalCampaignRadarEvidence(evidenceByID, observation.Evidence, "observation="+observation.ObservationID); err != nil {
			return GlobalRadarSnapshot{}, err
		}
		observations = append(observations, observation)
	}

	bridgeLinks := make([]GlobalRadarBridgeLink, 0, len(snapshot.BridgeLinks))
	for _, link := range snapshot.BridgeLinks {
		source, sourceOK := observationByID[link.SourceObservation.ObservationID]
		destination, destinationOK := observationByID[link.DestinationObservation.ObservationID]
		if !sourceOK || !destinationOK || !reflect.DeepEqual(source, link.SourceObservation) || !reflect.DeepEqual(destination, link.DestinationObservation) {
			return GlobalRadarSnapshot{}, fmt.Errorf("%w: bridge=%s endpoints", ErrGlobalCampaignRadarEvidenceInvalid, link.LinkID)
		}
		rebuilt, err := BuildGlobalRadarBridgeLink(source, destination)
		if err != nil || !reflect.DeepEqual(rebuilt, link) {
			return GlobalRadarSnapshot{}, fmt.Errorf("%w: bridge=%s", ErrGlobalCampaignRadarEvidenceInvalid, link.LinkID)
		}
		if err := registerGlobalCampaignRadarEvidence(evidenceByID, link.LinkEvidence, "bridge="+link.LinkID); err != nil {
			return GlobalRadarSnapshot{}, err
		}
		bridgeLinks = append(bridgeLinks, link)
	}

	relations := make([]GlobalRadarRelationEdge, 0, len(snapshot.Relations))
	for _, relation := range snapshot.Relations {
		var linkEvidence IntelligenceEvidence
		refs := nonEmptyIntelligenceRefs(relation.EvidenceRefs)
		if len(refs) > 1 {
			return GlobalRadarSnapshot{}, fmt.Errorf("%w: relation=%s has multiple canonical evidence refs", ErrGlobalCampaignRadarEvidenceInvalid, relation.EdgeID)
		}
		if len(refs) == 1 {
			var ok bool
			linkEvidence, ok = evidenceByID[refs[0]]
			if !ok {
				return GlobalRadarSnapshot{}, fmt.Errorf("%w: relation=%s evidence=%s", ErrGlobalCampaignRadarEvidenceInvalid, relation.EdgeID, refs[0])
			}
		}
		rebuilt, err := BuildGlobalRadarRelationEdge(relation.Source, relation.Target, relation.Relation, linkEvidence)
		if err != nil || !reflect.DeepEqual(rebuilt, relation) {
			return GlobalRadarSnapshot{}, fmt.Errorf("%w: relation=%s", ErrGlobalCampaignRadarEvidenceInvalid, relation.EdgeID)
		}
		relations = append(relations, relation)
	}

	canonical, err := BuildGlobalRadarSnapshot(
		observations,
		relations,
		bridgeLinks,
		append([]GlobalRadarVerdictReference(nil), snapshot.VerdictRefs...),
		snapshot.GeneratedAt,
	)
	if err != nil {
		return GlobalRadarSnapshot{}, fmt.Errorf("%w: %v", ErrGlobalCampaignRadarSnapshotInvalid, err)
	}
	return canonical, nil
}

func registerGlobalCampaignRadarEvidence(index map[string]IntelligenceEvidence, evidence IntelligenceEvidence, owner string) error {
	evidenceID := strings.TrimSpace(evidence.ID)
	if evidenceID == "" {
		return fmt.Errorf("%w: %s empty evidence id", ErrGlobalCampaignRadarEvidenceInvalid, owner)
	}
	if previous, exists := index[evidenceID]; exists {
		if !reflect.DeepEqual(previous, evidence) {
			return fmt.Errorf("%w: divergent duplicate evidence=%s owner=%s", ErrGlobalCampaignRadarEvidenceInvalid, evidenceID, owner)
		}
		return nil
	}
	index[evidenceID] = evidence
	return nil
}

func findGlobalCampaignTemporalCorrelation(correlations []GlobalCampaignTemporalCorrelation, leftRef, rightRef string) (GlobalCampaignTemporalCorrelation, bool) {
	for _, correlation := range correlations {
		if (correlation.LeftRef == leftRef && correlation.RightRef == rightRef) ||
			(correlation.LeftRef == rightRef && correlation.RightRef == leftRef) {
			return correlation, true
		}
	}
	return GlobalCampaignTemporalCorrelation{}, false
}

func hashGlobalCampaignRadarProjection(report GlobalCampaignRadarProjection) string {
	report.FingerprintSHA256 = ""
	// Snapshot assembly time is descriptive operator metadata, not evidence
	// identity. Excluding it keeps replay fingerprints stable for the same
	// canonical evidence materialized at different times.
	report.SnapshotGeneratedAt = time.Time{}
	payload, err := json.Marshal(report)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}
