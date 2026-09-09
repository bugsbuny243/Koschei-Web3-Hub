package clickhouse

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"koschei/api/internal/services"
)

func TestProjectARVISMemoryProducesObservedEvidenceOnly(t *testing.T) {
	snapshot := validProjectionSnapshot(t)
	generatedAt := time.Date(2026, 9, 9, 2, 0, 0, 0, time.UTC)
	investigation, err := ProjectARVISMemory(snapshot, generatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(investigation.Subjects) != 1 || investigation.Subjects[0].Raw != snapshot.Target {
		t.Fatalf("subject=%#v", investigation.Subjects)
	}
	if investigation.Subjects[0].ChainFamily != services.IntelligenceChainFamilySolana {
		t.Fatalf("chain family=%q", investigation.Subjects[0].ChainFamily)
	}
	if len(investigation.Evidence) != 2 {
		t.Fatalf("evidence=%#v", investigation.Evidence)
	}
	for _, evidence := range investigation.Evidence {
		if evidence.Status != services.IntelligenceEvidenceObserved {
			t.Fatalf("historical memory must remain observed, got %#v", evidence)
		}
	}
	if investigation.Decision.Status != services.IntelligenceEvidenceUnverified || investigation.Decision.Action != "investigate" {
		t.Fatalf("historical memory must not create a current decision: %#v", investigation.Decision)
	}
	if len(investigation.Relationships) != 0 || len(investigation.Behaviors) != 0 || len(investigation.Hypotheses) != 0 || len(investigation.AttackPaths) != 0 {
		t.Fatalf("projection invented higher-order intelligence: %#v", investigation)
	}

	var verdictEvidence services.IntelligenceEvidence
	for _, evidence := range investigation.Evidence {
		if evidence.ID == "clickhouse:verdict:11111111-1111-1111-1111-111111111111" {
			verdictEvidence = evidence
		}
	}
	if verdictEvidence.ID == "" {
		t.Fatal("missing verdict evidence")
	}
	if verdictEvidence.Attributes["stored_signed"] != true || verdictEvidence.Attributes["signature_verification"] != "not_performed_by_memory_projection" {
		t.Fatalf("signed verdict boundary lost: %#v", verdictEvidence.Attributes)
	}
	if verdictEvidence.Attributes["correlation_status"] != "linked" || verdictEvidence.Attributes["memory_authority"] != "historical_context_only" {
		t.Fatalf("verdict memory semantics=%#v", verdictEvidence.Attributes)
	}
}

func TestValidateARVISMemorySnapshotForProjectionRejectsTamperedVerdictEvidence(t *testing.T) {
	snapshot := validProjectionSnapshot(t)
	snapshot.Verdicts[0].Evidence = []string{"tampered"}
	if err := ValidateARVISMemorySnapshotForProjection(snapshot); err == nil || !strings.Contains(err.Error(), "evidence hash mismatch") {
		t.Fatalf("expected evidence hash mismatch, got %v", err)
	}
}

func TestValidateARVISMemorySnapshotForProjectionRejectsSubjectBoundaryEscape(t *testing.T) {
	snapshot := validProjectionSnapshot(t)
	snapshot.Verdicts[0].Target = strings.ToLower(snapshot.Target)
	snapshot.FingerprintSHA256 = arvisMemoryFingerprint(snapshot.Verdicts, snapshot.Events)
	if err := ValidateARVISMemorySnapshotForProjection(snapshot); err == nil || !strings.Contains(err.Error(), "escaped exact subject boundary") {
		t.Fatalf("expected subject boundary rejection, got %v", err)
	}
}

func TestValidateARVISMemorySnapshotForProjectionRejectsForgedCorrelationMetadata(t *testing.T) {
	snapshot := validProjectionSnapshot(t)
	snapshot.Links[0].Status = "event_not_observed_in_window"
	if err := ValidateARVISMemorySnapshotForProjection(snapshot); err == nil || !strings.Contains(err.Error(), "correlation metadata is inconsistent") {
		t.Fatalf("expected correlation rejection, got %v", err)
	}
}

func TestValidateARVISMemorySnapshotForProjectionRejectsFingerprintTamper(t *testing.T) {
	snapshot := validProjectionSnapshot(t)
	snapshot.FingerprintSHA256 = strings.Repeat("0", 64)
	if err := ValidateARVISMemorySnapshotForProjection(snapshot); err == nil || !strings.Contains(err.Error(), "fingerprint mismatch") {
		t.Fatalf("expected fingerprint mismatch, got %v", err)
	}
}

func validProjectionSnapshot(t *testing.T) ARVISMemorySnapshot {
	t.Helper()
	target := "62tJyrfNfYJ2qZncdbwFYmeJmSFn66BhGfgj491ppump"
	network := "solana-mainnet"
	since := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	until := since.Add(24 * time.Hour)
	signals, err := canonicalJSON(json.RawMessage(`{"creator_wallet":"CaseSensitiveCreator"}`))
	if err != nil {
		t.Fatal(err)
	}
	evidenceRefs := []string{"arvis:tx:abc", "arvis:slot:7"}
	eventID := "22222222-2222-2222-2222-222222222222"
	verdicts := []ARVISVerdictMemory{{
		VerdictID: "11111111-1111-1111-1111-111111111111", EventID: eventID, ModuleID: "final_verdict_engine",
		Target: target, TargetType: "token", Network: network, Grade: "B", RiskIndex: 62, RiskLevel: "high", Verdict: "BLOCK",
		Recommendation: "review historical evidence", Evidence: evidenceRefs, EvidenceSHA256: evidenceListSHA256(evidenceRefs),
		Signals: signals, SignalsSHA256: jsonPayloadSHA256(signals), RuleVersion: "rules-v2", Signed: true, Signature: "stored-signature",
		Source: "arvis_stream", EventType: "arvis_stream_verdict", Provider: "solana_rpc", CreatedAt: since.Add(time.Hour),
		SourceUpdatedAt: since.Add(2 * time.Hour), SourceVersion: 2, SchemaVersion: VerdictSnapshotSchemaVersion,
	}}
	events := []ARVISStreamMemory{{
		EventID: eventID, EventKey: strings.Repeat("a", 64), Provider: "solana_rpc", StreamMode: "journal", Network: network,
		ModuleID: "final_verdict_engine", EventType: "trade", Target: target, TargetType: "token", Signature: "chain-signature",
		Slot: 7, ProgramID: "ProgramCaseSensitive", EvidenceQuality: "raw_stream", DecodedSHA256: strings.Repeat("b", 64),
		RawEventSHA256: strings.Repeat("c", 64), CreatedAt: since.Add(time.Hour), IngestVersion: 1, SchemaVersion: StreamSchemaVersion,
	}}
	return correlateARVISMemory(ARVISMemoryReadRequest{Target: target, Network: network, Since: since, Until: until, Limit: 10}, verdicts, events)
}
