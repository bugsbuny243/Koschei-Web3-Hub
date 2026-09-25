package services

import (
	"context"
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
					"address": "0x" + strings.Repeat("e", 40),
					"topics": []string{"0x" + strings.Repeat("f", 64)},
					"data": "0x1234",
					"transactionHash": txHash,
					"blockHash": blockHash,
					"blockNumber": formatHex(head),
					"logIndex": "0x0",
					"removed": false,
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
