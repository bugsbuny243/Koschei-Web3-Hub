package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type SolanaSignatureInfo struct {
	Signature string `json:"signature"`
	Slot      int64  `json:"slot"`
	Err       any    `json:"err"`
	BlockTime *int64 `json:"blockTime"`
}

type solanaRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type solanaRPCResponse[T any] struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  T               `json:"result"`
	Error   *solanaRPCError `json:"error"`
}

type solanaRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func SolanaGetSignaturesForAddress(ctx context.Context, rpcURL string, address string, limit int) ([]SolanaSignatureInfo, error) {
	rpcURL = strings.TrimSpace(rpcURL)
	address = strings.TrimSpace(address)
	if rpcURL == "" {
		return nil, fmt.Errorf("solana rpc url is empty")
	}
	if address == "" {
		return nil, fmt.Errorf("solana address is empty")
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	return solanaRPCDo[[]SolanaSignatureInfo](ctx, rpcURL, "getSignaturesForAddress", []any{address, map[string]any{"limit": limit}})
}

func solanaRPCDo[T any](ctx context.Context, rpcURL, method string, params any) (T, error) {
	var zero T
	payload, err := json.Marshal(solanaRPCRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return zero, err
	}
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(payload))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return zero, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return zero, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return zero, fmt.Errorf("solana rpc http status %d: %s", res.StatusCode, string(body))
	}
	var out solanaRPCResponse[T]
	if err := json.Unmarshal(body, &out); err != nil {
		return zero, fmt.Errorf("solana rpc malformed response: %w", err)
	}
	if out.Error != nil {
		return zero, fmt.Errorf("solana rpc error %d: %s", out.Error.Code, out.Error.Message)
	}
	return out.Result, nil
}
