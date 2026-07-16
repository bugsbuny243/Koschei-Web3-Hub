package services

import (
	"bytes"
	"context"
	"encoding/json"
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

type solanaRPCBatchModeEntry struct {
	Mode      string
	ExpiresAt time.Time
}

var solanaRPCBatchModeCache = struct {
	sync.Mutex
	Items map[string]solanaRPCBatchModeEntry
}{Items: map[string]solanaRPCBatchModeEntry{}}

const solanaRPCBatchModeTTL = 15 * time.Minute

// SolanaGetTransactionsJSONParsedBatch groups getTransaction requests into
// adaptive HTTP/JSON-RPC batches. A missing or failed member is omitted from
// the returned map; callers preserve partial evidence instead of failing the
// full scan.
func SolanaGetTransactionsJSONParsedBatch(ctx context.Context, rpcURL string, signatures []string) (map[string]SolanaTransactionResult, error) {
	rpcURL = strings.TrimSpace(rpcURL)
	if rpcURL == "" {
		return nil, fmt.Errorf("solana rpc url is empty")
	}
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
		results, status, err := solanaGetTransactionsJSONParsedBatchChunk(ctx, rpcURL, chunk)
		if err != nil {
			lastErr = err
		}
		if status == http.StatusForbidden || status == http.StatusRequestEntityTooLarge {
			if chunkSize > 1 && modeOverride != "batch" {
				nextChunkSize := len(chunk) / 2
				if nextChunkSize < 1 {
					nextChunkSize = 1
				}
				if !degradationLogged {
					log.Printf("rpc batch degraded host=%s reason=%d chunk=%d→%d", host, status, chunkSize, nextChunkSize)
					degradationLogged = true
				}
				chunkSize = nextChunkSize
				continue
			}
			if modeOverride != "batch" {
				if !degradationLogged {
					log.Printf("rpc batch degraded host=%s reason=%d chunk=%d→single", host, status, chunkSize)
					degradationLogged = true
				}
				solanaRPCBatchRememberMode(host, "single")
				degradedToSingles = true
				continue
			}
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

func solanaGetTransactionsJSONParsedBatchChunk(ctx context.Context, rpcURL string, signatures []string) (map[string]SolanaTransactionResult, int, error) {
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
			return nil, res.StatusCode, fmt.Errorf("solana rpc batch http status %d: %s", res.StatusCode, string(body))
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, res.StatusCode, fmt.Errorf("solana rpc batch http status %d: %s", res.StatusCode, string(body))
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
				web3.LogRPCFailure("getTransaction", actualEndpoint, res.StatusCode, fmt.Errorf("rpc result unavailable"))
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
