package clickhouse

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"koschei/api/internal/services"
)

const maxProjectionSlot = uint64(1<<63 - 1)

// ProjectARVISMemory converts a previously validated ClickHouse ARVIS memory
// snapshot into the existing chain-neutral IntelligenceInvestigation contract.
// Historical memory is projected as observed evidence only: this function does
// not create a current risk decision, relationship, behavior, hypothesis, or
// attack path from historical rows alone.
func ProjectARVISMemory(snapshot ARVISMemorySnapshot, generatedAt time.Time) (services.IntelligenceInvestigation, error) {
	if err := ValidateARVISMemorySnapshotForProjection(snapshot); err != nil {
		return services.IntelligenceInvestigation{}, err
	}

	subject := services.ClassifyIntelligenceSubject(snapshot.Target, snapshot.Network)
	investigation := services.BuildIntelligenceInvestigation([]services.IntelligenceSubject{subject}, generatedAt)

	linksByVerdict := make(map[string]ARVISEvidenceLink, len(snapshot.Links))
	for _, link := range snapshot.Links {
		linksByVerdict[link.VerdictID] = link
	}

	evidence := make([]services.IntelligenceEvidence, 0, len(snapshot.Events)+len(snapshot.Verdicts))
	for _, event := range snapshot.Events {
		evidence = append(evidence, projectARVISStreamMemory(subject, event))
	}
	for _, verdict := range snapshot.Verdicts {
		evidence = append(evidence, projectARVISVerdictMemory(subject, verdict, linksByVerdict[verdict.VerdictID]))
	}

	sort.SliceStable(evidence, func(i, j int) bool {
		if evidence[i].ObservedAt.Equal(evidence[j].ObservedAt) {
			return evidence[i].ID < evidence[j].ID
		}
		return evidence[i].ObservedAt.Before(evidence[j].ObservedAt)
	})
	investigation.Evidence = evidence
	return investigation, nil
}

// ValidateARVISMemorySnapshotForProjection re-checks the safety boundary even
// when callers construct an ARVISMemorySnapshot directly instead of obtaining
// it from Client.ReadARVISMemory. It deliberately verifies only what the memory
// layer can prove: verdict content hashes, stream digest metadata, exact subject
// identity, bounded timestamps, and explicit event-id correlation.
func ValidateARVISMemorySnapshotForProjection(snapshot ARVISMemorySnapshot) error {
	snapshot.Target = strings.TrimSpace(snapshot.Target)
	snapshot.Network = strings.TrimSpace(snapshot.Network)
	if snapshot.Target == "" || snapshot.Network == "" {
		return fmt.Errorf("ARVIS memory projection requires exact target and network")
	}
	if snapshot.Since.IsZero() || snapshot.Until.IsZero() || !snapshot.Since.Before(snapshot.Until) {
		return fmt.Errorf("ARVIS memory projection window is invalid")
	}
	if snapshot.Until.Sub(snapshot.Since) > MaxARVISMemoryReadWindow {
		return fmt.Errorf("ARVIS memory projection window exceeds %s", MaxARVISMemoryReadWindow)
	}
	if snapshot.VerdictContentIntegrity != "sha256_verified" {
		return fmt.Errorf("ARVIS memory projection requires sha256-verified verdict content")
	}
	if snapshot.StreamPayloadPolicy != "digests_only_not_dereferenced" {
		return fmt.Errorf("ARVIS memory projection has unsupported stream payload policy %q", snapshot.StreamPayloadPolicy)
	}
	if !hex64RE.MatchString(strings.TrimSpace(snapshot.FingerprintSHA256)) {
		return fmt.Errorf("ARVIS memory projection fingerprint is invalid")
	}
	if got := arvisMemoryFingerprint(snapshot.Verdicts, snapshot.Events); got != snapshot.FingerprintSHA256 {
		return fmt.Errorf("ARVIS memory projection fingerprint mismatch")
	}

	seenEvidenceIDs := make(map[string]struct{}, len(snapshot.Verdicts)+len(snapshot.Events))
	for _, verdict := range snapshot.Verdicts {
		if err := validateProjectionVerdict(snapshot, verdict); err != nil {
			return err
		}
		id := "clickhouse:verdict:" + verdict.VerdictID
		if _, exists := seenEvidenceIDs[id]; exists {
			return fmt.Errorf("ARVIS memory projection contains duplicate evidence id %s", id)
		}
		seenEvidenceIDs[id] = struct{}{}
	}
	for _, event := range snapshot.Events {
		if err := validateProjectionEvent(snapshot, event); err != nil {
			return err
		}
		id := "clickhouse:event:" + event.EventID
		if _, exists := seenEvidenceIDs[id]; exists {
			return fmt.Errorf("ARVIS memory projection contains duplicate evidence id %s", id)
		}
		seenEvidenceIDs[id] = struct{}{}
	}

	expected := correlateARVISMemory(ARVISMemoryReadRequest{
		Target: snapshot.Target, Network: snapshot.Network, Since: snapshot.Since, Until: snapshot.Until, Limit: MaxARVISMemoryReadRows,
	}, snapshot.Verdicts, snapshot.Events)
	if snapshot.CorrelationStatus != expected.CorrelationStatus ||
		snapshot.LinkedVerdicts != expected.LinkedVerdicts ||
		snapshot.StandaloneVerdicts != expected.StandaloneVerdicts ||
		snapshot.MissingLinkedEvents != expected.MissingLinkedEvents ||
		snapshot.UnreferencedEvents != expected.UnreferencedEvents ||
		!reflect.DeepEqual(snapshot.Links, expected.Links) {
		return fmt.Errorf("ARVIS memory projection correlation metadata is inconsistent")
	}
	return nil
}

func validateProjectionVerdict(snapshot ARVISMemorySnapshot, verdict ARVISVerdictMemory) error {
	rawVerdictID := strings.TrimSpace(verdict.VerdictID)
	rawEventID := strings.TrimSpace(verdict.EventID)
	verdict.VerdictID = strings.ToLower(rawVerdictID)
	verdict.EventID = strings.ToLower(rawEventID)
	if rawVerdictID != verdict.VerdictID || rawEventID != verdict.EventID {
		return fmt.Errorf("ARVIS memory projection verdict identity is not canonical lowercase UUID text")
	}
	if !uuidRE.MatchString(verdict.VerdictID) {
		return fmt.Errorf("ARVIS memory projection contains invalid verdict UUID")
	}
	if verdict.EventID != "" && !uuidRE.MatchString(verdict.EventID) {
		return fmt.Errorf("ARVIS memory projection verdict %s contains invalid event UUID", verdict.VerdictID)
	}
	if verdict.Target != snapshot.Target || verdict.Network != snapshot.Network {
		return fmt.Errorf("ARVIS memory projection verdict %s escaped exact subject boundary", verdict.VerdictID)
	}
	if verdict.SchemaVersion != VerdictSnapshotSchemaVersion {
		return fmt.Errorf("ARVIS memory projection verdict %s has unsupported schema version %d", verdict.VerdictID, verdict.SchemaVersion)
	}
	if !hex64RE.MatchString(strings.TrimSpace(verdict.EvidenceSHA256)) || !hex64RE.MatchString(strings.TrimSpace(verdict.SignalsSHA256)) {
		return fmt.Errorf("ARVIS memory projection verdict %s has invalid content digest", verdict.VerdictID)
	}
	if got := evidenceListSHA256(verdict.Evidence); got != verdict.EvidenceSHA256 {
		return fmt.Errorf("ARVIS memory projection verdict %s evidence hash mismatch", verdict.VerdictID)
	}
	canonicalSignals, err := canonicalJSON(verdict.Signals)
	if err != nil {
		return fmt.Errorf("ARVIS memory projection verdict %s signals are invalid: %w", verdict.VerdictID, err)
	}
	if got := jsonPayloadSHA256(canonicalSignals); got != verdict.SignalsSHA256 {
		return fmt.Errorf("ARVIS memory projection verdict %s signals hash mismatch", verdict.VerdictID)
	}
	if verdict.SourceUpdatedAt.IsZero() || verdict.SourceUpdatedAt.Before(snapshot.Since) || !verdict.SourceUpdatedAt.Before(snapshot.Until) {
		return fmt.Errorf("ARVIS memory projection verdict %s is outside the requested memory window", verdict.VerdictID)
	}
	return nil
}

func validateProjectionEvent(snapshot ARVISMemorySnapshot, event ARVISStreamMemory) error {
	rawEventID := strings.TrimSpace(event.EventID)
	event.EventID = strings.ToLower(rawEventID)
	if rawEventID != event.EventID {
		return fmt.Errorf("ARVIS memory projection stream event identity is not canonical lowercase UUID text")
	}
	if !uuidRE.MatchString(event.EventID) {
		return fmt.Errorf("ARVIS memory projection contains invalid stream event UUID")
	}
	if event.Target != snapshot.Target || event.Network != snapshot.Network {
		return fmt.Errorf("ARVIS memory projection event %s escaped exact subject boundary", event.EventID)
	}
	if event.SchemaVersion != StreamSchemaVersion {
		return fmt.Errorf("ARVIS memory projection event %s has unsupported schema version %d", event.EventID, event.SchemaVersion)
	}
	if !hex64RE.MatchString(strings.TrimSpace(event.EventKey)) || !hex64RE.MatchString(strings.TrimSpace(event.DecodedSHA256)) || !hex64RE.MatchString(strings.TrimSpace(event.RawEventSHA256)) {
		return fmt.Errorf("ARVIS memory projection event %s has invalid digest metadata", event.EventID)
	}
	if event.Slot > maxProjectionSlot {
		return fmt.Errorf("ARVIS memory projection event %s slot exceeds intelligence contract range", event.EventID)
	}
	if event.CreatedAt.IsZero() || event.CreatedAt.Before(snapshot.Since) || !event.CreatedAt.Before(snapshot.Until) {
		return fmt.Errorf("ARVIS memory projection event %s is outside the requested memory window", event.EventID)
	}
	return nil
}

func projectARVISStreamMemory(subject services.IntelligenceSubject, event ARVISStreamMemory) services.IntelligenceEvidence {
	return services.IntelligenceEvidence{
		ID:              "clickhouse:event:" + event.EventID,
		SubjectID:       subject.ID,
		ChainFamily:     subject.ChainFamily,
		Chain:           subject.Chain,
		Network:         subject.Network,
		Source:          "clickhouse_arvis_stream_memory",
		Status:          services.IntelligenceEvidenceObserved,
		TransactionHash: strings.TrimSpace(event.Signature),
		BlockOrSlot:     int64(event.Slot),
		ObservedAt:      event.CreatedAt.UTC(),
		Address:         event.Target,
		Contract:        strings.TrimSpace(event.ProgramID),
		Method:          strings.TrimSpace(event.EventType),
		Provenance:      "existing_arvis_clickhouse_stream_shadow",
		Confidence:      1,
		Attributes: map[string]any{
			"event_id":              event.EventID,
			"event_key":             event.EventKey,
			"module_id":             event.ModuleID,
			"target_type":           event.TargetType,
			"provider":              event.Provider,
			"stream_mode":           event.StreamMode,
			"evidence_quality":      event.EvidenceQuality,
			"decoded_sha256":        event.DecodedSHA256,
			"raw_event_sha256":      event.RawEventSHA256,
			"stream_payload_policy": "digests_only_not_dereferenced",
			"ingest_version":        event.IngestVersion,
			"schema_version":        event.SchemaVersion,
		},
	}
}

func projectARVISVerdictMemory(subject services.IntelligenceSubject, verdict ARVISVerdictMemory, link ARVISEvidenceLink) services.IntelligenceEvidence {
	observedAt := verdict.SourceUpdatedAt.UTC()
	if observedAt.IsZero() {
		observedAt = verdict.CreatedAt.UTC()
	}
	return services.IntelligenceEvidence{
		ID:          "clickhouse:verdict:" + verdict.VerdictID,
		SubjectID:   subject.ID,
		ChainFamily: subject.ChainFamily,
		Chain:       subject.Chain,
		Network:     subject.Network,
		Source:      "clickhouse_arvis_verdict_memory",
		Status:      services.IntelligenceEvidenceObserved,
		ObservedAt:  observedAt,
		Address:     verdict.Target,
		Method:      "arvis_verdict",
		Provenance:  "existing_arvis_clickhouse_verdict_shadow",
		Confidence:  1,
		Attributes: map[string]any{
			"verdict_id":             verdict.VerdictID,
			"event_id":               verdict.EventID,
			"module_id":              verdict.ModuleID,
			"target_type":            verdict.TargetType,
			"grade":                  verdict.Grade,
			"risk_index":             verdict.RiskIndex,
			"risk_level":             verdict.RiskLevel,
			"historical_verdict":     verdict.Verdict,
			"recommendation":         verdict.Recommendation,
			"upstream_evidence_refs": append([]string(nil), verdict.Evidence...),
			"evidence_sha256":        verdict.EvidenceSHA256,
			"signals_sha256":         verdict.SignalsSHA256,
			"rule_version":           verdict.RuleVersion,
			"stored_signed":          verdict.Signed,
			"signature_present":      strings.TrimSpace(verdict.Signature) != "",
			"signature_verification": "not_performed_by_memory_projection",
			"source":                 verdict.Source,
			"event_type":             verdict.EventType,
			"provider":               verdict.Provider,
			"source_version":         verdict.SourceVersion,
			"schema_version":         verdict.SchemaVersion,
			"content_integrity":      "sha256_verified",
			"correlation_status":     link.Status,
			"memory_authority":       "historical_context_only",
		},
	}
}
