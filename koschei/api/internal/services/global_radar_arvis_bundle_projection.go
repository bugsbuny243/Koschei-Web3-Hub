package services

import (
	"errors"
	"sort"
	"strings"
	"time"
)

var (
	ErrGlobalRadarARVISBundleIdentity = errors.New("global radar ARVIS bundle requires a canonical target identity")
	ErrGlobalRadarARVISBundleEmpty    = errors.New("global radar ARVIS bundle has no verified evidence to project")
)

// ProjectSecurityRadarBundleToGlobalRadar turns the verified evidence arms of an
// ARVIS Security Radar bundle into canonical Global Radar observations. The
// projection is evidence-only: it never re-grades the target, creates a new
// verdict authority, infers actor identity, or grants containment authority.
func ProjectSecurityRadarBundleToGlobalRadar(bundle SecurityRadarBundle, generatedAt time.Time) (GlobalRadarSnapshot, error) {
	target := strings.TrimSpace(bundle.Target)
	network := normalizeRadarNetwork(bundle.Network)
	subject := ClassifyIntelligenceSubject(target, network)
	if target == "" || subject.ChainFamily == IntelligenceChainFamilyUnknown || strings.TrimSpace(subject.ID) == "" {
		return GlobalRadarSnapshot{}, ErrGlobalRadarARVISBundleIdentity
	}
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}

	verdicts := ArvisArmsFromBundle(bundle)
	if len(verdicts) == 0 {
		verdicts = []SecurityRadarVerdict{bundle.PumpSybilRadar, bundle.RaydiumPoolGuardian, bundle.WalletlessClaimShield}
	}
	observations := make([]GlobalRadarObservation, 0, len(verdicts))
	seenEvidence := make(map[string]struct{}, len(verdicts))
	for _, verdict := range verdicts {
		if !SecurityRadarVerdictHasVerifiedEvidence(verdict) {
			continue
		}
		if strings.TrimSpace(verdict.Target) != target || normalizeRadarNetwork(verdict.Network) != network {
			continue
		}
		observedAt, ok := parseSecurityRadarProjectionTime(verdict.GeneratedAt)
		if !ok {
			// The bundle creation time is descriptive runtime metadata, not evidence
			// identity. A verified arm without its own observation time is withheld.
			continue
		}
		evidenceID := securityRadarGlobalEvidenceID(verdict)
		if evidenceID == "" {
			continue
		}
		if _, duplicate := seenEvidence[evidenceID]; duplicate {
			continue
		}
		seenEvidence[evidenceID] = struct{}{}

		attributes := map[string]any{
			"module":               strings.TrimSpace(verdict.Module),
			"module_id":            strings.TrimSpace(verdict.ModuleID),
			"grade":                strings.TrimSpace(verdict.Grade),
			"risk_index":           verdict.RiskIndex,
			"risk_level":           strings.TrimSpace(verdict.RiskLevel),
			"verdict":              strings.TrimSpace(verdict.Verdict),
			"recommendation":       strings.TrimSpace(verdict.Recommendation),
			"rule_version":         strings.TrimSpace(verdict.RuleVersion),
			"source_evidence_refs": nonEmptyIntelligenceRefs(verdict.Evidence),
			"source_marked_signed": verdict.Signed,
			"signature_algorithm":  strings.TrimSpace(verdict.SignatureAlgorithm),
			"key_id":               strings.TrimSpace(verdict.KeyID),
			"payload_hash":         strings.TrimSpace(verdict.PayloadHash),
			"digest":               strings.TrimSpace(verdict.Digest),
			"authority_boundary":   "arvis_evidence_projection_only",
		}
		evidence := IntelligenceEvidence{
			ID:          evidenceID,
			SubjectID:   subject.ID,
			ChainFamily: subject.ChainFamily,
			Chain:       subject.Chain,
			Network:     subject.Network,
			Source:      "arvis_security_radar:" + strings.TrimSpace(verdict.ModuleID),
			Status:      IntelligenceEvidenceVerified,
			ObservedAt:  observedAt,
			Address:     target,
			Confidence:  1,
			Attributes:  attributes,
		}
		observation, err := BuildGlobalRadarObservation(GlobalRadarObservationThreat, subject, evidence)
		if err != nil {
			continue
		}
		observations = append(observations, observation)
	}
	if len(observations) == 0 {
		return GlobalRadarSnapshot{}, ErrGlobalRadarARVISBundleEmpty
	}
	sort.Slice(observations, func(i, j int) bool { return observations[i].ObservationID < observations[j].ObservationID })
	return BuildGlobalRadarSnapshot(observations, nil, nil, nil, generatedAt.UTC())
}

func securityRadarGlobalEvidenceID(verdict SecurityRadarVerdict) string {
	identityParts := []string{
		"arvis-security-radar-evidence",
		strings.TrimSpace(verdict.ModuleID),
		strings.TrimSpace(verdict.Target),
		normalizeRadarNetwork(verdict.Network),
		strings.TrimSpace(verdict.PayloadHash),
		strings.TrimSpace(verdict.Digest),
		strings.TrimSpace(verdict.Signature),
	}
	refs := nonEmptyIntelligenceRefs(verdict.Evidence)
	sort.Strings(refs)
	identityParts = append(identityParts, refs...)
	return intelligenceStableID(strings.Join(identityParts, "\x1f"))
}

func parseSecurityRadarProjectionTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}
