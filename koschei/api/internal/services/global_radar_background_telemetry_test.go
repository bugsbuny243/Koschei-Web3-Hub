package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"koschei/api/internal/radarevent"
)

type globalRadarTelemetryRecordingSink struct {
	snapshots []GlobalRadarSnapshot
}

type globalRadarTelemetryRecordingEventSink struct {
	events []radarevent.Event
}

func (s *globalRadarTelemetryRecordingSink) InsertGlobalRadarSnapshot(_ context.Context, snapshot GlobalRadarSnapshot) error {
	s.snapshots = append(s.snapshots, snapshot)
	return nil
}

func (s *globalRadarTelemetryRecordingEventSink) InsertGlobalRadarEvents(_ context.Context, events []radarevent.Event) error {
	s.events = append(s.events, events...)
	return nil
}

func TestCollectGlobalRadarBackgroundTelemetryPersistsEVMObservation(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "eth_chainId":
			_, _ = w.Write([]byte("{\"jsonrpc\":\"2.0\",\"id\":101,\"result\":\"0x1\"}"))
		case "web3_clientVersion":
			_, _ = w.Write([]byte("{\"jsonrpc\":\"2.0\",\"id\":102,\"result\":\"Geth/v1.16\"}"))
		case "net_peerCount":
			_, _ = w.Write([]byte("{\"jsonrpc\":\"2.0\",\"id\":103,\"result\":\"0xc\"}"))
		case "eth_blockNumber":
			_, _ = w.Write([]byte("{\"jsonrpc\":\"2.0\",\"id\":104,\"result\":\"0x17d7840\"}"))
		case "eth_syncing":
			_, _ = w.Write([]byte("{\"jsonrpc\":\"2.0\",\"id\":105,\"result\":false}"))
		default:
			http.Error(w, "unexpected method", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	sink := &globalRadarTelemetryRecordingSink{}
	eventSink := &globalRadarTelemetryRecordingEventSink{}
	now := time.Date(2026, 9, 24, 6, 0, 0, 0, time.UTC)
	snapshot, err := CollectGlobalRadarBackgroundTelemetry(context.Background(), GlobalRadarBackgroundTelemetryConfig{
		Sink:      sink,
		EventSink: eventSink,
		Targets: []GlobalRadarTelemetryTarget{{
			Kind: GlobalRadarTelemetryEVMNode, NetworkID: "ethereum-mainnet", Endpoint: server.URL,
		}},
		HTTPClient: server.Client(),
		Now:        func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sink.snapshots) != 1 || len(snapshot.Observations) != 1 {
		t.Fatalf("snapshots=%d observations=%d", len(sink.snapshots), len(snapshot.Observations))
	}
	if len(eventSink.events) != 1 {
		t.Fatalf("events=%d want 1", len(eventSink.events))
	}
	if err := eventSink.events[0].Verify(); err != nil {
		t.Fatal(err)
	}
	if eventSink.events[0].NetworkID != "ethereum-mainnet" || eventSink.events[0].Kind != radarevent.KindNetworkHealth {
		t.Fatalf("unexpected event: %#v", eventSink.events[0])
	}
	if snapshot.Observations[0].Subject.Network != "ethereum-mainnet" || snapshot.Coverage.RiskScoreProduced {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}

func TestCollectGlobalRadarBackgroundTelemetryPersistsSuccessfulTargetsAndReportsPartialFailure(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		responses := map[string]string{
			"eth_chainId":        "{\"jsonrpc\":\"2.0\",\"id\":101,\"result\":\"0x1\"}",
			"web3_clientVersion": "{\"jsonrpc\":\"2.0\",\"id\":102,\"result\":\"Geth/v1.16\"}",
			"net_peerCount":      "{\"jsonrpc\":\"2.0\",\"id\":103,\"result\":\"0x1\"}",
			"eth_blockNumber":    "{\"jsonrpc\":\"2.0\",\"id\":104,\"result\":\"0x10\"}",
			"eth_syncing":        "{\"jsonrpc\":\"2.0\",\"id\":105,\"result\":false}",
		}
		if payload, ok := responses[request.Method]; ok {
			_, _ = w.Write([]byte(payload))
			return
		}
		http.Error(w, "unexpected", http.StatusBadRequest)
	}))
	defer server.Close()

	sink := &globalRadarTelemetryRecordingSink{}
	snapshot, err := CollectGlobalRadarBackgroundTelemetry(context.Background(), GlobalRadarBackgroundTelemetryConfig{
		Sink: sink,
		Targets: []GlobalRadarTelemetryTarget{
			{Kind: GlobalRadarTelemetryEVMNode, NetworkID: "ethereum-mainnet", Endpoint: server.URL},
			{Kind: "unsupported", NetworkID: "ethereum-mainnet", Endpoint: server.URL},
		},
		HTTPClient: server.Client(),
		Now:        func() time.Time { return time.Date(2026, 9, 24, 6, 0, 0, 0, time.UTC) },
	})
	if err == nil {
		t.Fatal("expected partial-cycle error")
	}
	if len(snapshot.Observations) != 1 || len(sink.snapshots) != 1 {
		t.Fatalf("partial cycle did not persist successful evidence: %#v", snapshot)
	}
}
