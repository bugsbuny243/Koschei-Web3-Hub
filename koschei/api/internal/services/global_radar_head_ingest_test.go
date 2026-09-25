package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"koschei/api/internal/radarevent"
	"koschei/api/internal/runtimehealth"
)

func TestRunGlobalRadarHeadIngestCyclePersistsOnlyNewEVMHead(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "eth_chainId":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
		case "eth_blockNumber":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x64"})
		default:
			http.Error(w, "unexpected", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	sink := &globalRadarTelemetryRecordingEventSink{}
	health := runtimehealth.New()
	cfg := GlobalRadarHeadIngestConfig{
		EventSink: sink,
		Targets: []GlobalRadarHeadIngestTarget{{Kind: GlobalRadarHeadIngestEVM, NetworkID: "ethereum-mainnet", Endpoint: server.URL}},
		HTTPClient: server.Client(),
		Health: health,
		Now: func() time.Time { return time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC) },
	}
	lastSeen := map[string]uint64{}
	count, err := RunGlobalRadarHeadIngestCycle(context.Background(), cfg, lastSeen)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || len(sink.events) != 1 {
		t.Fatalf("count=%d events=%d", count, len(sink.events))
	}
	if sink.events[0].Kind != radarevent.KindBlock || sink.events[0].SubjectID != "ethereum-mainnet:height:100" {
		t.Fatalf("unexpected event: %#v", sink.events[0])
	}
	if err := sink.events[0].Verify(); err != nil {
		t.Fatal(err)
	}
	count, err = RunGlobalRadarHeadIngestCycle(context.Background(), cfg, lastSeen)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 || len(sink.events) != 1 {
		t.Fatalf("duplicate head persisted: count=%d events=%d", count, len(sink.events))
	}
}

func TestRunGlobalRadarHeadIngestCyclePersistsSuccessfulTargetAndReportsFailure(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "eth_chainId":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
		case "eth_blockNumber":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x65"})
		default:
			http.Error(w, "unexpected", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	sink := &globalRadarTelemetryRecordingEventSink{}
	health := runtimehealth.New()
	cfg := GlobalRadarHeadIngestConfig{
		EventSink: sink,
		Targets: []GlobalRadarHeadIngestTarget{
			{Kind: GlobalRadarHeadIngestEVM, NetworkID: "ethereum-mainnet", Endpoint: server.URL},
			{Kind: "unsupported", NetworkID: "base-mainnet", Endpoint: server.URL},
		},
		HTTPClient: server.Client(),
		Health: health,
		Now: func() time.Time { return time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC) },
	}
	count, err := RunGlobalRadarHeadIngestCycle(context.Background(), cfg, map[string]uint64{})
	if err == nil {
		t.Fatal("expected partial-cycle error")
	}
	if count != 1 || len(sink.events) != 1 {
		t.Fatalf("count=%d events=%d", count, len(sink.events))
	}
	snapshot := health.Snapshot()
	foundUnavailable := false
	for _, entry := range snapshot.Entries {
		if entry.NetworkID == "base-mainnet" && entry.State == runtimehealth.StateUnavailable {
			foundUnavailable = true
		}
	}
	if !foundUnavailable {
		t.Fatalf("missing unavailable target health: %#v", snapshot)
	}
}
