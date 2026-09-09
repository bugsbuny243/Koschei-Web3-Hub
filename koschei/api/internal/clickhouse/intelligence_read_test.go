package clickhouse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestReadARVISMemoryCorrelatesExactEvidenceWithoutLowercasingTarget(t *testing.T) {
	target := "AbCdEf123CaseSensitiveTarget"
	network := "solana-mainnet"
	since := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	until := since.Add(24 * time.Hour)
	evidence := []string{"tx:abc", "slot:7"}
	signals, err := canonicalJSON(json.RawMessage(`{"creator_wallet":"AbCdEfCreator"}`))
	if err != nil {
		t.Fatal(err)
	}
	verdictID := "11111111-1111-1111-1111-111111111111"
	eventID := "22222222-2222-2222-2222-222222222222"
	queries := 0

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries++
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		q := r.URL.Query().Get("query")
		if strings.Contains(strings.ToLower(q), "lower(") || strings.Contains(strings.ToLower(q), "lowerutf8") {
			t.Fatalf("query must not lowercase chain-native targets: %s", q)
		}
		if got := r.URL.Query().Get("param_target"); got != target {
			t.Fatalf("param_target=%q", got)
		}
		if got := r.URL.Query().Get("param_network"); got != network {
			t.Fatalf("param_network=%q", got)
		}
		if got := r.Header.Get("X-ClickHouse-Key"); got != "secret" {
			t.Fatalf("missing ClickHouse auth header")
		}

		encoder := json.NewEncoder(w)
		switch {
		case strings.Contains(q, "FROM arvis_verdict_snapshots FINAL"):
			_ = encoder.Encode(map[string]any{
				"verdict_id": verdictID, "event_id": eventID, "module_id": "final_verdict_engine",
				"target": target, "target_type": "token", "network": network,
				"grade": "B", "risk_index": 62, "risk_level": "high", "verdict": "BLOCK", "recommendation": "review",
				"evidence": evidence, "evidence_sha256": evidenceListSHA256(evidence),
				"signals": json.RawMessage(signals), "signals_sha256": jsonPayloadSHA256(signals),
				"rule_version": "rules-v2", "signed": 1, "signature": "signed-verdict", "source": "arvis_stream",
				"event_type": "arvis_stream_verdict", "provider": "solana_rpc",
				"created_at_us": since.Add(time.Hour).UnixMicro(), "source_updated_at_us": since.Add(2 * time.Hour).UnixMicro(),
				"source_version": uint64(since.Add(2 * time.Hour).UnixMicro()), "schema_version": VerdictSnapshotSchemaVersion,
			})
		case strings.Contains(q, "FROM security_radar_stream_events FINAL"):
			_ = encoder.Encode(map[string]any{
				"event_id": eventID, "event_key": strings.Repeat("a", 64), "provider": "solana_rpc", "stream_mode": "journal",
				"network": network, "module_id": "final_verdict_engine", "event_type": "trade", "target": target, "target_type": "token",
				"signature": "chain-signature", "slot": 7, "program_id": "ProgramCaseSensitive", "evidence_quality": "raw_stream",
				"decoded_sha256": strings.Repeat("b", 64), "raw_event_sha256": strings.Repeat("c", 64),
				"created_at_ms": since.Add(time.Hour).UnixMilli(), "ingest_version": uint64(7), "schema_version": StreamSchemaVersion,
			})
		default:
			t.Fatalf("unexpected query: %s", q)
		}
	}))
	defer server.Close()

	client, err := New(Config{HTTPURL: server.URL, Database: DefaultDatabase, User: "default", Password: "secret", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := client.ReadARVISMemory(t.Context(), ARVISMemoryReadRequest{Target: target, Network: network, Since: since, Until: until, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if queries != 2 {
		t.Fatalf("queries=%d", queries)
	}
	if snapshot.CorrelationStatus != "correlated" || snapshot.LinkedVerdicts != 1 || snapshot.MissingLinkedEvents != 0 {
		t.Fatalf("unexpected correlation: %#v", snapshot)
	}
	if len(snapshot.Links) != 1 || snapshot.Links[0].Status != "linked" {
		t.Fatalf("links=%#v", snapshot.Links)
	}
	if snapshot.Target != target || snapshot.Events[0].Target != target || snapshot.Verdicts[0].Target != target {
		t.Fatalf("case-sensitive target changed: %#v", snapshot)
	}
	if snapshot.VerdictContentIntegrity != "sha256_verified" || len(snapshot.FingerprintSHA256) != 64 {
		t.Fatalf("integrity/fingerprint=%q/%q", snapshot.VerdictContentIntegrity, snapshot.FingerprintSHA256)
	}
}

func TestReadARVISMemoryRejectsVerdictContentHashMismatch(t *testing.T) {
	target := "TargetExact"
	network := "solana-mainnet"
	since := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	until := since.Add(time.Hour)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		if !strings.Contains(q, "FROM arvis_verdict_snapshots FINAL") {
			t.Fatalf("stream query must not run after verdict integrity failure")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"verdict_id": "11111111-1111-1111-1111-111111111111", "event_id": zeroUUID, "module_id": "final_verdict_engine",
			"target": target, "target_type": "token", "network": network, "grade": "C", "risk_index": 40, "risk_level": "medium",
			"verdict": "REVIEW", "recommendation": "review", "evidence": []string{"tx:abc"}, "evidence_sha256": strings.Repeat("a", 64),
			"signals": json.RawMessage(`{}`), "signals_sha256": jsonPayloadSHA256(json.RawMessage(`{}`)), "rule_version": "rules-v1",
			"signed": 1, "signature": "sig", "source": "arvis", "event_type": "", "provider": "", "created_at_us": since.UnixMicro(),
			"source_updated_at_us": since.UnixMicro(), "source_version": uint64(since.UnixMicro()), "schema_version": VerdictSnapshotSchemaVersion,
		})
	}))
	defer server.Close()
	client, err := New(Config{HTTPURL: server.URL, Password: "secret", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ReadARVISMemory(t.Context(), ARVISMemoryReadRequest{Target: target, Network: network, Since: since, Until: until, Limit: 10})
	if err == nil || !strings.Contains(err.Error(), "evidence hash mismatch") {
		t.Fatalf("expected evidence hash mismatch, got %v", err)
	}
}

func TestCorrelateARVISMemoryKeepsMissingEventExplicit(t *testing.T) {
	req := ARVISMemoryReadRequest{Target: "ExactTarget", Network: "solana-mainnet", Since: time.Unix(1, 0).UTC(), Until: time.Unix(2, 0).UTC(), Limit: 10}
	verdicts := []ARVISVerdictMemory{{VerdictID: "11111111-1111-1111-1111-111111111111", EventID: "22222222-2222-2222-2222-222222222222"}}
	snapshot := correlateARVISMemory(req, verdicts, nil)
	if snapshot.CorrelationStatus != "partial" || snapshot.MissingLinkedEvents != 1 {
		t.Fatalf("snapshot=%#v", snapshot)
	}
	if len(snapshot.Links) != 1 || snapshot.Links[0].Status != "event_not_observed_in_window" {
		t.Fatalf("links=%#v", snapshot.Links)
	}
}

func TestNormalizeARVISMemoryReadRequestIsBounded(t *testing.T) {
	now := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	cases := []ARVISMemoryReadRequest{
		{Network: "solana-mainnet", Since: now.Add(-time.Hour), Until: now, Limit: 1},
		{Target: "x", Since: now.Add(-time.Hour), Until: now, Limit: 1},
		{Target: "x", Network: "solana-mainnet", Since: now, Until: now, Limit: 1},
		{Target: "x", Network: "solana-mainnet", Since: now.Add(-MaxARVISMemoryReadWindow - time.Second), Until: now, Limit: 1},
		{Target: "x", Network: "solana-mainnet", Since: now.Add(-time.Hour), Until: now, Limit: MaxARVISMemoryReadRows + 1},
	}
	for i, request := range cases {
		if _, err := normalizeARVISMemoryReadRequest(request); err == nil {
			t.Fatalf("case %d unexpectedly accepted", i)
		}
	}
}

func TestARVISMemoryQueryParamsUseBoundedServerSettings(t *testing.T) {
	values := url.Values{}
	now := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	setARVISMemoryQueryParams(values, ARVISMemoryReadRequest{Target: "CaseTarget", Network: "solana-mainnet", Since: now.Add(-time.Hour), Until: now, Limit: 9}, false)
	if values.Get("param_target") != "CaseTarget" || values.Get("param_limit") != "10" {
		t.Fatalf("params=%v", values)
	}
	if values.Get("max_rows_to_read") != "5000000" || values.Get("max_execution_time") != "30" || values.Get("result_overflow_mode") != "throw" {
		t.Fatalf("server bounds missing: %v", values)
	}
}
