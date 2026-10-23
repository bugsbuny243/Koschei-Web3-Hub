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

	"koschei/api/internal/outboundhttp"
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

// solanaRPCBatchPartialEvidenceError means an HTTP-successful provider batch
// contained unusable individual responses. Remaining independent chunks may
// still be queried, while the caller must retain an explicit incomplete error.
type solanaRPCBatchPartialEvidenceError struct {
	Summary string
}

func (e *solanaRPCBatchPartialEvidenceError) Error() string {
	if e == nil {
		return "solana rpc batch partial evidence"
	}
	return e.Summary
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
	// Skip deterministic unsupported-version failures already observed for
	// this exact provider and signature. This remains an explicit incomplete
	// evidence error; cached signatures never become verified transactions.
	pending := make([]string, 0, len(clean))
	cachedUnsupported := 0
	for _, signature := range clean {
		if cachedSolanaUnsupportedTransactionVersion(solanaTransactionFetchKey(rpcURL, signature)) {
			cachedUnsupported++
			continue
		}
		pending = append(pending, signature)
	}
	var cachedErr error
	if cachedUnsupported > 0 {
		cachedErr = fmt.Errorf("%d Solana transaction(s) still unsupported: %w", cachedUnsupported, errSolanaUnsupportedTransactionVersion)
	}
	if len(pending) == 0 {
		return map[string]SolanaTransactionResult{}, cachedErr
	}
	clean = pending

	solanaRPCBatchGate.Lock()
	defer solanaRPCBatchGate.Unlock()

	if unavailable, ok := currentSolanaRPCBatchCircuit(rpcURL, time.Now().UTC()); ok {
		return map[string]SolanaTransactionResult{}, &unavailable
	}

	out := map[string]SolanaTransactionResult{}
	incompleteErr := cachedErr
	// Shared across outer chunks so a proven request shape is not forgotten
	// and the invalid-params guard stays bounded for the whole call.
	shape := &solanaSinglesShapeState{}
	batchSize := solanaRPCTransactionBatchSize()
	for start := 0; start < len(clean); start += batchSize {
		if ctx.Err() != nil {
			return out, errors.Join(incompleteErr, ctx.Err())
		}
		end := start + batchSize
		if end > len(clean) {
			end = len(clean)
		}
		chunk, err := solanaGetTransactionsJSONParsedBatchChunk(ctx, rpcURL, clean[start:end], shape)
		mergeSolanaTransactionResults(out, chunk)
		if err != nil {
			var partial *solanaRPCBatchPartialEvidenceError
			if errors.As(err, &partial) {
				// A member-level failure is not an HTTP/provider-wide outage.
				// Continue the bounded independent chunks, but never erase
				// or downgrade the evidence gap from the final report.
				incompleteErr = errors.Join(incompleteErr, err)
				continue
			}
			// Rate limits, provider outages and budget exhaustion are terminal:
			// do not amplify 429 pressure by advancing to later chunks.
			return out, errors.Join(incompleteErr, err)
		}
	}
	return out, incompleteErr
}

func solanaGetTransactionsJSONParsedBatchChunk(ctx context.Context, rpcURL string, clean []string, shape *solanaSinglesShapeState) (map[string]SolanaTransactionResult, error) {
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
			results, err := solanaGetTransactionsJSONParsedSingles(ctx, rpcURL, clean[index:], shape)
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
		// Keep successful batch members before reporting a partial provider error.
		// A failed member must not discard independently verified siblings.
		for signature, result := range results {
			out[signature] = result
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
			var partial *solanaRPCBatchPartialEvidenceError
			if errors.As(err, &partial) {
				// Continue collecting independent signatures in this bounded
				// chunk despite a rejected/null/missing member response.
				lastErr = errors.Join(lastErr, err)
				index = end
				continue
			}
			// Only provider-wide faults stop the remaining chunk.
			return out, errors.Join(lastErr, err)
		}
		index = end
	}

	if lastErr != nil {
		return out, lastErr
	}
	return out, nil
}

func solanaGetTransactionsJSONParsedBatchRequest(ctx context.Context, rpcURL string, signatures []string) (map[string]SolanaTransactionResult, int, error) {
	requests := make([]solanaRPCRequest, 0, len(signatures))
	for i, signature := range signatures {
		requests = append(requests, solanaRPCRequest{JSONRPC: "2.0", ID: i + 1, Method: "getTransaction", Params: []any{signature, map[string]any{"encoding": "jsonParsed", "commitment": "confirmed", "maxSupportedTransactionVersion": SolanaMaxSupportedTransactionVersion}}})
	}
	payload, err := json.Marshal(requests)
	if err != nil {
		web3.LogRPCFailure("getTransactionBatch", rpcURL, 0, err)
		return nil, 0, err
	}
	maxRetries := solanaRPCMax429Retries()
	for attempt := 0; ; attempt++ {
		if err := reserveSolanaRPCBudgetUnits(ctx, "getTransactionBatch", len(signatures)); err != nil {
			web3.LogRPCFailure("getTransactionBatch", rpcURL, 0, err)
			return nil, 0, err
		}
		if err := waitForSolanaRPCSlot(ctx); err != nil {
			web3.LogRPCFailure("getTransactionBatch", rpcURL, 0, err)
			return nil, 0, err
		}
		req, err := outboundhttp.NewRequest(ctx, http.MethodPost, rpcURL, bytes.NewReader(payload))
		if err != nil {
			web3.LogRPCFailure("getTransactionBatch", rpcURL, 0, err)
			return nil, 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Koschei-RPC-Method", "getTransactionBatch")
		req.Header.Set("X-Koschei-RPC-Adaptive-Batch", "1")
		// Match single-transaction RPC's protected egress boundary. The
		// batch path must reject private destinations, unsafe transports
		// and cross-origin redirects even when a provider URL is misset.
		res, err := outboundhttp.Do(solanaRPCClient, req)
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
				var leftPartial *solanaRPCBatchPartialEvidenceError
				if leftErr != nil && !errors.As(leftErr, &leftPartial) {
					// Provider-wide failures must stop recursive retries,
					// especially throttling and exhausted RPC budgets.
					return left, res.StatusCode, leftErr
				}
				right, _, rightErr := solanaGetTransactionsJSONParsedBatchRequest(ctx, rpcURL, signatures[mid:])
				mergeSolanaTransactionResults(left, right)
				var rightPartial *solanaRPCBatchPartialEvidenceError
				if rightErr != nil && !errors.As(rightErr, &rightPartial) {
					// Retain earlier successful evidence, but do not classify
					// a terminal provider error as safe-to-continue partial.
					if leftErr != nil {
						return left, res.StatusCode, fmt.Errorf("earlier member gap: %v; provider failed: %w", leftErr, rightErr)
					}
					return left, res.StatusCode, rightErr
				}
				// The oversized envelope was successfully split; member-level
				// gaps now carry the returned 200 status so auto mode does not
				// degrade the entire chunk to single-RPC retries.
				return left, http.StatusOK, errors.Join(leftErr, rightErr)
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
		seenIDs := make(map[int]bool, len(signatures))
		duplicateIDs := make(map[int]bool)
		unsupportedErrorIDs := make(map[int]bool)
		memberErrors := 0
		rateLimitedMembers := 0
		requestWideErrorCode := 0
		nullResults := 0
		invalidIDs := 0
		firstErrorCode := 0
		for _, response := range responses {
			if response.ID <= 0 || response.ID > len(signatures) {
				invalidIDs++
				continue
			}
			if seenIDs[response.ID] {
				// Duplicate JSON-RPC IDs make the corresponding evidence
				// ambiguous. Keep independent successful members only.
				duplicateIDs[response.ID] = true
				delete(out, signatures[response.ID-1])
				continue
			}
			seenIDs[response.ID] = true
			if response.Error != nil {
				memberErrors++
				if memberErrors == 1 {
					firstErrorCode = response.Error.Code
				}
				memberErr := fmt.Errorf("solana rpc error %d: %s", response.Error.Code, response.Error.Message)
				if requestWideErrorCode == 0 && solanaRPCBatchRequestWideError(response.Error.Code) {
					requestWideErrorCode = response.Error.Code
				}
				if response.Error.Code == http.StatusTooManyRequests {
					// A provider can rate-limit JSON-RPC members inside HTTP 200.
					// Treat this as global throttling, not a recoverable
					// individual evidence gap that permits more chunks.
					rateLimitedMembers++
				}
				if isSolanaUnsupportedTransactionVersionError(memberErr) {
					// Delay negative caching until ALL envelopes are examined.
					// A duplicate ID (including a conflicting success) is not
					// authoritative evidence that this signature is version 1.
					unsupportedErrorIDs[response.ID] = true
				}
				web3.LogRPCFailure("getTransaction", actualEndpoint, res.StatusCode, memberErr)
				continue
			}
			if response.Result == nil {
				// getTransaction legitimately returns JSON null for unknown or
				// unavailable history; it does NOT establish verified evidence.
				nullResults++
				continue
			}
			out[signatures[response.ID-1]] = response.Result
		}
		for id := range unsupportedErrorIDs {
			if !duplicateIDs[id] {
				cacheSolanaUnsupportedTransactionVersion(solanaTransactionFetchKey(rpcURL, signatures[id-1]))
			}
		}
		missingResponses := len(signatures) - len(seenIDs)
		if rateLimitedMembers > 0 {
			delay := solanaRPC429Cooldown()
			deferSolanaRPCRequests(delay)
			web3.DeferSolanaRPCProvider(actualEndpoint, delay)
			// Other successful signatures remain valid partial evidence, but
			// additional chunks MUST NOT amplify the provider's rate limit.
			return out, res.StatusCode, fmt.Errorf(
				"solana rpc batch provider rate limited: %d/%d member(s) returned json-rpc code 429; first_rpc_code=%d",
				rateLimitedMembers, len(signatures), firstErrorCode,
			)
		}
		if requestWideErrorCode != 0 {
			// Method/parameter failures and node-wide unavailability apply
			// to the request shape, not just one signature. Preserve valid
			// sibling evidence but stop all later chunks to avoid a storm.
			return out, res.StatusCode, fmt.Errorf("solana rpc batch request-wide failure: code=%d", requestWideErrorCode)
		}
		if memberErrors > 0 || nullResults > 0 || missingResponses > 0 || len(duplicateIDs) > 0 || invalidIDs > 0 {
			// HTTP 200 does not guarantee that every batch member responded.
			// Never silently treat omitted, duplicated or invalid JSON-RPC IDs
			// as complete evidence; preserve only unambiguous successful rows.
			return out, res.StatusCode, &solanaRPCBatchPartialEvidenceError{Summary: fmt.Sprintf(
				"solana rpc batch partial evidence: %d/%d transaction request(s) failed; null_results=%d missing_responses=%d duplicate_ids=%d invalid_ids=%d first_rpc_code=%d",
				memberErrors, len(signatures), nullResults, missingResponses, len(duplicateIDs), invalidIDs, firstErrorCode,
			)}
		}
		return out, res.StatusCode, nil
	}
}

// solanaRPCBatchRequestWideError identifies faults that cannot be fixed by
// changing the signature in a later request with the same method and options.
// Signature-specific unsupported transaction versions (-32015) remain partial.
func solanaRPCBatchRequestWideError(code int) bool {
	switch code {
	case -32600, -32601, -32602, -32603, -32005:
		return true
	default:
		return false
	}
}

func isSolanaInvalidParamsError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "solana rpc error -32602:")
}

// solanaSinglesShapeState records whether single getTransaction requests have
// been answered during one top-level batch call.
type solanaSinglesShapeState struct {
	answered              int
	unprovenInvalidParams int
}

func solanaGetTransactionsJSONParsedSingles(ctx context.Context, rpcURL string, signatures []string, shape *solanaSinglesShapeState) (map[string]SolanaTransactionResult, error) {
	out := map[string]SolanaTransactionResult{}
	var partialErr error
	for _, signature := range signatures {
		result, err := SolanaGetTransactionJSONParsed(ctx, rpcURL, signature)
		if err != nil {
			if isSolanaUnsupportedTransactionVersionError(err) {
				// This signature alone lacks supported evidence. Keep querying
				// independent signatures and later outer chunks.
				partialErr = errors.Join(partialErr, err)
				continue
			}
			// A one-signature request rejected as invalid params points at that
			// signature (e.g. a malformed externally sourced value) once the
			// request shape is proven by an answered request. Before that, a
			// second consecutive rejection is treated as a request-shape fault.
			if isSolanaInvalidParamsError(err) && (shape.answered > 0 || shape.unprovenInvalidParams == 0) {
				if shape.answered == 0 {
					shape.unprovenInvalidParams++
				}
				partialErr = errors.Join(partialErr, err)
				continue
			}
			// Provider-wide failures, including 429, must stop requests.
			return out, errors.Join(partialErr, err)
		}
		// Any answer, including null history, proves the request shape works.
		shape.answered++
		if result != nil {
			out[signature] = result
		} else {
			// A nil result is unavailable history, never verified evidence.
			partialErr = errors.Join(partialErr, fmt.Errorf("solana rpc single-transaction evidence unavailable"))
		}
	}
	if partialErr != nil {
		return out, &solanaRPCBatchPartialEvidenceError{Summary: fmt.Sprintf("solana rpc single-transaction partial evidence: %v", partialErr)}
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
