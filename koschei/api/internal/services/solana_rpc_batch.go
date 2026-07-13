package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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

// SolanaGetTransactionsJSONParsedBatch groups getTransaction requests into one
// HTTP/JSON-RPC batch. A missing or failed member is omitted from the returned
// map; callers preserve partial evidence instead of failing the full scan.
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
	requests := make([]solanaRPCRequest, 0, len(clean))
	for i, signature := range clean {
		requests = append(requests, solanaRPCRequest{
			JSONRPC: "2.0", ID: i + 1, Method: "getTransaction",
			Params: []any{signature, map[string]any{"encoding": "jsonParsed", "commitment": "confirmed", "maxSupportedTransactionVersion": 0}},
		})
	}
	payload, err := json.Marshal(requests)
	if err != nil {
		web3.LogRPCFailure("getTransactionBatch", rpcURL, 0, err)
		return nil, err
	}
	maxRetries := solanaRPCMax429Retries()
	for attempt := 0; ; attempt++ {
		if err := reserveSolanaRPCBudget(ctx, "getTransactionBatch"); err != nil {
			web3.LogRPCFailure("getTransactionBatch", rpcURL, 0, err)
			return nil, err
		}
		if err := waitForSolanaRPCSlot(ctx); err != nil {
			web3.LogRPCFailure("getTransactionBatch", rpcURL, 0, err)
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(payload))
		if err != nil {
			web3.LogRPCFailure("getTransactionBatch", rpcURL, 0, err)
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Koschei-RPC-Method", "getTransactionBatch")
		res, err := solanaRPCClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(res.Body, 32<<20))
		res.Body.Close()
		actualEndpoint := rpcURL
		if res.Request != nil && res.Request.URL != nil {
			actualEndpoint = res.Request.URL.String()
		}
		if readErr != nil {
			web3.LogRPCFailure("getTransactionBatch", actualEndpoint, res.StatusCode, readErr)
			return nil, readErr
		}
		if res.StatusCode == http.StatusTooManyRequests {
			delay := maxDuration(solanaRPC429Delay(attempt, res.Header.Get("Retry-After")), solanaRPC429Cooldown())
			deferSolanaRPCRequests(delay)
			if attempt < maxRetries {
				continue
			}
			return nil, fmt.Errorf("solana rpc batch http status %d: %s", res.StatusCode, string(body))
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, fmt.Errorf("solana rpc batch http status %d: %s", res.StatusCode, string(body))
		}
		var responses []solanaRPCBatchTransactionResponse
		if err := json.Unmarshal(body, &responses); err != nil {
			wrapped := fmt.Errorf("solana rpc malformed batch response: %w", err)
			web3.LogRPCFailure("getTransactionBatch", actualEndpoint, res.StatusCode, wrapped)
			return nil, wrapped
		}
		out := map[string]SolanaTransactionResult{}
		for _, response := range responses {
			if response.ID <= 0 || response.ID > len(clean) {
				continue
			}
			if response.Error != nil {
				web3.LogRPCFailure("getTransaction", actualEndpoint, res.StatusCode,
					fmt.Errorf("solana rpc error %d: %s", response.Error.Code, response.Error.Message))
				continue
			}
			if response.Result == nil {
				web3.LogRPCFailure("getTransaction", actualEndpoint, res.StatusCode, fmt.Errorf("rpc result unavailable"))
				continue
			}
			out[clean[response.ID-1]] = response.Result
		}
		return out, nil
	}
}
