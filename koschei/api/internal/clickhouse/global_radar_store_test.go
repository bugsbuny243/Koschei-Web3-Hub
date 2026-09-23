package clickhouse

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"koschei/api/internal/services"
)

func globalRadarStoreObservation(t *testing.T, network, address, txHash, protocol, transferID string, observedAt time.Time) services.GlobalRadarObservation {
	t.Helper()
	subject := services.ClassifyIntelligenceSubject(address, network)
	evidence := services.IntelligenceEvidence{
		ID:              "evidence:" + network + ":" + txHash,
		SubjectID:       subject.ID,
		ChainFamily:     subject.ChainFamily,
		Chain:           subject.Chain,
		Network:         subject.Network,
		Source:          "test_chain_adapter",
		Status:          services.IntelligenceEvidenceObserved,
		TransactionHash: txHash,
		ObservedAt:      observedAt,
		Method:          "test_transaction",
		Confidence:      0.8,
		Attributes: map[string]any{
			"bridge_protocol":    protocol,
			"bridge_transfer_id": transferID,
		},
	}
	observation, err := services.BuildGlobalRadarObservation(services.GlobalRadarObservationTransaction, subject, evidence)
	if err != nil {
		t.Fatal(err)
	}
	return observation
}

func globalRadarStoreSnapshot(t *testing.T) services.GlobalRadarSnapshot {
	t.Helper()
	baseTime := time.Date(2026, 9, 23, 19, 0, 0, 0, time.UTC)
	source := globalRadarStoreObservation(
		t,
		"ethereum-mainnet",
		"0x1111111111111111111111111111111111111111",
		"0xsource",
		"wormhole",
		"vaa:2/test/9",
		baseTime,
	)
	destination := globalRadarStoreObservation(
		t,
		"base-mainnet",
		"0x2222222222222222222222222222222222222222",
		"0xdestination",
		"wormhole",
		"vaa:2/test/9",
		baseTime.Add(time.Minute),
	)
	link, err := services.BuildGlobalRadarBridgeLink(source, destination)
	if err != nil {
		t.Fatal(err)
	}

	solanaSubject := services.ClassifyIntelligenceSubject("So11111111111111111111111111111111111111112", "solana-mainnet")
	solanaSubject.Kind = services.IntelligenceSubjectToken
	solanaEvidence := services.IntelligenceEvidence{
		ID:          "arvis-evidence-1",
		SubjectID:   solanaSubject.ID,
		ChainFamily: solanaSubject.ChainFamily,
		Chain:       solanaSubject.Chain,
		Network:     solanaSubject.Network,
		Source:      "arvis-canonical-evidence",
		Status:      services.IntelligenceEvidenceVerified,
		ObservedAt:  baseTime.Add(2 * time.Minute),
		Method:      "canonical_arvis_evidence",
		Confidence:  1,
	}
	solanaObservation, err := services.BuildGlobalRadarObservation(services.GlobalRadarObservationState, solanaSubject, solanaEvidence)
	if err != nil {
		t.Fatal(err)
	}
	verdict := services.UnifiedRadarVerdict{
		Target:             solanaSubject.Raw,
		Network:            solanaSubject.Network,
		Grade:              "F",
		Verdict:            "hard_trigger",
		RulesetVersion:     "koschei-unified-radar-rules-v1.4.0",
		ActorRuleset:       "actor-rules-v1",
		Signed:             true,
		Signature:          "test-signature",
		SignatureAlgorithm: services.UnifiedVerdictSignatureAlgorithmV1,
		KeyID:              "verdict-key-v1",
		PayloadHash:        "sha256:0123456789abcdef",
		GeneratedAt:        baseTime.Add(3 * time.Minute),
	}
	decision := services.IntelligenceDecision{
		Status:       services.IntelligenceEvidenceVerified,
		Action:       "review_signed_verdict",
		EvidenceRefs: []string{solanaEvidence.ID},
		Confidence:   1,
	}
	verdictRef, err := services.ProjectARVISSignedVerdictToGlobalRadar(solanaSubject, verdict, decision)
	if err != nil {
		t.Fatal(err)
	}

	snapshot, err := services.BuildGlobalRadarSnapshot(
		[]services.GlobalRadarObservation{source, destination, solanaObservation},
		[]services.GlobalRadarRelationEdge{link.Relation},
		[]services.GlobalRadarBridgeLink{link},
		[]services.GlobalRadarVerdictReference{verdictRef},
		baseTime.Add(4*time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestGlobalRadarRowsFromSnapshotPreserveRecordTypesAndHashes(t *testing.T) {
	snapshot := globalRadarStoreSnapshot(t)
	rows, err := globalRadarRowsFromSnapshot(snapshot, 12345)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 6 {
		t.Fatalf("row count=%d want=6", len(rows))
	}
	types := map[string]int{}
	snapshotHashes := map[string]bool{}
	for _, row := range rows {
		types[row.RecordType]++
		snapshotHashes[row.SnapshotSHA256] = true
		if row.PayloadSHA256 == "" || row.PayloadSHA256 != jsonPayloadSHA256(json.RawMessage(row.PayloadJSON)) {
			t.Fatalf("payload hash mismatch: %#v", row)
		}
		if row.IngestVersion != 12345 {
			t.Fatalf("ingest version=%d", row.IngestVersion)
		}
	}
	if types["observation"] != 3 || types["relation"] != 1 || types["bridge_link"] != 1 || types["verdict_reference"] != 1 {
		t.Fatalf("record types=%v", types)
	}
	if len(snapshotHashes) != 1 {
		t.Fatalf("records escaped one snapshot hash: %v", snapshotHashes)
	}
}

func TestInsertGlobalRadarSnapshotUsesBoundedJSONEachRowContract(t *testing.T) {
	snapshot := globalRadarStoreSnapshot(t)
	var rows []globalRadarGraphRecordRow
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-ClickHouse-Database") != "koschei_web3" ||
			r.Header.Get("X-ClickHouse-User") != "radar-user" ||
			r.Header.Get("X-ClickHouse-Key") != "radar-password" {
			t.Fatal("ClickHouse auth/database headers missing or changed")
		}
		if got := r.URL.Query().Get("query"); got != "INSERT INTO global_radar_graph_records FORMAT JSONEachRow" {
			t.Fatalf("insert query=%q", got)
		}
		if r.URL.Query().Get("async_insert") != "1" || r.URL.Query().Get("wait_for_async_insert") != "1" {
			t.Fatal("Global Radar insert must wait for async acceptance")
		}
		decoder := json.NewDecoder(r.Body)
		for {
			var row globalRadarGraphRecordRow
			err := decoder.Decode(&row)
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			rows = append(rows, row)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(Config{
		HTTPURL:    server.URL,
		Database:   "koschei_web3",
		User:       "radar-user",
		Password:   "radar-password",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.InsertGlobalRadarSnapshot(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 6 {
		t.Fatalf("inserted rows=%d", len(rows))
	}
}

func TestReadGlobalRadarGraphRechecksPayloadHashAndBoundaries(t *testing.T) {
	recordedAt := time.Date(2026, 9, 23, 19, 5, 0, 0, time.UTC)
	payload := json.RawMessage(`{"schema_version":"koschei.global-radar-observation.v1","observation_id":"obs-1"}`)
	payloadHash := jsonPayloadSHA256(payload)
	snapshotHash := strings.Repeat("a", 64)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query, err := url.QueryUnescape(r.URL.Query().Get("query"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(query, "FROM global_radar_graph_records FINAL") ||
			!strings.Contains(query, "(network = {network:String} OR secondary_network = {network:String})") ||
			!strings.Contains(query, "(subject_id = {subject:String} OR target_subject_id = {subject:String})") {
			t.Fatalf("read query lost graph boundaries: %s", query)
		}
		if r.URL.Query().Get("param_network") != "ethereum-mainnet" ||
			r.URL.Query().Get("param_subject") != "subject-1" {
			t.Fatal("read query parameters escaped exact requested boundary")
		}
		row := globalRadarGraphReadRow{
			RecordType:      "observation",
			RecordID:        "obs-1",
			SchemaVersion:   services.GlobalRadarObservationSchemaVersion,
			SnapshotSHA256:  snapshotHash,
			Network:         "ethereum-mainnet",
			SubjectID:       "subject-1",
			RecordKind:      services.GlobalRadarObservationTransaction,
			EvidenceStatus:  services.IntelligenceEvidenceObserved,
			EvidenceRefs:    []string{"evidence-1"},
			ObservedAtNanos: recordedAt.Add(-time.Minute).UnixNano(),
			RecordedAtNanos: recordedAt.UnixNano(),
			PayloadJSON:     string(payload),
			PayloadSHA256:   payloadHash,
			IngestVersion:   99,
		}
		_ = json.NewEncoder(w).Encode(row)
	}))
	defer server.Close()

	client, err := New(Config{
		HTTPURL:    server.URL,
		Database:   "koschei_web3",
		User:       "radar-user",
		Password:   "radar-password",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := client.ReadGlobalRadarGraph(context.Background(), GlobalRadarGraphReadRequest{
		Network:   "ethereum-mainnet",
		SubjectID: "subject-1",
		Since:     recordedAt.Add(-time.Hour),
		Until:     recordedAt.Add(time.Hour),
		Limit:     10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].PayloadSHA256 != payloadHash || rows[0].RecordID != "obs-1" {
		t.Fatalf("unexpected read rows: %#v", rows)
	}
}

func TestValidateGlobalRadarGraphReadRowRejectsTamperingAndBoundaryEscape(t *testing.T) {
	now := time.Date(2026, 9, 23, 19, 5, 0, 0, time.UTC)
	payload := json.RawMessage(`{"ok":true}`)
	request := GlobalRadarGraphReadRequest{
		Network: "ethereum-mainnet",
		Since:   now.Add(-time.Hour),
		Until:   now.Add(time.Hour),
		Limit:   10,
	}
	base := globalRadarGraphReadRow{
		RecordType:      "observation",
		RecordID:        "obs-1",
		SchemaVersion:   services.GlobalRadarObservationSchemaVersion,
		SnapshotSHA256:  strings.Repeat("a", 64),
		Network:         "ethereum-mainnet",
		SubjectID:       "subject-1",
		RecordKind:      services.GlobalRadarObservationTransaction,
		EvidenceStatus:  services.IntelligenceEvidenceObserved,
		RecordedAtNanos: now.UnixNano(),
		PayloadJSON:     string(payload),
		PayloadSHA256:   jsonPayloadSHA256(payload),
	}
	tampered := base
	tampered.PayloadJSON = `{"ok":false}`
	if _, err := validateGlobalRadarGraphReadRow(tampered, request); err == nil {
		t.Fatal("tampered payload was accepted")
	}
	escaped := base
	escaped.Network = "base-mainnet"
	if _, err := validateGlobalRadarGraphReadRow(escaped, request); err == nil {
		t.Fatal("row outside requested network was accepted")
	}
}

func TestNormalizeGlobalRadarGraphReadRequestIsBounded(t *testing.T) {
	now := time.Now().UTC()
	for _, request := range []GlobalRadarGraphReadRequest{
		{Since: now.Add(-time.Hour), Until: now, Limit: 10},
		{Network: "solana-mainnet", Since: now.Add(-32 * 24 * time.Hour), Until: now, Limit: 10},
		{Network: "solana-mainnet", Since: now.Add(-time.Hour), Until: now, Limit: 0},
		{Network: "solana-mainnet", Since: now.Add(-time.Hour), Until: now, Limit: 10, RecordType: "prediction"},
	} {
		if _, err := normalizeGlobalRadarGraphReadRequest(request); err == nil {
			t.Fatalf("invalid request accepted: %#v", request)
		}
	}
}
