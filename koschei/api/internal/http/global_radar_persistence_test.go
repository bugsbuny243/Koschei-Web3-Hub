package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"koschei/api/internal/radarevent"
	"koschei/api/internal/services"
)

type recordingGlobalRadarSink struct {
	snapshots []services.GlobalRadarSnapshot
	err       error
}

type recordingGlobalRadarEventSink struct {
	events []radarevent.Event
	err    error
}

func (s *recordingGlobalRadarEventSink) InsertGlobalRadarEvents(_ context.Context, events []radarevent.Event) error {
	if s.err != nil {
		return s.err
	}
	s.events = append(s.events, events...)
	return nil
}

func (s *recordingGlobalRadarSink) InsertGlobalRadarSnapshot(_ context.Context, snapshot services.GlobalRadarSnapshot) error {
	if s.err != nil {
		return s.err
	}
	s.snapshots = append(s.snapshots, snapshot)
	return nil
}

func globalRadarPersistenceTestRPC(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode rpc request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "eth_chainId":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
		case "eth_getCode":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":"0x60016000"}`))
		default:
			http.Error(w, "unexpected method", http.StatusBadRequest)
		}
	}))
}

func TestNetworkProbeIntelligencePersistsCanonicalSnapshotWhenSinkConfigured(t *testing.T) {
	rpc := globalRadarPersistenceTestRPC(t)
	defer rpc.Close()
	t.Setenv("ETHEREUM_RPC_URL", rpc.URL)

	sink := &recordingGlobalRadarSink{}
	request := httptest.NewRequest(http.MethodPost, "/fabric/networks/probe/intelligence", strings.NewReader(`{"network":"ethereum-mainnet","address":"0x1111111111111111111111111111111111111111"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	networkTargetProbeWithDependencies(response, request, rpc.Client(), sink)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(sink.snapshots) != 1 {
		t.Fatalf("persisted snapshots=%d want 1", len(sink.snapshots))
	}
	snapshot := sink.snapshots[0]
	if snapshot.SchemaVersion != services.GlobalRadarSnapshotSchemaVersion ||
		len(snapshot.Observations) != 1 ||
		snapshot.Observations[0].Subject.Network != "ethereum-mainnet" ||
		snapshot.Coverage.ObservationCount != 1 ||
		snapshot.Coverage.RiskScoreProduced {
		t.Fatalf("unexpected persisted snapshot: %#v", snapshot)
	}

	var payload struct {
		RadarPersistence string `json:"radar_persistence"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.RadarPersistence != "clickhouse" {
		t.Fatalf("radar_persistence=%q", payload.RadarPersistence)
	}
}

func TestNetworkProbeIntelligenceFailsClosedWhenConfiguredSinkCannotPersist(t *testing.T) {
	rpc := globalRadarPersistenceTestRPC(t)
	defer rpc.Close()
	t.Setenv("ETHEREUM_RPC_URL", rpc.URL)

	sink := &recordingGlobalRadarSink{err: errors.New("clickhouse unavailable")}
	request := httptest.NewRequest(http.MethodPost, "/fabric/networks/probe/intelligence", strings.NewReader(`{"network":"ethereum-mainnet","address":"0x1111111111111111111111111111111111111111"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	networkTargetProbeWithDependencies(response, request, rpc.Client(), sink)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"error":"radar_persistence_unavailable"`) {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}

func TestNetworkProbeIntelligenceEmitsAndPersistsNativeDigestEVMEvent(t *testing.T) {
	rpc := globalRadarPersistenceTestRPC(t)
	defer rpc.Close()
	t.Setenv("ETHEREUM_RPC_URL", rpc.URL)

	snapshotSink := &recordingGlobalRadarSink{}
	eventSink := &recordingGlobalRadarEventSink{}
	request := httptest.NewRequest(http.MethodPost, "/fabric/networks/probe/intelligence", strings.NewReader(`{"network":"ethereum-mainnet","address":"0x1111111111111111111111111111111111111111"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	networkTargetProbeWithStores(response, request, rpc.Client(), snapshotSink, eventSink)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(eventSink.events) != 1 {
		t.Fatalf("persisted events=%d want 1", len(eventSink.events))
	}
	event := eventSink.events[0]
	if err := event.Verify(); err != nil {
		t.Fatal(err)
	}
	if len(event.SourceDigests) != 2 {
		t.Fatalf("source digests=%#v", event.SourceDigests)
	}

	var payload struct {
		RadarEvent            *radarevent.Event `json:"radar_event"`
		RadarEventPersistence string            `json:"radar_event_persistence"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.RadarEvent == nil || payload.RadarEvent.EventSHA256 != event.EventSHA256 {
		t.Fatalf("response event mismatch: %#v", payload.RadarEvent)
	}
	if payload.RadarEventPersistence != "clickhouse" {
		t.Fatalf("radar_event_persistence=%q", payload.RadarEventPersistence)
	}
}

func TestNetworkProbeIntelligenceFailsClosedWhenEventSinkCannotPersist(t *testing.T) {
	rpc := globalRadarPersistenceTestRPC(t)
	defer rpc.Close()
	t.Setenv("ETHEREUM_RPC_URL", rpc.URL)

	eventSink := &recordingGlobalRadarEventSink{err: errors.New("event clickhouse unavailable")}
	request := httptest.NewRequest(http.MethodPost, "/fabric/networks/probe/intelligence", strings.NewReader(`{"network":"ethereum-mainnet","address":"0x1111111111111111111111111111111111111111"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	networkTargetProbeWithStores(response, request, rpc.Client(), nil, eventSink)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"error":"radar_event_persistence_unavailable"`) {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}
