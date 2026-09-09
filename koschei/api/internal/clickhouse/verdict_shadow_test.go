package clickhouse

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestVerdictSnapshotCanonicalizesSignalsAndPreservesNativeCase(t *testing.T) {
	base := VerdictSnapshot{
		VerdictID:       "11111111-1111-1111-1111-111111111111",
		ModuleID:        "final_verdict_engine",
		Target:          "AbCdEf123",
		TargetType:      "token",
		Network:         "solana-mainnet",
		Verdict:         "REVIEW",
		Evidence:        []string{"tx:SigCase", "slot:42"},
		Signals:         json.RawMessage(`{"b":2,"a":1}`),
		RuleVersion:     "arvis-v1",
		Signed:          true,
		Signature:       "SigCaseSensitive",
		Source:          "arvis_stream",
		CreatedAt:       time.Date(2026, 9, 9, 0, 0, 0, 123456000, time.UTC),
		SourceUpdatedAt: time.Date(2026, 9, 9, 0, 0, 1, 654321000, time.UTC),
	}
	first, err := normalizeVerdictSnapshot(base)
	if err != nil {
		t.Fatalf("normalize first: %v", err)
	}
	base.Signals = json.RawMessage(`{"a":1,"b":2}`)
	second, err := normalizeVerdictSnapshot(base)
	if err != nil {
		t.Fatalf("normalize second: %v", err)
	}
	if first.Target != "AbCdEf123" || first.Signature != "SigCaseSensitive" {
		t.Fatalf("case-sensitive native identifiers changed target=%q signature=%q", first.Target, first.Signature)
	}
	if first.SignalsSHA256 != second.SignalsSHA256 {
		t.Fatalf("semantic JSON ordering changed signals hash first=%s second=%s", first.SignalsSHA256, second.SignalsSHA256)
	}
	if first.SourceVersion != uint64(base.SourceUpdatedAt.UnixMicro()) {
		t.Fatalf("source_version=%d want=%d", first.SourceVersion, base.SourceUpdatedAt.UnixMicro())
	}
}

func TestInsertVerdictSnapshotsUsesSecureHeadersAndPayloadHashes(t *testing.T) {
	var received map[string]any
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-ClickHouse-Database") != "koschei_web3" || r.Header.Get("X-ClickHouse-User") != "shadow-user" || r.Header.Get("X-ClickHouse-Key") != "shadow-password" {
			t.Fatal("ClickHouse auth/database headers missing or changed")
		}
		query := r.URL.Query()
		if query.Get("query") != "INSERT INTO arvis_verdict_snapshots FORMAT JSONEachRow" {
			t.Fatalf("insert query=%q", query.Get("query"))
		}
		if query.Get("async_insert") != "1" || query.Get("wait_for_async_insert") != "1" {
			t.Fatal("verdict shadow must use durable async insert acknowledgement")
		}
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&received); err != nil {
			t.Fatalf("decode inserted verdict: %v", err)
		}
		var extra map[string]any
		if err := decoder.Decode(&extra); err != io.EOF {
			t.Fatalf("unexpected extra row err=%v row=%v", err, extra)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(Config{HTTPURL: server.URL, Database: "koschei_web3", User: "shadow-user", Password: "shadow-password", HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	item := VerdictSnapshot{
		VerdictID:       "11111111-1111-1111-1111-111111111111",
		EventID:         "22222222-2222-2222-2222-222222222222",
		ModuleID:        "final_verdict_engine",
		Target:          "AbCdEf123",
		TargetType:      "token",
		Network:         "solana-mainnet",
		Grade:           "C",
		RiskIndex:       42,
		RiskLevel:       "medium",
		Verdict:         "REVIEW",
		Recommendation:  "Inspect evidence before action.",
		Evidence:        []string{"tx:SigA", "slot:7"},
		Signals:         json.RawMessage(`{"verified_evidence":true,"provider":"solana_rpc"}`),
		RuleVersion:     "rules-v1",
		Signed:          true,
		Signature:       "SigCaseSensitive",
		Source:          "arvis_stream",
		EventType:       "arvis_stream_verdict",
		Provider:        "solana_rpc",
		CreatedAt:       time.Date(2026, 9, 9, 0, 0, 0, 123456000, time.UTC),
		SourceUpdatedAt: time.Date(2026, 9, 9, 0, 0, 1, 654321000, time.UTC),
	}
	if err := client.InsertVerdictSnapshots(context.Background(), []VerdictSnapshot{item}); err != nil {
		t.Fatalf("insert verdict snapshot: %v", err)
	}
	if received["target"] != item.Target || received["signature"] != item.Signature {
		t.Fatalf("native identity changed row=%v", received)
	}
	row, err := normalizeVerdictSnapshot(item)
	if err != nil {
		t.Fatalf("normalize expected verdict: %v", err)
	}
	if received["evidence_sha256"] != row.EvidenceSHA256 || received["signals_sha256"] != row.SignalsSHA256 {
		t.Fatalf("payload hashes missing or changed row=%v", received)
	}
	if got, ok := received["source_version"].(float64); !ok || uint64(got) != row.SourceVersion {
		t.Fatalf("source_version=%v want=%d", received["source_version"], row.SourceVersion)
	}
}

func TestVerdictParityRejectsTamperedEvidenceHash(t *testing.T) {
	item := VerdictSnapshot{
		VerdictID:       "11111111-1111-1111-1111-111111111111",
		ModuleID:        "final_verdict_engine",
		Target:          "AbCd",
		Network:         "solana-mainnet",
		Verdict:         "REVIEW",
		Evidence:        []string{"evidence-a"},
		Signals:         json.RawMessage(`{"ok":true}`),
		Source:          "arvis_stream",
		CreatedAt:       time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC),
		SourceUpdatedAt: time.Date(2026, 9, 9, 0, 0, 1, 0, time.UTC),
	}
	row, err := normalizeVerdictSnapshot(item)
	if err != nil {
		t.Fatalf("normalize verdict: %v", err)
	}
	parity := parityRowFromVerdictSnapshotRow(row)
	parity.EvidenceSHA256 = strings.Repeat("0", 64)
	accumulator := NewVerdictParityAccumulator()
	if err := accumulator.addRow(parity); err == nil {
		t.Fatal("tampered evidence hash unexpectedly accepted")
	}
}

func TestCountDistinctVerdictSnapshotsUsesBoundedFinalQuery(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if !strings.Contains(query.Get("query"), "FROM arvis_verdict_snapshots FINAL") {
			t.Fatalf("count query must use FINAL: %q", query.Get("query"))
		}
		if query.Get("max_rows_to_read") != "10000000" || query.Get("max_execution_time") != "30" {
			t.Fatal("verdict count query is not bounded")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"count":9}]}`))
	}))
	defer server.Close()
	client, err := New(Config{HTTPURL: server.URL, User: "shadow-user", Password: "shadow-password", HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	count, err := client.CountDistinctVerdictSnapshots(context.Background(), time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("count verdict snapshots: %v", err)
	}
	if count != 9 {
		t.Fatalf("count=%d", count)
	}
}
