package services

import (
	"errors"
	"strings"
	"time"
)

const GlobalRadarVerdictReferenceSchemaVersion = "koschei.global-radar-verdict-reference.v1"

type GlobalRadarVerdictReference struct {
	SchemaVersion          string   `json:"schema_version"`
	ReferenceID            string   `json:"reference_id"`
	AuthoritativeEngine    string   `json:"authoritative_engine"`
	TargetSubjectID        string   `json:"target_subject_id"`
	Network                string   `json:"network"`
	Grade                  string   `json:"grade"`
	Verdict                string   `json:"verdict"`
	RulesetVersion         string   `json:"ruleset_version"`
	ActorRulesetVersion    string   `json:"actor_ruleset_version,omitempty"`
	GeneratedAt            time.Time `json:"generated_at"`
	SourceMarkedSigned     bool     `json:"source_marked_signed"`
	SignatureAlgorithm     string   `json:"signature_algorithm"`
	KeyID                  string   `json:"key_id"`
	PayloadHash            string   `json:"payload_hash"`
	Signature              string   `json:"signature"`
	SignatureVerification  string   `json:"signature_verification"`
	DecisionEvidenceStatus string   `json:"decision_evidence_status"`
	EvidenceRefs           []string `json:"evidence_refs"`
	ProjectionState        string   `json:"projection_state"`
	TrustBoundary          string   `json:"trust_boundary"`
}

// ProjectARVISSignedVerdictToGlobalRadar preserves an existing authoritative ARVIS
// verdict as a graph reference. It never re-grades, re-signs, or independently
// authenticates the signature.
//
// Independent Ed25519 verification requires resolving KeyID through an out-of-band
// trusted public-key registry. Until that registry is supplied to a verifier, the
// Global Radar must not claim that it reverified the signature.
func ProjectARVISSignedVerdictToGlobalRadar(
	subject IntelligenceSubject,
	verdict UnifiedRadarVerdict,
	decision IntelligenceDecision,
) (GlobalRadarVerdictReference, error) {
	if subject.ChainFamily != IntelligenceChainFamilySolana ||
		strings.TrimSpace(subject.ID) == "" ||
		strings.TrimSpace(subject.Network) != "solana-mainnet" {
		return GlobalRadarVerdictReference{}, errors.New("ARVIS verdict reference requires canonical Solana subject")
	}
	if strings.TrimSpace(verdict.Target) == "" ||
		strings.TrimSpace(verdict.Target) != strings.TrimSpace(subject.Raw) ||
		strings.TrimSpace(verdict.Network) != strings.TrimSpace(subject.Network) {
		return GlobalRadarVerdictReference{}, errors.New("ARVIS verdict target identity mismatch")
	}
	if !verdict.Signed ||
		!strings.EqualFold(strings.TrimSpace(verdict.SignatureAlgorithm), UnifiedVerdictSignatureAlgorithmV1) ||
		strings.TrimSpace(verdict.Signature) == "" ||
		strings.TrimSpace(verdict.KeyID) == "" ||
		!strings.HasPrefix(strings.TrimSpace(verdict.PayloadHash), "sha256:") {
		return GlobalRadarVerdictReference{}, errors.New("authenticated ARVIS verdict metadata is required")
	}
	if strings.TrimSpace(verdict.Verdict) == "" ||
		strings.TrimSpace(verdict.RulesetVersion) == "" ||
		verdict.GeneratedAt.IsZero() {
		return GlobalRadarVerdictReference{}, errors.New("complete ARVIS verdict contract is required")
	}

	decisionStatus := strings.ToLower(strings.TrimSpace(decision.Status))
	if decision.Action != "review_signed_verdict" ||
		(decisionStatus != IntelligenceEvidenceObserved && decisionStatus != IntelligenceEvidenceVerified) {
		return GlobalRadarVerdictReference{}, errors.New("evidence-linked ARVIS intelligence decision is required")
	}
	evidenceRefs := nonEmptyIntelligenceRefs(decision.EvidenceRefs)
	if len(evidenceRefs) == 0 {
		return GlobalRadarVerdictReference{}, errors.New("ARVIS verdict reference requires canonical decision evidence")
	}

	identity := strings.Join([]string{
		"global-radar-arvis-verdict-reference",
		subject.ID,
		strings.TrimSpace(verdict.RulesetVersion),
		strings.TrimSpace(verdict.PayloadHash),
		strings.TrimSpace(verdict.KeyID),
	}, ":")

	return GlobalRadarVerdictReference{
		SchemaVersion:          GlobalRadarVerdictReferenceSchemaVersion,
		ReferenceID:            intelligenceStableID(identity),
		AuthoritativeEngine:    "arvis",
		TargetSubjectID:        subject.ID,
		Network:                subject.Network,
		Grade:                  strings.TrimSpace(verdict.Grade),
		Verdict:                strings.TrimSpace(verdict.Verdict),
		RulesetVersion:         strings.TrimSpace(verdict.RulesetVersion),
		ActorRulesetVersion:    strings.TrimSpace(verdict.ActorRuleset),
		GeneratedAt:            verdict.GeneratedAt.UTC(),
		SourceMarkedSigned:     true,
		SignatureAlgorithm:     strings.ToLower(strings.TrimSpace(verdict.SignatureAlgorithm)),
		KeyID:                  strings.TrimSpace(verdict.KeyID),
		PayloadHash:            strings.TrimSpace(verdict.PayloadHash),
		Signature:              strings.TrimSpace(verdict.Signature),
		SignatureVerification:  "not_reverified_by_global_radar",
		DecisionEvidenceStatus: decisionStatus,
		EvidenceRefs:           evidenceRefs,
		ProjectionState:        "authoritative_reference_only",
		TrustBoundary:          "signature_requires_out_of_band_trusted_key_registry_for_independent_verification",
	}, nil
}
