package services

import (
	"context"
	"koschei/api/internal/radaradapter"
	"koschei/api/internal/radaradapter/builtin"
	"strings"
	"time"
)

var runtimeRadarCollectors, radarRegistrationErrors = builtin.Registry()

func RadarCapabilities() []radaradapter.Capability { return runtimeRadarCollectors.Capabilities() }

// Liveness performs no provider IO and never implies network readiness.
func RadarCollectorHealth() map[string]any {
	return map[string]any{"mode": "observe_only", "fixture_conformant_networks": len(RadarCapabilities()), "registration_failures": len(radarRegistrationErrors), "live_verified": false, "grade_issued": false}
}

type RadarInvestigationResult struct {
	OK           bool                      `json:"ok"`
	Verdict      string                    `json:"verdict"`
	GradeIssued  bool                      `json:"grade_issued"`
	Status       string                    `json:"collector_status"`
	Subject      IntelligenceSubject       `json:"subject"`
	Evidence     []IntelligenceEvidence    `json:"evidence"`
	Limitations  []radaradapter.Limitation `json:"limitations"`
	Reasons      []string                  `json:"unmet_evidence_reasons"`
	Requests     int                       `json:"provider_request_count"`
	LatencyMS    int64                     `json:"provider_latency_ms"`
	EvidenceRows int                       `json:"evidence_row_count"`
}

func ExecuteRadarInvestigation(ctx context.Context, target radaradapter.Target) RadarInvestigationResult {
	return collectRadarInvestigation(ctx, target, NewRadarTransport())
}

// Collection enters the existing intelligence evidence bridge. No collector
// supplies a grade and no missing evidence can be promoted to a safe result.
func collectRadarInvestigation(ctx context.Context, target radaradapter.Target, tr radaradapter.Transport) RadarInvestigationResult {
	started := time.Now()
	collected := runtimeRadarCollectors.Run(ctx, target, tr)
	out := RadarInvestigationResult{Verdict: "withhold", Status: collected.Status, Evidence: []IntelligenceEvidence{}, Limitations: collected.Limitations, Requests: collected.Requests, Reasons: []string{"OBSERVE_ONLY: collection does not issue a security grade."}}
	out.LatencyMS = time.Since(started).Milliseconds()
	for _, l := range out.Limitations {
		out.Reasons = append(out.Reasons, l.Code+": "+l.Reason)
	}
	if len(collected.Evidence) == 0 {
		return out
	}
	var family string
	for _, c := range RadarCapabilities() {
		if c.Network == target.Network {
			family = c.Family
			break
		}
	}
	id := target.Network + ":" + target.Kind + ":" + collected.Subject.NativeID
	subject := IntelligenceSubject{ID: id, CanonicalRef: id, Network: target.Network, ChainFamily: family, Chain: strings.TrimSuffix(target.Network, "-mainnet"), Kind: target.Kind, Raw: target.Raw, ClassificationBasis: "native_collector_target_validation"}
	out.Subject = subject
	for i := range collected.Evidence {
		row := &collected.Evidence[i]
		if row.SubjectID != collected.Subject.NativeID {
			out.Status = "insufficient_evidence"
			out.Reasons = append(out.Reasons, "SUBJECT_MISMATCH")
			return out
		}
		row.SubjectID = id
		row.Attributes["relation"] = row.Relation
	}
	evidence, err := AdaptRadarEvidence(subject, collected.Subject, collected.Evidence)
	if err != nil {
		out.Status = "insufficient_evidence"
		out.Reasons = append(out.Reasons, "EVIDENCE_CONTRACT: collector output could not be bound to the subject.")
		return out
	}
	out.OK = true
	out.Evidence = evidence
	out.EvidenceRows = len(evidence)
	return out
}
