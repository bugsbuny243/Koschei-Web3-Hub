package radarcase

import (
	"errors"
	"fmt"

	"koschei/api/internal/radarpath"
	"koschei/api/internal/securityevidence"
)

const (
	PathEvidenceProducerV1 = "global-radar/attack-path-v1"
	PathFindingKindV1      = "global_attack_path"
)

func BuildPathEvidence(path radarpath.Path) (securityevidence.Event, error) {
	if err := path.Verify(); err != nil {
		return securityevidence.Event{}, fmt.Errorf("attack path verification failed: %w", err)
	}
	if path.Status != radarpath.StatusEvidenceBacked {
		return securityevidence.Event{}, errors.New("candidate path cannot be promoted into ARVIS security evidence")
	}
	if len(path.Segments) == 0 || len(path.Networks) == 0 {
		return securityevidence.Event{}, errors.New("attack path is empty")
	}

	state := securityevidence.StateVerified
	fromUnixMS := path.Segments[0].ObservedAtUnixMS
	toUnixMS := fromUnixMS
	for _, segment := range path.Segments {
		if segment.Basis != "EVIDENCE" {
			return securityevidence.Event{}, errors.New("attack path contains non-evidence segment")
		}
		if segment.EvidenceState != securityevidence.StateVerified {
			state = securityevidence.StateObserved
		}
		if segment.ObservedAtUnixMS < fromUnixMS {
			fromUnixMS = segment.ObservedAtUnixMS
		}
		if segment.ObservedAtUnixMS > toUnixMS {
			toUnixMS = segment.ObservedAtUnixMS
		}
	}

	subjectChain := path.Networks[0]
	if len(path.Networks) > 1 {
		subjectChain = "multi-chain"
	}

	event := securityevidence.Event{
		SchemaVersion: securityevidence.SchemaVersionV1,
		Producer:      PathEvidenceProducerV1,
		Subject: securityevidence.Subject{
			Chain: subjectChain,
			Type:  "attack_path",
			ID:    path.PathSHA256,
		},
		Window: securityevidence.ObservationWindow{
			FromUnixMS: fromUnixMS,
			ToUnixMS:   toUnixMS,
		},
		SourceDigests: []string{path.PathSHA256},
		Findings: []securityevidence.Finding{{
			ID:             "path:" + path.PathSHA256,
			Kind:           PathFindingKindV1,
			State:          state,
			EvidenceSHA256: path.PathSHA256,
			Summary:        fmt.Sprintf("Evidence-backed path with %d segments across %d network(s)", len(path.Segments), len(path.Networks)),
		}},
	}
	return event.Seal()
}
