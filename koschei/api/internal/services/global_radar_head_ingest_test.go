package services

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"koschei/api/internal/radarcursor"
	"koschei/api/internal/radarevent"
)

type headIngestCursorStore struct {
	items map[string]radarcursor.Checkpoint
	saves []radarcursor.Checkpoint
}

func newHeadIngestCursorStore() *headIngestCursorStore {
	return &headIngestCursorStore{items: map[string]radarcursor.Checkpoint{}}
}

func (s *headIngestCursorStore) LoadGlobalRadarIngestCheckpoint(_ context.Context, key string) (radarcursor.Checkpoint, bool, error) {
	item, ok := s.items[key]
	return item, ok, nil
}

func (s *headIngestCursorStore) SaveGlobalRadarIngestCheckpoint(_ context.Context, checkpoint radarcursor.Checkpoint) error {
	canonical, err := checkpoint.Canonical()
	if err != nil {
		return err
	}
	s.items[canonical.CursorKey] = canonical
	s.saves = append(s.saves, canonical)
	return nil
}

func (s *headIngestCursorStore) LoadGlobalRadarCanonicalCheckpointAtHeight(_ context.Context, key string, height uint64) (radarcursor.Checkpoint, bool, error) {
	for i := len(s.saves) - 1; i >= 0; i-- {
		item := s.saves[i]
		if item.CursorKey == key && item.Height == height && item.State == radarcursor.StateCanonical {
			return item, true, nil
		}
	}
	item, ok := s.items[key]
	if ok && item.Height == height && item.State == radarcursor.StateCanonical {
		return item, true, nil
	}
	return radarcursor.Checkpoint{}, false, nil
}

type headIngestEventSink struct {
	events []radarevent.Event
}

func (s *headIngestEventSink) InsertGlobalRadarEvents(_ context.Context, events []radarevent.Event) error {
	s.events = append(s.events, events...)
	return nil
}

func evmHeadIngestServer(t *testing.T, head uint64, blockHash, parentHash string, txHashes []string, withLog bool) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct {
			ID     int               `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "eth_chainId":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
		case "eth_blockNumber":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": formatHex(head)})
		case "eth_getBlockByNumber":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{
				"number": formatHex(head), "hash": blockHash, "parentHash": parentHash, "timestamp": "0x64", "transactions": txHashes,
			}})
		case "eth_getLogs":
			logs := []map[string]any{}
			if withLog {
				txHash := txHashes[0]
				logs = append(logs, map[string]any{
					"address":         "0x" + strings.Repeat("e", 40),
					"topics":          []string{"0x" + strings.Repeat("f", 64)},
					"data":            "0x1234",
					"transactionHash": txHash,
					"blockHash":       blockHash,
					"blockNumber":     formatHex(head),
					"logIndex":        "0x0",
					"removed":         false,
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": logs})
		default:
			http.Error(w, "unexpected method", http.StatusBadRequest)
		}
	}))
}

func formatHex(value uint64) string {
	const digits = "0123456789abcdef"
	if value == 0 {
		return "0x0"
	}
	var buf [16]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = digits[value&0xf]
		value >>= 4
	}
	return "0x" + string(buf[i:])
}

func TestRunGlobalRadarHeadIngestCycleBootstrapsDurableEVMBlockBundle(t *testing.T) {
	blockHash := "0x" + strings.Repeat("a", 64)
	parentHash := "0x" + strings.Repeat("b", 64)
	txHash := "0x" + strings.Repeat("c", 64)
	server := evmHeadIngestServer(t, 100, blockHash, parentHash, []string{txHash}, true)
	defer server.Close()

	sink := &headIngestEventSink{}
	store := newHeadIngestCursorStore()
	target := GlobalRadarHeadIngestTarget{Kind: GlobalRadarHeadIngestEVM, NetworkID: "ethereum-mainnet", Endpoint: server.URL}
	cfg := GlobalRadarHeadIngestConfig{
		EventSink: sink, CursorStore: store, Targets: []GlobalRadarHeadIngestTarget{target},
		HTTPClient: server.Client(), Now: func() time.Time { return time.Date(2026, 9, 25, 13, 0, 0, 0, time.UTC) },
	}

	count, err := RunGlobalRadarHeadIngestCycle(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 || len(sink.events) != 3 {
		t.Fatalf("count=%d events=%d", count, len(sink.events))
	}
	if sink.events[0].Kind != radarevent.KindBlock || sink.events[1].Kind != radarevent.KindTransaction || sink.events[2].Kind != radarevent.KindLog {
		t.Fatalf("unexpected event kinds: %#v", sink.events)
	}
	cursor, ok := store.items[GlobalRadarHeadIngestCursorKey(target)]
	if !ok || cursor.Height != 100 || cursor.BlockHash != blockHash || cursor.State != radarcursor.StateCanonical {
		t.Fatalf("unexpected cursor: ok=%t %#v", ok, cursor)
	}
	if cursor.SourceEventSHA256 != sink.events[0].EventSHA256 {
		t.Fatal("cursor is not bound to canonical block event")
	}

	count, err = RunGlobalRadarHeadIngestCycle(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 || len(sink.events) != 3 {
		t.Fatalf("same head replay persisted: count=%d events=%d", count, len(sink.events))
	}
}

func TestRunGlobalRadarHeadIngestCycleAdvancesOnlyWhenParentMatches(t *testing.T) {
	parentHash := "0x" + strings.Repeat("a", 64)
	blockHash := "0x" + strings.Repeat("d", 64)
	server := evmHeadIngestServer(t, 101, blockHash, parentHash, nil, false)
	defer server.Close()

	store := newHeadIngestCursorStore()
	target := GlobalRadarHeadIngestTarget{Kind: GlobalRadarHeadIngestEVM, NetworkID: "ethereum-mainnet", Endpoint: server.URL}
	key := GlobalRadarHeadIngestCursorKey(target)
	store.items[key] = radarcursor.Checkpoint{
		CursorKey: key, NetworkID: target.NetworkID, StreamKind: target.Kind, Height: 100,
		BlockHash: parentHash, ParentHash: "0x" + strings.Repeat("b", 64),
		SourceEventSHA256: strings.Repeat("1", 64), State: radarcursor.StateCanonical,
		ObservedAt: time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC),
	}
	sink := &headIngestEventSink{}
	cfg := GlobalRadarHeadIngestConfig{EventSink: sink, CursorStore: store, Targets: []GlobalRadarHeadIngestTarget{target}, HTTPClient: server.Client()}

	count, err := RunGlobalRadarHeadIngestCycle(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || len(sink.events) != 1 || sink.events[0].Kind != radarevent.KindBlock {
		t.Fatalf("count=%d events=%#v", count, sink.events)
	}
	cursor := store.items[key]
	if cursor.Height != 101 || cursor.BlockHash != blockHash || cursor.ParentHash != parentHash || cursor.State != radarcursor.StateCanonical {
		t.Fatalf("cursor did not advance canonically: %#v", cursor)
	}
}

func TestRunGlobalRadarHeadIngestCycleFreezesOnReorg(t *testing.T) {
	storedHash := "0x" + strings.Repeat("a", 64)
	observedParent := "0x" + strings.Repeat("f", 64)
	blockHash := "0x" + strings.Repeat("d", 64)
	server := evmHeadIngestServer(t, 101, blockHash, observedParent, nil, false)
	defer server.Close()

	store := newHeadIngestCursorStore()
	target := GlobalRadarHeadIngestTarget{Kind: GlobalRadarHeadIngestEVM, NetworkID: "ethereum-mainnet", Endpoint: server.URL}
	key := GlobalRadarHeadIngestCursorKey(target)
	store.items[key] = radarcursor.Checkpoint{
		CursorKey: key, NetworkID: target.NetworkID, StreamKind: target.Kind, Height: 100,
		BlockHash: storedHash, ParentHash: "0x" + strings.Repeat("b", 64),
		SourceEventSHA256: strings.Repeat("1", 64), State: radarcursor.StateCanonical,
		ObservedAt: time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC),
	}
	sink := &headIngestEventSink{}
	cfg := GlobalRadarHeadIngestConfig{EventSink: sink, CursorStore: store, Targets: []GlobalRadarHeadIngestTarget{target}, HTTPClient: server.Client()}

	count, err := RunGlobalRadarHeadIngestCycle(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "reorg observed") {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if count != 0 || len(sink.events) != 0 {
		t.Fatalf("reorg block must not persist events: count=%d events=%d", count, len(sink.events))
	}
	cursor := store.items[key]
	if cursor.Height != 100 || cursor.BlockHash != storedHash || cursor.State != radarcursor.StateReorgObserved {
		t.Fatalf("cursor not frozen on old canonical checkpoint: %#v", cursor)
	}

	count, err = RunGlobalRadarHeadIngestCycle(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "frozen") || count != 0 {
		t.Fatalf("frozen cursor unexpectedly advanced: count=%d err=%v", count, err)
	}
}


type evmLineageFixture struct {
	hash       string
	parentHash string
}

func trustedTLSServerClient(servers ...*httptest.Server) *http.Client {
	pool := x509.NewCertPool()
	for _, server := range servers {
		pool.AddCert(server.Certificate())
	}
	return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}}
}

func evmLineageServer(t *testing.T, head uint64, blocks map[uint64]evmLineageFixture) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct {
			ID     int               `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "eth_chainId":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
		case "eth_blockNumber":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": formatHex(head)})
		case "eth_getBlockByNumber":
			if len(req.Params) == 0 {
				http.Error(w, "missing height", http.StatusBadRequest)
				return
			}
			var tag string
			if err := json.Unmarshal(req.Params[0], &tag); err != nil {
				t.Fatal(err)
			}
			var height uint64
			for _, candidate := range []uint64{head, 100, 99, 98} {
				if formatHex(candidate) == tag {
					height = candidate
					break
				}
			}
			fixture, ok := blocks[height]
			if !ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": nil})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{
				"number": formatHex(height), "hash": fixture.hash, "parentHash": fixture.parentHash,
				"timestamp": "0x64", "transactions": []string{},
			}})
		case "eth_getLogs":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": []map[string]any{}})
		default:
			http.Error(w, "unexpected method", http.StatusBadRequest)
		}
	}))
}

func TestRunGlobalRadarHeadIngestCycleRejectsProviderDisagreement(t *testing.T) {
	parentHash := "0x" + strings.Repeat("b", 64)
	primaryHash := "0x" + strings.Repeat("a", 64)
	confirmationHash := "0x" + strings.Repeat("c", 64)
	primary := evmLineageServer(t, 100, map[uint64]evmLineageFixture{100: {hash: primaryHash, parentHash: parentHash}})
	defer primary.Close()
	confirmation := evmLineageServer(t, 100, map[uint64]evmLineageFixture{100: {hash: confirmationHash, parentHash: parentHash}})
	defer confirmation.Close()

	store := newHeadIngestCursorStore()
	sink := &headIngestEventSink{}
	target := GlobalRadarHeadIngestTarget{
		Kind: GlobalRadarHeadIngestEVM, NetworkID: "ethereum-mainnet",
		Endpoint: primary.URL, ConfirmationEndpoint: confirmation.URL,
	}
	cfg := GlobalRadarHeadIngestConfig{
		EventSink: sink, CursorStore: store, Targets: []GlobalRadarHeadIngestTarget{target},
		HTTPClient: trustedTLSServerClient(primary, confirmation), RequireConfirmation: true,
	}

	count, err := RunGlobalRadarHeadIngestCycle(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "multi-provider block disagreement") {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if count != 0 || len(sink.events) != 0 || len(store.saves) != 0 {
		t.Fatalf("provider disagreement persisted state: count=%d events=%d saves=%d", count, len(sink.events), len(store.saves))
	}
}

func TestRunGlobalRadarHeadIngestCycleAutomaticallyRewindsToTwoProviderCommonAncestor(t *testing.T) {
	hash98 := "0x" + strings.Repeat("8", 64)
	hash99 := "0x" + strings.Repeat("9", 64)
	old100 := "0x" + strings.Repeat("a", 64)
	new100 := "0x" + strings.Repeat("b", 64)
	new101 := "0x" + strings.Repeat("c", 64)

	blocks := map[uint64]evmLineageFixture{
		99:  {hash: hash99, parentHash: hash98},
		100: {hash: new100, parentHash: hash99},
		101: {hash: new101, parentHash: new100},
	}
	primary := evmLineageServer(t, 101, blocks)
	defer primary.Close()
	confirmation := evmLineageServer(t, 101, blocks)
	defer confirmation.Close()

	store := newHeadIngestCursorStore()
	target := GlobalRadarHeadIngestTarget{
		Kind: GlobalRadarHeadIngestEVM, NetworkID: "ethereum-mainnet",
		Endpoint: primary.URL, ConfirmationEndpoint: confirmation.URL,
	}
	key := GlobalRadarHeadIngestCursorKey(target)
	if err := store.SaveGlobalRadarIngestCheckpoint(context.Background(), radarcursor.Checkpoint{
		CursorKey: key, NetworkID: target.NetworkID, StreamKind: target.Kind, Height: 99,
		BlockHash: hash99, ParentHash: hash98, SourceEventSHA256: strings.Repeat("1", 64),
		State: radarcursor.StateCanonical, ObservedAt: time.Date(2026, 9, 25, 11, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveGlobalRadarIngestCheckpoint(context.Background(), radarcursor.Checkpoint{
		CursorKey: key, NetworkID: target.NetworkID, StreamKind: target.Kind, Height: 100,
		BlockHash: old100, ParentHash: hash99, SourceEventSHA256: strings.Repeat("2", 64),
		State: radarcursor.StateCanonical, ObservedAt: time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}

	sink := &headIngestEventSink{}
	cfg := GlobalRadarHeadIngestConfig{
		EventSink: sink, CursorStore: store, Targets: []GlobalRadarHeadIngestTarget{target},
		HTTPClient: trustedTLSServerClient(primary, confirmation), RequireConfirmation: true, AutoReorgRecovery: true, MaxReorgRewind: 8,
	}
	count, err := RunGlobalRadarHeadIngestCycle(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "rewound durable cursor to height 99") {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if count != 0 || len(sink.events) != 0 {
		t.Fatalf("reorg detection must not persist replacement block before rewind: count=%d events=%d", count, len(sink.events))
	}
	latest := store.items[key]
	if latest.Height != 99 || latest.BlockHash != hash99 || latest.State != radarcursor.StateRewind {
		t.Fatalf("unexpected rewind checkpoint: %#v", latest)
	}
}
