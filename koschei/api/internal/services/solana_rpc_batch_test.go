package services

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type solanaRPCBatchTestRecorder struct {
	sync.Mutex
	batchSizes []int
	single     int
}

func (r *solanaRPCBatchTestRecorder) addBatch(size int) {
	r.Lock()
	defer r.Unlock()
	r.batchSizes = append(r.batchSizes, size)
}

func (r *solanaRPCBatchTestRecorder) addSingle() {
	r.Lock()
	defer r.Unlock()
	r.single++
}

func (r *solanaRPCBatchTestRecorder) snapshot() ([]int, int) {
	r.Lock()
	defer r.Unlock()
	return append([]int(nil), r.batchSizes...), r.single
}

func TestSolanaGetTransactionsJSONParsedBatchChunksAndMerges(t *testing.T) {
	resetSolanaRPCBatchTestState(t)
	t.Setenv("SOLANA_RPC_BATCH_CHUNK_SIZE", "10")
	recorder := &solanaRPCBatchTestRecorder{}
	server := newSolanaRPCBatchTestServer(t, recorder, func(size int, _ bool) int { return http.StatusOK })
	defer server.Close()

	results, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, testSignatures(25))
	if err != nil {
		t.Fatalf("batch request failed: %v", err)
	}
	if len(results) != 25 {
		t.Fatalf("results = %d, want 25", len(results))
	}
	batches, singles := recorder.snapshot()
	if got := intsCSV(batches); got != "10,10,5" {
		t.Fatalf("batch sizes = %s, want 10,10,5", got)
	}
	if singles != 0 {
		t.Fatalf("single calls = %d, want 0", singles)
	}
}

func TestSolanaGetTransactionsJSONParsedBatchHalvesOn413(t *testing.T) {
	resetSolanaRPCBatchTestState(t)
	t.Setenv("SOLANA_RPC_BATCH_CHUNK_SIZE", "10")
	recorder := &solanaRPCBatchTestRecorder{}
	server := newSolanaRPCBatchTestServer(t, recorder, func(size int, single bool) int {
		if !single && size == 10 {
			return http.StatusRequestEntityTooLarge
		}
		return http.StatusOK
	})
	defer server.Close()

	results, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, testSignatures(25))
	if err != nil {
		t.Fatalf("batch request failed: %v", err)
	}
	if len(results) != 25 {
		t.Fatalf("results = %d, want 25", len(results))
	}
	batches, singles := recorder.snapshot()
	if got := intsCSV(batches); got != "10,5,5,5,5,5" {
		t.Fatalf("batch sizes = %s, want 10,5,5,5,5,5", got)
	}
	if singles != 0 {
		t.Fatalf("single calls = %d, want 0", singles)
	}
}

func TestSolanaGetTransactionsJSONParsedBatchFallsBackToSinglesOn403Once(t *testing.T) {
	resetSolanaRPCBatchTestState(t)
	t.Setenv("SOLANA_RPC_BATCH_CHUNK_SIZE", "10")
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previous)
	recorder := &solanaRPCBatchTestRecorder{}
	server := newSolanaRPCBatchTestServer(t, recorder, func(_ int, single bool) int {
		if single {
			return http.StatusOK
		}
		return http.StatusForbidden
	})
	defer server.Close()

	results, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, testSignatures(25))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if len(results) != 25 {
		t.Fatalf("results = %d, want 25", len(results))
	}
	batches, singles := recorder.snapshot()
	if got := intsCSV(batches); got != "10,5,2,1" {
		t.Fatalf("batch sizes = %s, want 10,5,2,1", got)
	}
	if singles != 25 {
		t.Fatalf("single calls = %d, want 25", singles)
	}
	if count := strings.Count(logs.String(), "rpc batch degraded host="); count != 1 {
		t.Fatalf("degradation log count = %d, want 1; logs=%s", count, logs.String())
	}
}

func TestSolanaGetTransactionsJSONParsedBatchModeCacheStartsSingles(t *testing.T) {
	resetSolanaRPCBatchTestState(t)
	t.Setenv("SOLANA_RPC_BATCH_CHUNK_SIZE", "10")
	recorder := &solanaRPCBatchTestRecorder{}
	server := newSolanaRPCBatchTestServer(t, recorder, func(_ int, single bool) int {
		if single {
			return http.StatusOK
		}
		return http.StatusForbidden
	})
	defer server.Close()

	if _, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, testSignatures(3)); err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	batches, singles := recorder.snapshot()
	if got := intsCSV(batches); got != "3,1" || singles != 3 {
		t.Fatalf("first call batches=%s singles=%d, want batches 3,1 singles 3", got, singles)
	}
	if _, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, testSignatures(4)); err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	batches, singles = recorder.snapshot()
	if got := intsCSV(batches); got != "3,1" {
		t.Fatalf("batch sizes after cached call = %s, want no new 403 batch", got)
	}
	if singles != 7 {
		t.Fatalf("single calls = %d, want 7", singles)
	}
}

func TestSolanaGetTransactionsJSONParsedBatchEmptyAndDuplicateInput(t *testing.T) {
	resetSolanaRPCBatchTestState(t)
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls++
		var requests []solanaRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeSolanaRPCBatchTestResponses(t, w, requests)
	}))
	defer server.Close()

	empty, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{"", "  "})
	if err != nil {
		t.Fatalf("empty request failed: %v", err)
	}
	if len(empty) != 0 || serverCalls != 0 {
		t.Fatalf("empty result len=%d serverCalls=%d, want 0 and 0", len(empty), serverCalls)
	}
	results, err := SolanaGetTransactionsJSONParsedBatch(context.Background(), server.URL, []string{" sig-a ", "sig-a", "sig-b"})
	if err != nil {
		t.Fatalf("duplicate request failed: %v", err)
	}
	if len(results) != 2 || serverCalls != 1 {
		t.Fatalf("duplicate result len=%d serverCalls=%d, want 2 and 1", len(results), serverCalls)
	}
}

func resetSolanaRPCBatchTestState(t *testing.T) {
	t.Helper()
	resetSolanaRPCCachesForTest()
	resetSolanaRPCBatchModeCacheForTest()
	t.Setenv("APP_ENV", "test")
	t.Setenv("SOLANA_RPC_MIN_INTERVAL_MS", "0")
	t.Setenv("SOLANA_RPC_BATCH_MODE", "auto")
}

func newSolanaRPCBatchTestServer(t *testing.T, recorder *solanaRPCBatchTestRecorder, status func(size int, single bool) int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := strings.TrimSpace(readRequestBody(t, r))
		isBatch := strings.HasPrefix(body, "[")
		if isBatch {
			var requests []solanaRPCRequest
			if err := json.Unmarshal([]byte(body), &requests); err != nil {
				t.Fatalf("decode batch request: %v", err)
			}
			recorder.addBatch(len(requests))
			if code := status(len(requests), false); code != http.StatusOK {
				w.WriteHeader(code)
				_, _ = w.Write([]byte(`{"error":"batch rejected"}`))
				return
			}
			writeSolanaRPCBatchTestResponses(t, w, requests)
			return
		}
		recorder.addSingle()
		if code := status(1, true); code != http.StatusOK {
			w.WriteHeader(code)
			return
		}
		var request solanaRPCRequest
		if err := json.Unmarshal([]byte(body), &request); err != nil {
			t.Fatalf("decode single request: %v", err)
		}
		writeSolanaRPCSingleTestResponse(t, w, request)
	}))
}

func readRequestBody(t *testing.T, r *http.Request) string {
	t.Helper()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r.Body); err != nil {
		t.Fatalf("read request body: %v", err)
	}
	return buf.String()
}

func writeSolanaRPCBatchTestResponses(t *testing.T, w http.ResponseWriter, requests []solanaRPCRequest) {
	t.Helper()
	responses := make([]map[string]any, 0, len(requests))
	for _, request := range requests {
		signature := request.Params.([]any)[0].(string)
		responses = append(responses, map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": map[string]any{"signature": signature}})
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(responses); err != nil {
		t.Fatalf("encode batch response: %v", err)
	}
}

func writeSolanaRPCSingleTestResponse(t *testing.T, w http.ResponseWriter, request solanaRPCRequest) {
	t.Helper()
	signature := request.Params.([]any)[0].(string)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": map[string]any{"signature": signature}}); err != nil {
		t.Fatalf("encode single response: %v", err)
	}
}

func testSignatures(count int) []string {
	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, "sig-"+string(rune('a'+i)))
	}
	return out
}

func intsCSV(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ",")
}
