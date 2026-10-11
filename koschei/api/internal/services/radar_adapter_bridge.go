package services

import (
	"errors"
	"strconv"
	"strings"

	"koschei/api/internal/radaradapter"
)

// AdaptRadarEvidence projects validated collector output into the existing
// intelligence evidence contract. Collector output cannot award verification
// or a verdict: a separate independent confirmation path is required.
func AdaptRadarEvidence(subject IntelligenceSubject, identity radaradapter.SubjectIdentity, rows []radaradapter.EvidenceRow) ([]IntelligenceEvidence, error) {
	if strings.TrimSpace(subject.ID) == "" ||
		!strings.EqualFold(strings.TrimSpace(subject.Network), strings.TrimSpace(identity.Network)) ||
		strings.TrimSpace(subject.CanonicalRef) == "" {
		return nil, errors.New("radar subject and native chain identity mismatch")
	}
	if err := radaradapter.ValidateEvidence(identity, rows); err != nil {
		return nil, err
	}
	out := make([]IntelligenceEvidence, 0, len(rows))
	seenEvidenceIDs := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if _, duplicate := seenEvidenceIDs[row.ID]; duplicate {
			return nil, errors.New("duplicate radar evidence ID")
		}
		seenEvidenceIDs[row.ID] = struct{}{}
		if row.SubjectID != subject.ID {
			return nil, errors.New("radar evidence subject ID mismatch")
		}
		height, err := strconv.ParseInt(row.Height, 10, 64)
		if err != nil || height < 0 {
			return nil, errors.New("radar evidence height must be a nonnegative integer")
		}
		attributes := make(map[string]any, len(row.Attributes)+3)
		for key, value := range row.Attributes {
			attributes[key] = value
		}
		// Reserved provenance keys cannot be overwritten by adapter input.
		attributes["raw_response_sha256"] = row.RawHash
		attributes["native_chain_identity_kind"] = identity.IdentityKind
		attributes["native_chain_identity"] = identity.IdentityValue
		out = append(out, IntelligenceEvidence{
			ID:          row.ID,
			SubjectID:   subject.ID,
			ChainFamily: subject.ChainFamily,
			Chain:       subject.Chain,
			Network:     subject.Network,
			Source:      row.Source,
			Status:      IntelligenceEvidenceObserved,
			BlockOrSlot: height,
			ObservedAt:  row.ObservedAt,
			Provenance:  "radaradapter_read_only",
			Attributes:  attributes,
		})
	}
	return out, nil
}
