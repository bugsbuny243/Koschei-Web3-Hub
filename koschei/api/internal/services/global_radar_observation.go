package services

import (
	"errors"
	"strings"
	"time"
)

const GlobalRadarObservationSchemaVersion = "koschei.global-radar-observation.v1"

const (
	GlobalRadarObservationIdentity        = "identity"
	GlobalRadarObservationTransaction     = "transaction"
	GlobalRadarObservationState           = "state"
	GlobalRadarObservationContractProgram = "contract_program"
	GlobalRadarObservationNode            = "node"
	GlobalRadarObservationBridge          = "bridge"
	GlobalRadarObservationLiquidity       = "liquidity"
	GlobalRadarObservationThreat          = "threat"
)

type GlobalRadarObservation struct {
	SchemaVersion   string               `json:"schema_version"`
	ObservationID   string               `json:"observation_id"`
	ObservationKind string               `json:"observation_kind"`
	Subject         IntelligenceSubject  `json:"subject"`
	Evidence        IntelligenceEvidence `json:"evidence"`
	ObservedAt      time.Time            `json:"observed_at"`
	DecisionState   string               `json:"decision_state"`
}

// BuildGlobalRadarObservation binds one already-normalized intelligence evidence
// record to one canonical subject. It does not infer relationships, promote
// evidence, or create a verdict.
//
// This is the shared observation envelope for the Global Radar. Chain-native
// adapters remain responsible for proving network identity and normalizing their
// own security semantics before this function is called.
func BuildGlobalRadarObservation(kind string, subject IntelligenceSubject, evidence IntelligenceEvidence) (GlobalRadarObservation, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if !globalRadarObservationKindAllowed(kind) {
		return GlobalRadarObservation{}, errors.New("unsupported global radar observation kind")
	}
	if strings.TrimSpace(subject.ID) == "" || strings.TrimSpace(subject.CanonicalRef) == "" ||
		strings.TrimSpace(subject.ChainFamily) == "" || strings.TrimSpace(subject.Chain) == "" ||
		strings.TrimSpace(subject.Network) == "" || subject.ChainFamily == IntelligenceChainFamilyUnknown {
		return GlobalRadarObservation{}, errors.New("canonical global radar subject is required")
	}
	if strings.TrimSpace(evidence.ID) == "" || strings.TrimSpace(evidence.SubjectID) == "" ||
		strings.TrimSpace(evidence.Source) == "" || evidence.ObservedAt.IsZero() {
		return GlobalRadarObservation{}, errors.New("concrete global radar evidence is required")
	}
	if evidence.Status != IntelligenceEvidenceObserved && evidence.Status != IntelligenceEvidenceVerified {
		return GlobalRadarObservation{}, errors.New("global radar observation requires observed or verified evidence")
	}
	if evidence.SubjectID != subject.ID ||
		!strings.EqualFold(strings.TrimSpace(evidence.ChainFamily), strings.TrimSpace(subject.ChainFamily)) ||
		!strings.EqualFold(strings.TrimSpace(evidence.Chain), strings.TrimSpace(subject.Chain)) ||
		!strings.EqualFold(strings.TrimSpace(evidence.Network), strings.TrimSpace(subject.Network)) {
		return GlobalRadarObservation{}, errors.New("global radar subject and evidence identity mismatch")
	}

	observedAt := evidence.ObservedAt.UTC()
	identity := strings.Join([]string{
		"global-radar-observation",
		kind,
		subject.ID,
		evidence.ID,
		observedAt.Format(time.RFC3339Nano),
	}, ":")

	return GlobalRadarObservation{
		SchemaVersion:   GlobalRadarObservationSchemaVersion,
		ObservationID:   intelligenceStableID(identity),
		ObservationKind: kind,
		Subject:         subject,
		Evidence:        evidence,
		ObservedAt:      observedAt,
		DecisionState:   "evidence_only_no_verdict_created",
	}, nil
}

func globalRadarObservationKindAllowed(kind string) bool {
	switch kind {
	case GlobalRadarObservationIdentity,
		GlobalRadarObservationTransaction,
		GlobalRadarObservationState,
		GlobalRadarObservationContractProgram,
		GlobalRadarObservationNode,
		GlobalRadarObservationBridge,
		GlobalRadarObservationLiquidity,
		GlobalRadarObservationThreat:
		return true
	default:
		return false
	}
}
