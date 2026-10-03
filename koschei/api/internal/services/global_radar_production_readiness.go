package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"sort"
	"strings"
	"time"
)

const GlobalRadarProductionReadinessSchemaVersion = "koschei.global-radar-production-readiness.v1"

type GlobalRadarProductionReadinessStatus string

const (
	GlobalRadarProductionReadinessUnknown  GlobalRadarProductionReadinessStatus = "UNKNOWN"
	GlobalRadarProductionReadinessNotReady GlobalRadarProductionReadinessStatus = "NOT_READY"
	GlobalRadarProductionReadinessReady    GlobalRadarProductionReadinessStatus = "READY"
)

var ErrGlobalRadarProductionReadinessInvalid = errors.New("global radar production readiness evidence is invalid")

type GlobalRadarProductionReadinessPolicy struct {
	PolicyVersion                     string `json:"policy_version"`
	MinWindowSeconds                  int64  `json:"min_window_seconds"`
	MaxEvidenceAgeSeconds             int64  `json:"max_evidence_age_seconds"`
	MaxIngestLagSeconds               int64  `json:"max_ingest_lag_seconds"`
	MaxBacklogDepth                   int64  `json:"max_backlog_depth"`
	MaxMTTDSeconds                    int64  `json:"max_mttd_seconds"`
	MaxMTTRSeconds                    int64  `json:"max_mttr_seconds"`
	MaxCostPerMillionEventsMicrounits int64  `json:"max_cost_per_million_events_microunits"`
}

type GlobalRadarProductionReadinessEvidence struct {
	ExpectedRevision                    string    `json:"expected_revision"`
	DeployedRevision                    string    `json:"deployed_revision"`
	WindowStart                         time.Time `json:"window_start"`
	WindowEnd                           time.Time `json:"window_end"`
	Networks                            []string  `json:"networks"`
	CoverageComplete                    bool      `json:"coverage_complete"`
	BlindGapCount                       int64     `json:"blind_gap_count"`
	OpenIngestGapCount                  int64     `json:"open_ingest_gap_count"`
	UnresolvedProviderDisagreementCount int64     `json:"unresolved_provider_disagreement_count"`
	MaxIngestLagSeconds                 int64     `json:"max_ingest_lag_seconds"`
	MaxBacklogDepth                     int64     `json:"max_backlog_depth"`
	MTTDSeconds                         int64     `json:"mttd_seconds"`
	MTTRSeconds                         int64     `json:"mttr_seconds"`
	EventsProcessed                     int64     `json:"events_processed"`
	TotalCostMicrounits                 int64     `json:"total_cost_microunits"`
	ArchiveRestoreVerified              bool      `json:"archive_restore_verified"`
	ArchiveRestoreEvidenceSHA256        string    `json:"archive_restore_evidence_sha256"`
	ReorgRecoveryVerified               bool      `json:"reorg_recovery_verified"`
	ReorgRecoveryEvidenceSHA256         string    `json:"reorg_recovery_evidence_sha256"`
	ResponseCapabilityClaimed           bool      `json:"response_capability_claimed"`
	VerifiedEffectProofCount            int64     `json:"verified_effect_proof_count"`
	EvidenceRefs                        []string  `json:"evidence_refs"`
	ObservedAt                          time.Time `json:"observed_at"`
}

type GlobalRadarProductionReadiness struct {
	SchemaVersion                  string                                 `json:"schema_version"`
	Policy                         GlobalRadarProductionReadinessPolicy   `json:"policy"`
	Evidence                       GlobalRadarProductionReadinessEvidence `json:"evidence"`
	Status                         GlobalRadarProductionReadinessStatus   `json:"status"`
	Ready                          bool                                   `json:"ready"`
	FailedGates                    []string                               `json:"failed_gates"`
	MissingEvidence                []string                               `json:"missing_evidence"`
	CostPerMillionEventsMicrounits int64                                  `json:"cost_per_million_events_microunits"`
	EvaluatedAt                    time.Time                              `json:"evaluated_at"`
	ReadinessHashSHA256            string                                 `json:"readiness_hash_sha256"`
	VerdictAuthority               bool                                   `json:"verdict_authority"`
	ContainmentAuthority           bool                                   `json:"containment_authority"`
	ExecutionAuthority             bool                                   `json:"execution_authority"`
}

func GlobalRadarProductionReadinessPolicyV1() GlobalRadarProductionReadinessPolicy {
	return GlobalRadarProductionReadinessPolicy{
		PolicyVersion:                     "koschei.global-radar-production-readiness-policy.v1",
		MinWindowSeconds:                  24 * 60 * 60,
		MaxEvidenceAgeSeconds:             15 * 60,
		MaxIngestLagSeconds:               120,
		MaxBacklogDepth:                   10000,
		MaxMTTDSeconds:                    300,
		MaxMTTRSeconds:                    3600,
		MaxCostPerMillionEventsMicrounits: 1000000000,
	}
}

func BuildGlobalRadarProductionReadiness(policy GlobalRadarProductionReadinessPolicy, evidence GlobalRadarProductionReadinessEvidence, evaluatedAt time.Time) (GlobalRadarProductionReadiness, error) {
	if err := validateGlobalRadarProductionReadinessPolicy(policy); err != nil {
		return GlobalRadarProductionReadiness{}, err
	}
	evaluatedAt = evaluatedAt.UTC()
	if evaluatedAt.IsZero() {
		return GlobalRadarProductionReadiness{}, ErrGlobalRadarProductionReadinessInvalid
	}

	evidence.ExpectedRevision = strings.TrimSpace(evidence.ExpectedRevision)
	evidence.DeployedRevision = strings.TrimSpace(evidence.DeployedRevision)
	evidence.WindowStart = evidence.WindowStart.UTC()
	evidence.WindowEnd = evidence.WindowEnd.UTC()
	evidence.Networks = normalizeGlobalCampaignStrings(evidence.Networks)
	evidence.ArchiveRestoreEvidenceSHA256 = normalizeResponseDigest(evidence.ArchiveRestoreEvidenceSHA256)
	evidence.ReorgRecoveryEvidenceSHA256 = normalizeResponseDigest(evidence.ReorgRecoveryEvidenceSHA256)
	evidence.EvidenceRefs = normalizeGlobalCampaignStrings(evidence.EvidenceRefs)
	evidence.ObservedAt = evidence.ObservedAt.UTC()

	out := GlobalRadarProductionReadiness{
		SchemaVersion:   GlobalRadarProductionReadinessSchemaVersion,
		Policy:          policy,
		Evidence:        evidence,
		Status:          GlobalRadarProductionReadinessUnknown,
		FailedGates:     []string{},
		MissingEvidence: []string{},
		EvaluatedAt:     evaluatedAt,
	}

	missing := func(code string) { out.MissingEvidence = append(out.MissingEvidence, code) }
	fail := func(code string) { out.FailedGates = append(out.FailedGates, code) }

	if evidence.ExpectedRevision == "" {
		missing("expected_revision")
	}
	if evidence.DeployedRevision == "" {
		missing("deployed_revision")
	}
	if evidence.WindowStart.IsZero() || evidence.WindowEnd.IsZero() || !evidence.WindowEnd.After(evidence.WindowStart) {
		missing("valid_observation_window")
	}
	if len(evidence.Networks) == 0 {
		missing("network_coverage_set")
	}
	if evidence.ObservedAt.IsZero() {
		missing("observed_at")
	}
	if len(evidence.EvidenceRefs) == 0 {
		missing("evidence_refs")
	}
	if !evidence.ArchiveRestoreVerified || !validResponseDigest(evidence.ArchiveRestoreEvidenceSHA256) {
		missing("archive_restore_proof")
	}
	if !evidence.ReorgRecoveryVerified || !validResponseDigest(evidence.ReorgRecoveryEvidenceSHA256) {
		missing("reorg_recovery_proof")
	}
	if evidence.EventsProcessed <= 0 {
		missing("events_processed")
	}
	if evidence.TotalCostMicrounits < 0 {
		return GlobalRadarProductionReadiness{}, ErrGlobalRadarProductionReadinessInvalid
	}
	if evidence.BlindGapCount < 0 || evidence.OpenIngestGapCount < 0 || evidence.UnresolvedProviderDisagreementCount < 0 || evidence.MaxIngestLagSeconds < 0 || evidence.MaxBacklogDepth < 0 || evidence.MTTDSeconds < 0 || evidence.MTTRSeconds < 0 || evidence.VerifiedEffectProofCount < 0 {
		return GlobalRadarProductionReadiness{}, ErrGlobalRadarProductionReadinessInvalid
	}

	if evidence.ExpectedRevision != "" && evidence.DeployedRevision != "" && evidence.ExpectedRevision != evidence.DeployedRevision {
		fail("deployed_revision_mismatch")
	}
	if !evidence.WindowStart.IsZero() && !evidence.WindowEnd.IsZero() && evidence.WindowEnd.After(evidence.WindowStart) && int64(evidence.WindowEnd.Sub(evidence.WindowStart).Seconds()) < policy.MinWindowSeconds {
		fail("observation_window_too_short")
	}
	if !evidence.CoverageComplete {
		fail("coverage_incomplete")
	}
	if evidence.BlindGapCount > 0 {
		fail("blind_gaps_present")
	}
	if evidence.OpenIngestGapCount > 0 {
		fail("open_ingest_gaps_present")
	}
	if evidence.UnresolvedProviderDisagreementCount > 0 {
		fail("provider_disagreement_unresolved")
	}
	if evidence.MaxIngestLagSeconds > policy.MaxIngestLagSeconds {
		fail("ingest_lag_exceeds_policy")
	}
	if evidence.MaxBacklogDepth > policy.MaxBacklogDepth {
		fail("backlog_exceeds_policy")
	}
	if evidence.MTTDSeconds > policy.MaxMTTDSeconds {
		fail("mttd_exceeds_policy")
	}
	if evidence.MTTRSeconds > policy.MaxMTTRSeconds {
		fail("mttr_exceeds_policy")
	}
	if !evidence.ObservedAt.IsZero() && (evidence.ObservedAt.After(evaluatedAt) || evaluatedAt.Sub(evidence.ObservedAt) > time.Duration(policy.MaxEvidenceAgeSeconds)*time.Second) {
		fail("evidence_stale_or_future")
	}
	if evidence.ResponseCapabilityClaimed && evidence.VerifiedEffectProofCount == 0 {
		fail("response_capability_without_verified_effect_proof")
	}

	if evidence.EventsProcessed > 0 {
		cost, ok := safeCostPerMillionEvents(evidence.TotalCostMicrounits, evidence.EventsProcessed)
		if !ok {
			return GlobalRadarProductionReadiness{}, ErrGlobalRadarProductionReadinessInvalid
		}
		out.CostPerMillionEventsMicrounits = cost
		if cost > policy.MaxCostPerMillionEventsMicrounits {
			fail("cost_per_event_exceeds_policy")
		}
	}

	sort.Strings(out.FailedGates)
	sort.Strings(out.MissingEvidence)
	if len(out.MissingEvidence) > 0 {
		out.Status = GlobalRadarProductionReadinessUnknown
		out.Ready = false
	} else if len(out.FailedGates) > 0 {
		out.Status = GlobalRadarProductionReadinessNotReady
		out.Ready = false
	} else {
		out.Status = GlobalRadarProductionReadinessReady
		out.Ready = true
	}
	out.ReadinessHashSHA256 = hashGlobalRadarProductionReadiness(out)
	if err := ValidateGlobalRadarProductionReadiness(out); err != nil {
		return GlobalRadarProductionReadiness{}, err
	}
	return out, nil
}

func ValidateGlobalRadarProductionReadiness(readiness GlobalRadarProductionReadiness) error {
	if readiness.SchemaVersion != GlobalRadarProductionReadinessSchemaVersion || readiness.EvaluatedAt.IsZero() || !validResponseDigest(readiness.ReadinessHashSHA256) {
		return ErrGlobalRadarProductionReadinessInvalid
	}
	if readiness.VerdictAuthority || readiness.ContainmentAuthority || readiness.ExecutionAuthority {
		return ErrGlobalRadarProductionReadinessInvalid
	}
	if readiness.Ready != (readiness.Status == GlobalRadarProductionReadinessReady) {
		return ErrGlobalRadarProductionReadinessInvalid
	}
	if readiness.Status == GlobalRadarProductionReadinessReady && (len(readiness.FailedGates) != 0 || len(readiness.MissingEvidence) != 0) {
		return ErrGlobalRadarProductionReadinessInvalid
	}
	if readiness.Status == GlobalRadarProductionReadinessUnknown && len(readiness.MissingEvidence) == 0 {
		return ErrGlobalRadarProductionReadinessInvalid
	}
	if readiness.Status == GlobalRadarProductionReadinessNotReady && len(readiness.FailedGates) == 0 {
		return ErrGlobalRadarProductionReadinessInvalid
	}
	if hashGlobalRadarProductionReadiness(readiness) != readiness.ReadinessHashSHA256 {
		return ErrGlobalRadarProductionReadinessInvalid
	}
	return nil
}

func validateGlobalRadarProductionReadinessPolicy(policy GlobalRadarProductionReadinessPolicy) error {
	if strings.TrimSpace(policy.PolicyVersion) == "" || policy.MinWindowSeconds <= 0 || policy.MaxEvidenceAgeSeconds <= 0 || policy.MaxIngestLagSeconds < 0 || policy.MaxBacklogDepth < 0 || policy.MaxMTTDSeconds < 0 || policy.MaxMTTRSeconds < 0 || policy.MaxCostPerMillionEventsMicrounits < 0 {
		return ErrGlobalRadarProductionReadinessInvalid
	}
	return nil
}

func safeCostPerMillionEvents(totalCostMicrounits, events int64) (int64, bool) {
	if totalCostMicrounits < 0 || events <= 0 {
		return 0, false
	}
	value := new(big.Int).SetInt64(totalCostMicrounits)
	value.Mul(value, big.NewInt(1000000))
	value.Div(value, big.NewInt(events))
	if !value.IsInt64() || value.Int64() < 0 || value.Int64() > math.MaxInt64 {
		return 0, false
	}
	return value.Int64(), true
}

func hashGlobalRadarProductionReadiness(readiness GlobalRadarProductionReadiness) string {
	readiness.ReadinessHashSHA256 = ""
	payload, err := json.Marshal(readiness)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}
