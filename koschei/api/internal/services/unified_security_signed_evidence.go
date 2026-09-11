package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"koschei/api/internal/securityevidence"
)

const (
	unifiedSignedEvidenceProvenance   = "authenticated_security_evidence_event"
	IntelligenceSubjectValidationCase = "validation_case"
)

type UnifiedSignedEvidenceBinding struct {
	Domain           string
	SubjectKind      string
	Network          string
	ExpectedProducer string
	TrustedPublicKey string
	ExpectedSubject  securityevidence.Subject
}

type UnifiedSignedEvidenceProjection struct {
	Subject  IntelligenceSubject
	Evidence []IntelligenceEvidence
}

// AdaptUnifiedSignedSecurityEvidence authenticates a SecurityEvidence event and
// projects only its evidence statements into the unified intelligence model.
// It intentionally does not infer capabilities, actions, attack paths or a new
// customer decision from generic findings; domain-specific adapters must do
// that semantic work separately.
func AdaptUnifiedSignedSecurityEvidence(event securityevidence.Event, binding UnifiedSignedEvidenceBinding) (UnifiedSignedEvidenceProjection, error) {
	domain := normalizeUnifiedSecurityDomain(binding.Domain)
	if domain == UnifiedSecurityDomainUnknown {
		return UnifiedSignedEvidenceProjection{}, errors.New("recognized unified security domain is required")
	}
	kind := normalizeUnifiedSignedEvidenceSubjectKind(binding.SubjectKind)
	if kind == "" {
		return UnifiedSignedEvidenceProjection{}, errors.New("recognized unified security subject kind is required")
	}
	expectedProducer := strings.TrimSpace(binding.ExpectedProducer)
	trustedPublicKey := strings.TrimSpace(binding.TrustedPublicKey)
	if expectedProducer == "" || trustedPublicKey == "" {
		return UnifiedSignedEvidenceProjection{}, errors.New("trusted producer identity and public key are required")
	}
	eventSHA256 := strings.ToLower(strings.TrimSpace(event.EventSHA256))
	if err := event.VerifyEd25519(expectedProducer, trustedPublicKey); err != nil {
		return UnifiedSignedEvidenceProjection{}, fmt.Errorf("authenticate unified security evidence event: %w", err)
	}
	canonical, err := event.Canonical()
	if err != nil {
		return UnifiedSignedEvidenceProjection{}, fmt.Errorf("canonicalize unified security evidence event: %w", err)
	}
	canonical.EventSHA256 = eventSHA256
	if !unifiedSignedEvidenceSubjectMatches(canonical.Subject, binding.ExpectedSubject) {
		return UnifiedSignedEvidenceProjection{}, errors.New("security evidence subject does not match trusted adapter binding")
	}
	if len(canonical.Findings) == 0 {
		return UnifiedSignedEvidenceProjection{}, errors.New("signed security evidence event must contain at least one finding")
	}

	subject := buildUnifiedSignedEvidenceSubject(domain, kind, binding.Network, canonical.Subject)
	evidence := make([]IntelligenceEvidence, 0, len(canonical.Findings))
	for _, finding := range canonical.Findings {
		evidence = append(evidence, buildUnifiedSignedFindingEvidence(domain, subject, canonical, finding))
	}
	return UnifiedSignedEvidenceProjection{Subject: subject, Evidence: evidence}, nil
}

func normalizeUnifiedSignedEvidenceSubjectKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case IntelligenceSubjectAgent:
		return IntelligenceSubjectAgent
	case IntelligenceSubjectIdentity:
		return IntelligenceSubjectIdentity
	case IntelligenceSubjectDevice:
		return IntelligenceSubjectDevice
	case IntelligenceSubjectContract:
		return IntelligenceSubjectContract
	case IntelligenceSubjectTreasury:
		return IntelligenceSubjectTreasury
	case IntelligenceSubjectOracle:
		return IntelligenceSubjectOracle
	case IntelligenceSubjectCredential:
		return IntelligenceSubjectCredential
	case IntelligenceSubjectAsset:
		return IntelligenceSubjectAsset
	case IntelligenceSubjectAddress:
		return IntelligenceSubjectAddress
	case IntelligenceSubjectValidationCase:
		return IntelligenceSubjectValidationCase
	default:
		return ""
	}
}

func unifiedSignedEvidenceSubjectMatches(actual, expected securityevidence.Subject) bool {
	return strings.ToLower(strings.TrimSpace(actual.Chain)) == strings.ToLower(strings.TrimSpace(expected.Chain)) &&
		strings.ToLower(strings.TrimSpace(actual.Type)) == strings.ToLower(strings.TrimSpace(expected.Type)) &&
		strings.TrimSpace(actual.ID) == strings.TrimSpace(expected.ID)
}

func buildUnifiedSignedEvidenceSubject(domain, kind, network string, source securityevidence.Subject) IntelligenceSubject {
	network = strings.ToLower(strings.TrimSpace(network))
	chainFamily := IntelligenceChainFamilyUnknown
	chain := "unknown"
	if domain == UnifiedSecurityDomainBlockchain {
		chain = strings.ToLower(strings.TrimSpace(source.Chain))
		chainFamily = unifiedSignedEvidenceChainFamily(chain)
	}
	canonicalRef := strings.Join([]string{
		"signed-evidence",
		domain,
		kind,
		strings.ToLower(strings.TrimSpace(source.Chain)),
		strings.ToLower(strings.TrimSpace(source.Type)),
		strings.TrimSpace(source.ID),
	}, ":")
	return IntelligenceSubject{
		ID:                  intelligenceStableID(canonicalRef),
		Raw:                 strings.TrimSpace(source.ID),
		CanonicalRef:        canonicalRef,
		ChainFamily:         chainFamily,
		Chain:               chain,
		Network:             network,
		Kind:                kind,
		ClassificationBasis: "authenticated_security_evidence_subject_binding",
	}
}

func unifiedSignedEvidenceChainFamily(chain string) string {
	chain = strings.ToLower(strings.TrimSpace(chain))
	switch {
	case chain == "solana", strings.HasPrefix(chain, "solana:"):
		return IntelligenceChainFamilySolana
	case chain == "evm", strings.HasPrefix(chain, "eip155:"),
		chain == "ethereum", chain == "base", chain == "arbitrum", chain == "optimism",
		chain == "polygon", chain == "bnb-chain", chain == "bsc", chain == "avalanche":
		return IntelligenceChainFamilyEVM
	case chain == "bitcoin", chain == "utxo", strings.HasPrefix(chain, "bip122:"):
		return IntelligenceChainFamilyUTXO
	default:
		return IntelligenceChainFamilyUnknown
	}
}

func buildUnifiedSignedFindingEvidence(domain string, subject IntelligenceSubject, event securityevidence.Event, finding securityevidence.Finding) IntelligenceEvidence {
	status := unifiedSignedFindingStatus(finding.State)
	attributes := map[string]any{
		"event_sha256":            strings.TrimSpace(event.EventSHA256),
		"producer":                strings.TrimSpace(event.Producer),
		"finding_id":              strings.TrimSpace(finding.ID),
		"finding_state":           string(finding.State),
		"window_from_unix_ms":     event.Window.FromUnixMS,
		"window_to_unix_ms":       event.Window.ToUnixMS,
		"source_digests_sha256":   append([]string(nil), event.SourceDigests...),
		"unified_security_domain": domain,
		"subject_kind":            subject.Kind,
		"producer_subject_chain":  event.Subject.Chain,
		"producer_subject_type":   event.Subject.Type,
		"producer_subject_id":     event.Subject.ID,
	}
	if digest := strings.TrimSpace(finding.EvidenceSHA256); digest != "" {
		attributes["finding_evidence_sha256"] = digest
	}
	observedAt := time.Time{}
	if event.Window.ToUnixMS > 0 {
		observedAt = time.UnixMilli(event.Window.ToUnixMS).UTC()
	}
	return IntelligenceEvidence{
		ID:          intelligenceStableID(strings.Join([]string{"signed-evidence", event.EventSHA256, finding.ID}, ":")),
		SubjectID:   subject.ID,
		ChainFamily: subject.ChainFamily,
		Chain:       subject.Chain,
		Network:     subject.Network,
		Source:      strings.TrimSpace(event.Producer),
		Status:      status,
		ObservedAt:  observedAt,
		Address:     subject.Raw,
		Method:      strings.TrimSpace(finding.Kind),
		StateChange: strings.TrimSpace(finding.Summary),
		Provenance:  unifiedSignedEvidenceProvenance,
		Confidence:  unifiedSignedFindingConfidence(status),
		Attributes:  attributes,
	}
}

func unifiedSignedFindingStatus(state securityevidence.EvidenceState) string {
	switch state {
	case securityevidence.StateVerified:
		return IntelligenceEvidenceVerified
	case securityevidence.StateObserved:
		return IntelligenceEvidenceObserved
	default:
		return IntelligenceEvidenceUnverified
	}
}

func unifiedSignedFindingConfidence(status string) float64 {
	switch status {
	case IntelligenceEvidenceVerified:
		return 1
	case IntelligenceEvidenceObserved:
		return 0.8
	default:
		return 0
	}
}
