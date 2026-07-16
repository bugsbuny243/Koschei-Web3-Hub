package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"koschei/api/internal/web3"
)

func SolanaGetSignaturesForAddressBefore(ctx context.Context, rpcURL, address string, limit int, before string) ([]SolanaSignatureInfo, error) {
	rpcURL = strings.TrimSpace(rpcURL)
	address = strings.TrimSpace(address)
	if rpcURL == "" {
		return nil, fmt.Errorf("solana rpc url is empty")
	}
	if address == "" {
		return nil, fmt.Errorf("solana address is empty")
	}
	if limit <= 0 || limit > 1000 {
		limit = 50
	}
	options := map[string]any{"limit": limit}
	if before = strings.TrimSpace(before); before != "" {
		options["before"] = before
	}
	return solanaRPCDo[[]SolanaSignatureInfo](ctx, rpcURL, "getSignaturesForAddress", []any{address, options})
}

type solanaRPCBatchTransactionResponse struct {
	JSONRPC string                  `json:"jsonrpc"`
	ID      int                     `json:"id"`
	Result  SolanaTransactionResult `json:"result"`
	Error   *solanaRPCError         `json:"error"`
}

type solanaRPCBatchUnavailableError struct {
	Provider   string
	StatusCode int
	Reason     string
	RetryAt    time.Time
}

func (e *solanaRPCBatchUnavailableError) Error() string {
	if e == nil {
		return "solana rpc batch unavailable"
	}
	message := "solana rpc batch unavailable"
	if e.Provider != "" {
		message += " for " + e.Provider
	}
	if e.StatusCode > 0 {
		message += fmt.Sprintf(" (http %d)", e.StatusCode)
	}
	if e.Reason != "" {
		message += ": " + e.Reason
	}
	if !e.RetryAt.IsZero() {
		message += "; retry after " + e.RetryAt.UTC().Format(time.RFC3339)
	}
	return message
}

func IsSolanaRPCBatchUnavailable(err error) bool {
	var target *solanaRPCBatchUnavailableError
	return errors.As(err, &target)
}

var solanaRPCBatchCircuit = struct {
	sync.Mutex
	Disabled map[string]solanaRPCBatchUnavailableError
}{Disabled: map[string]solanaRPCBatchUnavailableError{}}

// A single process-wide gate prevents several ATA workers from discovering the
// same provider rejection simultaneously. Once the first 403/413 trips the
// circuit, queued workers degrade locally without sending another HTTP request.
var solanaRPCBatchGate sync.Mutex

type solanaRPCBatchModeEntry struct {
	Mode      string
	ExpiresAt time.Time
}

const solanaRPCBatchModeTTL = 15 * time.Minute

var solanaRPCBatchModeCache = struct {
	sync.Mutex
	Items map[string]solanaRPCBatchModeEntry
}{Items: map[string]solanaRPCBatchModeEntry{}}

// SolanaGetTransactionsJSONParsedBatch groups getTransaction requests into
// bounded HTTP/JSON-RPC batches. Missing members and successful chunks are
// preserved. Providers that reject batch traffic are circuit-broken so one
// manual investigation cannot turn into a retry storm.
func SolanaGetTransactionsJSONParsedBatch(ctx context.Context, rpcURL string, signatures []string) (map[string]SolanaTransactionResult, error) {
	rpcURL = strings.TrimSpace(rpcURL)
	if rpcURL == "" {
		return nil, fmt.Errorf("solana rpc url is empty")
	}
	clean := cleanSolanaBatchSignatures(signatures)
	if len(clean) == 0 {
		return map[string]SolanaTransactionResult{}, nil
	}
	if !solanaRPCBatchEnabled() {
		return map[string]SolanaTransactionResult{}, &solanaRPCBatchUnavailableError{
			Provider: solanaRPCBatchProviderKey(rpcURL), Reason: "disabled by SOLANA_RPC_BATCH_ENABLED",
		}
	}

	solanaRPCBatchGate.Lock()
	defer solanaRPCBatchGate.Unlock()

	if unavailable, ok := currentSolanaRPCBatchCircuit(rpcURL, time.Now().UTC()); ok {
		return map[string]SolanaTransactionResult{}, &unavailable
	}

	out := map[string]SolanaTransactionResult{}
	batchSize := solanaRPCTransactionBatchSize()
	for start := 0; start < len(clean); start += batchSize {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		end := start + batchSize
		if end > len(clean) {
			end = len(clean)
		}
		chunk, err := solanaGetTransactionsJSONParsedBatchChunk(ctx, rpcURL, clean[start:end])
		mergeSolanaTransactionResults(out, chunk)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func solanaGetTransactionsJSONParsedBatchChunk(ctx context.Context, rpcURL string, clean []string) (map[string]SolanaTransactionResult, error) {
	if len(clean) == 0 {
		return map[string]SolanaTransactionResult{}, nil
	}

	host := web3.RPCProviderHost(rpcURL)
	modeOverride := solanaRPCBatchModeOverride()
	degradedToSingles := modeOverride == "single" || (modeOverride == "auto" && solanaRPCBatchCachedMode(host) == "single")
	chunkSize := solanaRPCBatchChunkSize()
	out := map[string]SolanaTransactionResult{}
	var lastErr error
	degradationLogged := false

	for index := 0; index < len(clean); {
		if degradedToSingles {
			results, err := solanaGetTransactionsJSONParsedSingles(ctx, rpcURL, clean[index:])
			for signature, result := range results {
				out[signature] = result
			}
			if err != nil {
				lastErr = err
			}
			break
		}

		end := index + chunkSize
		if end > len(clean) {
			end = len(clean)
		}
		chunk := clean[index:end]
		results, status, err := solanaGetTransactionsJSONParsedBatchRequest(ctx, rpcURL, chunk)
		if err != nil {
			lastErr = err
		}
		if status == http.StatusRequestEntityTooLarge && err != nil && modeOverride != "batch" {
			if !degradationLogged {
				log.Printf("rpc batch degraded host=%s reason=%d chunk=%d→single", host, status, chunkSize)
				degradationLogged = true
			}
			solanaRPCBatchRememberMode(host, "single")
			degradedToSingles = true
			continue
		}
		if err != nil {
			if len(out) == 0 {
				return nil, err
			}
			return out, nil
		}
		for signature, result := range results {
			out[signature] = result
		}
		index = end
	}

	if len(out) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return out, nil
}

func solanaGetTransactionsJSONParsedBatchRequest(ctx context.Context, rpcURL string, signatures []string) (map[string]SolanaTransactionResult, int, error) {
	requests := make([]solanaRPCRequest, 0, len(signatures))
	for i, signature := range signatures {
		requests = append(requests, solanaRPCRequest{JSONRPC: "2.0", ID: i + 1, Method: "getTransaction", Params: []any{signature, map[string]any{"encoding": "jsonParsed", "commitment": "confirmed", "maxSupportedTransactionVersion": 0}}})
	}
	payload, err := json.Marshal(requests)
	if err != nil {
		web3.LogRPCFailure("getTransactionBatch", rpcURL, 0, err)
		return nil, 0, err
	}
	maxRetries := solanaRPCMax429Retries()
	for attempt := 0; ; attempt++ {
		if err := reserveSolanaRPCBudget(ctx, "getTransactionBatch"); err != nil {
			web3.LogRPCFailure("getTransactionBatch", rpcURL, 0, err)
			return nil, 0, err
		}
		if err := waitForSolanaRPCSlot(ctx); err != nil {
			web3.LogRPCFailure("getTransactionBatch", rpcURL, 0, err)
			return nil, 0, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(payload))
		if err != nil {
			web3.LogRPCFailure("getTransactionBatch", rpcURL, 0, err)
			return nil, 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Koschei-RPC-Method", "getTransactionBatch")
		req.Header.Set("X-Koschei-RPC-Adaptive-Batch", "1")
		res, err := solanaRPCClient.Do(req)
		if err != nil {
			return nil, 0, err
		}
		body, readErr := io.ReadAll(io.LimitReader(res.Body, 32<<20))
		res.Body.Close()
		actualEndpoint := rpcURL
		if res.Request != nil && res.Request.URL != nil {
			actualEndpoint = res.Request.URL.String()
		}
		if readErr != nil {
			web3.LogRPCFailure("getTransactionBatch", actualEndpoint, res.StatusCode, readErr)
			return nil, res.StatusCode, readErr
		}
		if res.StatusCode == http.StatusTooManyRequests {
			delay := maxDuration(solanaRPC429Delay(attempt, res.Header.Get("Retry-After")), solanaRPC429Cooldown())
			deferSolanaRPCRequests(delay)
			if attempt < maxRetries {
				continue
			}
			return nil, res.StatusCode, fmt.Errorf("solana rpc batch http status %d: %s", res.StatusCode, compactSolanaBatchBody(body))
		}
		if res.StatusCode == http.StatusRequestEntityTooLarge {
			// Some providers accept JSON-RPC batches but enforce a smaller member
			// count. Split once recursively; a one-member 413 means batch mode is
			// unusable and trips the provider circuit.
			if len(signatures) > 1 {
				mid := len(signatures) / 2
				left, _, leftErr := solanaGetTransactionsJSONParsedBatchRequest(ctx, rpcURL, signatures[:mid])
				right := map[string]SolanaTransactionResult{}
				var rightErr error
				if leftErr == nil {
					right, _, rightErr = solanaGetTransactionsJSONParsedBatchRequest(ctx, rpcURL, signatures[mid:])
				}
				mergeSolanaTransactionResults(left, right)
				if leftErr != nil {
					return left, res.StatusCode, leftErr
				}
				return left, res.StatusCode, rightErr
			}
			return nil, res.StatusCode, tripSolanaRPCBatchCircuit(rpcURL, res.StatusCode, "provider rejected a single-member batch")
		}
		if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusMethodNotAllowed || res.StatusCode == http.StatusNotImplemented {
			return nil, res.StatusCode, tripSolanaRPCBatchCircuit(rpcURL, res.StatusCode, "provider does not permit JSON-RPC batch requests")
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, res.StatusCode, fmt.Errorf("solana rpc batch http status %d: %s", res.StatusCode, compactSolanaBatchBody(body))
		}
		var responses []solanaRPCBatchTransactionResponse
		if err := json.Unmarshal(body, &responses); err != nil {
			wrapped := fmt.Errorf("solana rpc malformed batch response: %w", err)
			web3.LogRPCFailure("getTransactionBatch", actualEndpoint, res.StatusCode, wrapped)
			return nil, res.StatusCode, wrapped
		}
		out := map[string]SolanaTransactionResult{}
		for _, response := range responses {
			if response.ID <= 0 || response.ID > len(signatures) {
				continue
			}
			if response.Error != nil {
				web3.LogRPCFailure("getTransaction", actualEndpoint, res.StatusCode, fmt.Errorf("solana rpc error %d: %s", response.Error.Code, response.Error.Message))
				continue
			}
			if response.Result == nil {
				continue
			}
			out[signatures[response.ID-1]] = response.Result
		}
		return out, res.StatusCode, nil
	}
}

func solanaGetTransactionsJSONParsedSingles(ctx context.Context, rpcURL string, signatures []string) (map[string]SolanaTransactionResult, error) {
	out := map[string]SolanaTransactionResult{}
	var lastErr error
	for _, signature := range signatures {
		result, err := SolanaGetTransactionJSONParsed(ctx, rpcURL, signature)
		if err != nil {
			lastErr = err
			continue
		}
		if result != nil {
			out[signature] = result
		}
	}
	if len(out) == 0 && lastErr != nil {
		return out, lastErr
	}
	return out, nil
}

func solanaRPCBatchChunkSize() int {
	value := 10
	if raw := strings.TrimSpace(os.Getenv("SOLANA_RPC_BATCH_CHUNK_SIZE")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			value = parsed
		}
	}
	if value < 1 {
		return 1
	}
	if value > 100 {
		return 100
	}
	return value
}

func solanaRPCBatchModeOverride() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SOLANA_RPC_BATCH_MODE"))) {
	case "batch", "single":
		return strings.ToLower(strings.TrimSpace(os.Getenv("SOLANA_RPC_BATCH_MODE")))
	default:
		return "auto"
	}
}

func solanaRPCBatchCachedMode(host string) string {
	now := time.Now()
	solanaRPCBatchModeCache.Lock()
	defer solanaRPCBatchModeCache.Unlock()
	entry, ok := solanaRPCBatchModeCache.Items[host]
	if !ok || !entry.ExpiresAt.After(now) {
		delete(solanaRPCBatchModeCache.Items, host)
		return ""
	}
	return entry.Mode
}

func solanaRPCBatchRememberMode(host, mode string) {
	solanaRPCBatchModeCache.Lock()
	solanaRPCBatchModeCache.Items[host] = solanaRPCBatchModeEntry{Mode: mode, ExpiresAt: time.Now().Add(solanaRPCBatchModeTTL)}
	solanaRPCBatchModeCache.Unlock()
}

func resetSolanaRPCBatchModeCacheForTest() {
	solanaRPCBatchModeCache.Lock()
	solanaRPCBatchModeCache.Items = map[string]solanaRPCBatchModeEntry{}
	solanaRPCBatchModeCache.Unlock()
}

func cleanSolanaBatchSignatures(signatures []string) []string {
	clean := make([]string, 0, len(signatures))
	seen := map[string]bool{}
	for _, signature := range signatures {
		signature = strings.TrimSpace(signature)
		if signature == "" || seen[signature] {
			continue
		}
		seen[signature] = true
		clean = append(clean, signature)
	}
	return clean
}

func mergeSolanaTransactionResults(dst, src map[string]SolanaTransactionResult) {
	if dst == nil {
		return
	}
	for signature, transaction := range src {
		dst[signature] = transaction
	}
}

func solanaRPCBatchProviderKey(rpcURL string) string {
	if host := strings.TrimSpace(web3.RPCProviderHost(rpcURL)); host != "" {
		return host
	}
	return strings.TrimSpace(rpcURL)
}

func currentSolanaRPCBatchCircuit(rpcURL string, now time.Time) (solanaRPCBatchUnavailableError, bool) {
	key := solanaRPCBatchProviderKey(rpcURL)
	solanaRPCBatchCircuit.Lock()
	defer solanaRPCBatchCircuit.Unlock()
	entry, ok := solanaRPCBatchCircuit.Disabled[key]
	if !ok {
		return solanaRPCBatchUnavailableError{}, false
	}
	if !entry.RetryAt.IsZero() && !entry.RetryAt.After(now) {
		delete(solanaRPCBatchCircuit.Disabled, key)
		return solanaRPCBatchUnavailableError{}, false
	}
	return entry, true
}

func tripSolanaRPCBatchCircuit(rpcURL string, statusCode int, reason string) error {
	key := solanaRPCBatchProviderKey(rpcURL)
	entry := solanaRPCBatchUnavailableError{
		Provider: key, StatusCode: statusCode, Reason: reason,
		RetryAt: time.Now().UTC().Add(solanaRPCBatchCooldown()),
	}
	solanaRPCBatchCircuit.Lock()
	solanaRPCBatchCircuit.Disabled[key] = entry
	solanaRPCBatchCircuit.Unlock()
	return &entry
}

func solanaRPCTransactionBatchSize() int {
	return solanaRPCBatchEnvInt("SOLANA_RPC_TX_BATCH_SIZE", 8, 1, 20)
}

func solanaRPCBatchCooldown() time.Duration {
	seconds := solanaRPCBatchEnvInt("SOLANA_RPC_BATCH_COOLDOWN_SECONDS", 600, 30, 3600)
	return time.Duration(seconds) * time.Second
}

func solanaRPCBatchEnabled() bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("SOLANA_RPC_BATCH_ENABLED")))
	return raw != "false" && raw != "0" && raw != "off" && raw != "disabled"
}

func solanaRPCBatchEnvInt(name string, fallback, min, max int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return fallback
	}
	return value
}

func compactSolanaBatchBody(body []byte) string {
	value := strings.Join(strings.Fields(string(body)), " ")
	if len(value) > 240 {
		value = value[:240]
	}
	return value
}

func resetSolanaRPCBatchCircuitForTest() {
	solanaRPCBatchCircuit.Lock()
	solanaRPCBatchCircuit.Disabled = map[string]solanaRPCBatchUnavailableError{}
	solanaRPCBatchCircuit.Unlock()
}
