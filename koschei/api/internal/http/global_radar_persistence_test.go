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

func TestBitcoinNetworkProbeIntelligenceEmitsAndPersistsNativeDigestEvent(t *testing.T) {
	const address = "1BoatSLRHtKNngkdXEeobR76b53LETtpyT"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/block-height/0":
			_, _ = w.Write([]byte("000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f"))
		case "/address/" + address:
			_, _ = w.Write([]byte(`{"address":"1BoatSLRHtKNngkdXEeobR76b53LETtpyT","chain_stats":{"funded_txo_count":1,"funded_txo_sum":1000,"spent_txo_count":0,"spent_txo_sum":0,"tx_count":1},"mempool_stats":{"funded_txo_count":0,"funded_txo_sum":0,"spent_txo_count":0,"spent_txo_sum":0,"tx_count":0}}`))
		default:
			http.Error(w, "unexpected path", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	t.Setenv("BITCOIN_ESPLORA_URL", server.URL)

	snapshotSink := &recordingGlobalRadarSink{}
	eventSink := &recordingGlobalRadarEventSink{}
	request := httptest.NewRequest(http.MethodPost, "/fabric/networks/probe/intelligence", strings.NewReader(`{"network":"bitcoin-mainnet","address":"1BoatSLRHtKNngkdXEeobR76b53LETtpyT"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	networkTargetProbeWithStores(response, request, server.Client(), snapshotSink, eventSink)
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
	if event.NetworkID != "bitcoin-mainnet" || event.Kind != radarevent.KindAccount || len(event.SourceDigests) != 2 {
		t.Fatalf("unexpected bitcoin radar event: %#v", event)
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

func TestMoveNetworkProbeIntelligenceEmitsAndPersistsNativeDigestEvents(t *testing.T) {
	t.Run("sui", func(t *testing.T) {
		const address = "0x1111111111111111111111111111111111111111111111111111111111111111"
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"data":{"chainIdentifier":"4btiuiMPvEENsttpZC7CZ53DruC3MAgfznDbASZ7DR6S"}}`))
		}))
		defer server.Close()
		t.Setenv("SUI_GRAPHQL_URL", server.URL)

		eventSink := &recordingGlobalRadarEventSink{}
		request := httptest.NewRequest(http.MethodPost, "/fabric/networks/probe/intelligence", strings.NewReader(`{"network":"sui-mainnet","address":"0x1111111111111111111111111111111111111111111111111111111111111111"}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()

		networkTargetProbeWithStores(response, request, server.Client(), nil, eventSink)
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
		if event.NetworkID != "sui-mainnet" || event.Kind != radarevent.KindNetworkHealth || len(event.SourceDigests) != 1 {
			t.Fatalf("unexpected sui event: %#v", event)
		}
	})

	t.Run("aptos", func(t *testing.T) {
		const address = "0x2222222222222222222222222222222222222222222222222222222222222222"
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"chain_id":1,"epoch":"123","ledger_version":"456789","ledger_timestamp":"1700000000000000","block_height":"98765","node_role":"full_node","git_hash":"abcdef"}`))
		}))
		defer server.Close()
		t.Setenv("APTOS_REST_URL", server.URL)

		eventSink := &recordingGlobalRadarEventSink{}
		request := httptest.NewRequest(http.MethodPost, "/fabric/networks/probe/intelligence", strings.NewReader(`{"network":"aptos-mainnet","address":"0x2222222222222222222222222222222222222222222222222222222222222222"}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()

		networkTargetProbeWithStores(response, request, server.Client(), nil, eventSink)
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
		if event.NetworkID != "aptos-mainnet" || event.Kind != radarevent.KindNetworkHealth || len(event.SourceDigests) != 1 {
			t.Fatalf("unexpected aptos event: %#v", event)
		}
	})
}
