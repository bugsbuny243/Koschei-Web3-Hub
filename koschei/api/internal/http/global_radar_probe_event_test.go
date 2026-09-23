package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"koschei/api/internal/radarevent"
)

func TestGlobalRadarEVMProbeProducesVerifiableEvent(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
			ID     int    `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "eth_chainId":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
		case "eth_getCode":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":"0x6001600055"}`))
		default:
			t.Fatalf("unexpected rpc method %q", request.Method)
		}
	}))
	defer server.Close()
	t.Setenv("ETHEREUM_RPC_URL", server.URL)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/fabric/radar/probe-event",
		strings.NewReader(`{"network":"ethereum-mainnet","address":"0x1111111111111111111111111111111111111111"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	globalRadarProbeEventWithClient(response, request, server.Client())

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var envelope globalRadarProbeEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != globalRadarProbeEnvelopeSchema ||
		envelope.SourceBinding != "normalized_probe_json_sha256" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if envelope.RadarEvent.Kind != radarevent.KindAccount ||
		envelope.RadarEvent.NetworkID != "ethereum-mainnet" {
		t.Fatalf("unexpected radar event: %#v", envelope.RadarEvent)
	}
	if err := envelope.RadarEvent.Verify(); err != nil {
		t.Fatal(err)
	}
	if len(envelope.RadarEvent.SourceDigests) != 1 {
		t.Fatalf("source digests=%v", envelope.RadarEvent.SourceDigests)
	}
}

func TestGlobalRadarBitcoinProbeProducesVerifiableEvent(t *testing.T) {
	address := "1BoatSLRHtKNngkdXEeobR76b53LETtpyT"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/block-height/0":
			_, _ = w.Write([]byte("000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f"))
		case strings.HasPrefix(r.URL.Path, "/address/"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"address":"` + address + `","chain_stats":{"funded_txo_count":2,"funded_txo_sum":5000,"spent_txo_count":1,"spent_txo_sum":2000,"tx_count":3},"mempool_stats":{"funded_txo_count":0,"funded_txo_sum":0,"spent_txo_count":0,"spent_txo_sum":0,"tx_count":0}}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("BITCOIN_ESPLORA_URL", server.URL)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/fabric/radar/probe-event",
		strings.NewReader(`{"network":"bitcoin-mainnet","address":"`+address+`"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	globalRadarProbeEventWithClient(response, request, server.Client())

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var envelope globalRadarProbeEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.RadarEvent.NetworkID != "bitcoin-mainnet" ||
		envelope.RadarEvent.Kind != radarevent.KindAccount {
		t.Fatalf("unexpected radar event: %#v", envelope.RadarEvent)
	}
	if err := envelope.RadarEvent.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestGlobalRadarProbeRouteFailsClosedWithoutProviderConfiguration(t *testing.T) {
	t.Setenv("ETHEREUM_RPC_URL", "")
	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/fabric/radar/probe-event",
		strings.NewReader(`{"network":"ethereum-mainnet","address":"0x1111111111111111111111111111111111111111"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	MountFabric(http.NotFoundHandler()).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable ||
		!strings.Contains(response.Body.String(), "evm_rpc_configuration_required") {
		t.Fatalf("unexpected fail-closed response: %d %s", response.Code, response.Body.String())
	}
}

func TestGlobalRadarProbeRouteRejectsGET(t *testing.T) {
	response := httptest.NewRecorder()
	MountFabric(http.NotFoundHandler()).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/fabric/radar/probe-event", nil),
	)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
