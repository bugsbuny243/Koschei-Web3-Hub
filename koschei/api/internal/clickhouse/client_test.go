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

func TestStreamEventKeyPreservesCaseSensitiveIdentifiers(t *testing.T) {
	upper := StreamEventKey(StreamEvent{
		EventID:   "11111111-1111-1111-1111-111111111111",
		Signature: "sig",
		ProgramID: "Program",
		ModuleID:  "pump_sybil_radar",
		EventType: "pump_launch_or_trade",
		Target:    "AbCdEf123",
	})
	lower := StreamEventKey(StreamEvent{
		EventID:   "11111111-1111-1111-1111-111111111111",
		Signature: "sig",
		ProgramID: "Program",
		ModuleID:  "pump_sybil_radar",
		EventType: "pump_launch_or_trade",
		Target:    "abcdef123",
	})
	if upper == lower {
		t.Fatal("stream event key must preserve case-sensitive chain identifiers")
	}
}

func TestStreamEventKeyUsesCommittedIDForUnsignedRows(t *testing.T) {
	first := StreamEventKey(StreamEvent{EventID: "11111111-1111-1111-1111-111111111111"})
	second := StreamEventKey(StreamEvent{EventID: "22222222-2222-2222-2222-222222222222"})
	if first == second {
		t.Fatal("unsigned events with distinct committed ids must not collapse")
	}
}

func TestInsertStreamEventsUsesSecureHeadersAsyncBatchAndPayloadHashes(t *testing.T) {
	var rows []map[string]any
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		if got := r.Header.Get("X-ClickHouse-Database"); got != "koschei_web3" {
			t.Fatalf("database header=%q", got)
		}
		if got := r.Header.Get("X-ClickHouse-User"); got != "shadow-user" {
			t.Fatalf("user header=%q", got)
		}
		if got := r.Header.Get("X-ClickHouse-Key"); got != "shadow-password" {
			t.Fatalf("key header missing or changed")
		}
		query := r.URL.Query()
		if got := query.Get("query"); got != "INSERT INTO security_radar_stream_events FORMAT JSONEachRow" {
			t.Fatalf("query=%q", got)
		}
		if query.Get("async_insert") != "1" || query.Get("wait_for_async_insert") != "1" {
			t.Fatal("shadow writes must use durable async inserts")
		}
		decoder := json.NewDecoder(r.Body)
		for {
			var row map[string]any
			err := decoder.Decode(&row)
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("decode row: %v", err)
			}
			rows = append(rows, row)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(Config{
		HTTPURL:    server.URL,
		Database:   "koschei_web3",
		User:       "shadow-user",
		Password:   "shadow-password",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	event := StreamEvent{
		EventID:         "11111111-1111-1111-1111-111111111111",
		Provider:        "solana_wss",
		StreamMode:      "logs_subscribe",
		Network:         "solana-mainnet",
		ModuleID:        "pump_sybil_radar",
		EventType:       "pump_launch_or_trade",
		Target:          "AbCdEf123",
		TargetType:      "token",
		Signature:       "SigCaseSensitive",
		Slot:            42,
		ProgramID:       "ProgramCaseSensitive",
		EvidenceQuality: "raw_stream",
		Decoded:         json.RawMessage(`{"ok":true}`),
		RawEvent:        json.RawMessage(`{"source":"test"}`),
		CreatedAt:       time.Date(2026, 9, 7, 12, 0, 0, 123456789, time.UTC),
	}
	if !json.Valid(event.Decoded) || !json.Valid(event.RawEvent) {
		t.Fatal("test fixture JSON must be valid")
	}
	if err := client.InsertStreamEvents(context.Background(), []StreamEvent{event}); err != nil {
		t.Fatalf("insert stream events: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%d", len(rows))
	}
	if got := rows[0]["target"]; got != event.Target {
		t.Fatalf("target=%v", got)
	}
	if got := rows[0]["signature"]; got != event.Signature {
		t.Fatalf("signature=%v", got)
	}
	if got := rows[0]["event_key"]; got != StreamEventKey(event) {
		t.Fatalf("event_key=%v", got)
	}
	if got := rows[0]["decoded_sha256"]; got != jsonPayloadSHA256(event.Decoded) {
		t.Fatalf("decoded_sha256=%v", got)
	}
	if got := rows[0]["raw_event_sha256"]; got != jsonPayloadSHA256(event.RawEvent) {
		t.Fatalf("raw_event_sha256=%v", got)
	}
	if got, ok := rows[0]["created_at"].(string); !ok || !strings.Contains(got, ".123Z") {
		t.Fatalf("created_at=%v want millisecond-normalized UTC timestamp", rows[0]["created_at"])
	}
}

func TestCountDistinctStreamEventsUsesBoundedQuery(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if !strings.Contains(query.Get("query"), "countDistinct(event_id)") {
			t.Fatalf("query=%q", query.Get("query"))
		}
		if query.Get("max_execution_time") != "30" {
			t.Fatal("parity query must have an execution-time cap")
		}
		if query.Get("max_rows_to_read") != "10000000" {
			t.Fatal("parity query must have a scan cap")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"count":17}]}`))
	}))
	defer server.Close()

	client, err := New(Config{
		HTTPURL:    server.URL,
		User:       "shadow-user",
		Password:   "shadow-password",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	count, err := client.CountDistinctStreamEvents(
		context.Background(),
		time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("count distinct: %v", err)
	}
	if count != 17 {
		t.Fatalf("count=%d", count)
	}
}

func TestStreamEventsFingerprintUsesFinalBoundedReadAndMatchesSourceAccumulator(t *testing.T) {
	events := []StreamEvent{
		{
			EventID: "11111111-1111-1111-1111-111111111111", Provider: "solana_wss", StreamMode: "logs_subscribe",
			Network: "solana-mainnet", ModuleID: "pump_sybil_radar", EventType: "event", Target: "AbCd",
			TargetType: "token", Signature: "SigA", Slot: 10, ProgramID: "ProgramA", EvidenceQuality: "raw_stream",
			Decoded: json.RawMessage(`{"a":1}`), RawEvent: json.RawMessage(`{"raw":"a"}`),
			CreatedAt: time.Date(2026, 9, 7, 1, 2, 3, 456789000, time.UTC),
		},
		{
			EventID: "22222222-2222-2222-2222-222222222222", Provider: "solana_wss", StreamMode: "logs_subscribe",
			Network: "solana-mainnet", ModuleID: "raydium_pool_guardian", EventType: "event", Target: "EfGh",
			TargetType: "token", Signature: "SigB", Slot: 11, ProgramID: "ProgramB", EvidenceQuality: "raw_stream",
			Decoded: json.RawMessage(`{"b":2}`), RawEvent: json.RawMessage(`{"raw":"b"}`),
			CreatedAt: time.Date(2026, 9, 7, 1, 2, 4, 987654000, time.UTC),
		},
	}

	expected := NewStreamParityAccumulator()
	stored := make([]streamParityRow, 0, len(events))
	for _, event := range events {
		if err := expected.Add(event); err != nil {
			t.Fatalf("add expected fingerprint event: %v", err)
		}
		row, err := normalizeStreamEvent(event, 1)
		if err != nil {
			t.Fatalf("normalize test event: %v", err)
		}
		stored = append(stored, parityRowFromStreamEventRow(row))
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if !strings.Contains(query.Get("query"), "FROM security_radar_stream_events FINAL") {
			t.Fatalf("query must use FINAL: %q", query.Get("query"))
		}
		if !strings.Contains(query.Get("query"), "decoded_sha256") || !strings.Contains(query.Get("query"), "raw_event_sha256") {
			t.Fatal("content parity query must include payload hashes")
		}
		if query.Get("max_rows_to_read") != "10000000" || query.Get("max_execution_time") != "60" {
			t.Fatal("content parity query must be bounded")
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		encoder := json.NewEncoder(w)
		for _, row := range stored {
			if err := encoder.Encode(row); err != nil {
				t.Fatalf("encode parity row: %v", err)
			}
		}
	}))
	defer server.Close()

	client, err := New(Config{HTTPURL: server.URL, User: "shadow-user", Password: "shadow-password", HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	fingerprint, count, err := client.StreamEventsFingerprint(
		context.Background(),
		time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC),
		100,
	)
	if err != nil {
		t.Fatalf("stream fingerprint: %v", err)
	}
	if count != expected.Count() {
		t.Fatalf("count=%d expected=%d", count, expected.Count())
	}
	if fingerprint != expected.SumHex() {
		t.Fatalf("fingerprint=%s expected=%s", fingerprint, expected.SumHex())
	}
}

func TestStreamParityFingerprintChangesWhenEvidenceChanges(t *testing.T) {
	base := StreamEvent{
		EventID: "11111111-1111-1111-1111-111111111111", Provider: "solana_wss", StreamMode: "logs_subscribe",
		Network: "solana-mainnet", ModuleID: "pump_sybil_radar", EventType: "event", Target: "AbCd",
		TargetType: "token", Signature: "SigA", ProgramID: "ProgramA", EvidenceQuality: "raw_stream",
		Decoded: json.RawMessage(`{"ok":true}`), RawEvent: json.RawMessage(`{"raw":1}`),
		CreatedAt: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
	}
	first := NewStreamParityAccumulator()
	if err := first.Add(base); err != nil {
		t.Fatal(err)
	}
	changed := base
	changed.RawEvent = json.RawMessage(`{"raw":2}`)
	second := NewStreamParityAccumulator()
	if err := second.Add(changed); err != nil {
		t.Fatal(err)
	}
	if first.SumHex() == second.SumHex() {
		t.Fatal("content fingerprint must change when raw evidence changes")
	}
}

func TestNewRejectsCredentialBearingOrInsecureURLs(t *testing.T) {
	for _, rawURL := range []string{
		"http://clickhouse.example.test:8443",
		"https://user:secret@clickhouse.example.test:8443",
	} {
		t.Run(strings.ReplaceAll(rawURL, "/", "_"), func(t *testing.T) {
			_, err := New(Config{HTTPURL: rawURL, Password: "secret"})
			if err == nil {
				t.Fatalf("expected URL %q to be rejected", rawURL)
			}
		})
	}
}
