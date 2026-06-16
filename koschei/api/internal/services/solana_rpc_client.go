package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type SolanaSignatureInfo struct {
	Signature string `json:"signature"`
	Slot      int64  `json:"slot"`
	Err       any    `json:"err"`
	BlockTime *int64 `json:"blockTime"`
}

type SolanaTokenAmount struct {
	Amount         string   `json:"amount"`
	Decimals       int      `json:"decimals"`
	UIAmount       *float64 `json:"uiAmount"`
	UIAmountString string   `json:"uiAmountString"`
}

type SolanaTokenSupplyResult struct {
	Value SolanaTokenAmount `json:"value"`
}

type SolanaLargestTokenAccount struct {
	Address string `json:"address"`
	SolanaTokenAmount
}

type SolanaLargestAccountsResult struct {
	Value []SolanaLargestTokenAccount `json:"value"`
}

type SolanaAccountInfoResult struct {
	Value *SolanaAccountInfo `json:"value"`
}

type SolanaAccountInfo struct {
	Data       any    `json:"data"`
	Executable bool   `json:"executable"`
	Lamports   int64  `json:"lamports"`
	Owner      string `json:"owner"`
	RentEpoch  any    `json:"rentEpoch"`
	Space      int64  `json:"space"`
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
	if limit <= 0 || limit > 1000 {
		limit = 10
	}
	return solanaRPCDo[[]SolanaSignatureInfo](ctx, rpcURL, "getSignaturesForAddress", []any{address, map[string]any{"limit": limit}})
}

func SolanaGetTokenSupply(ctx context.Context, rpcURL string, mint string) (SolanaTokenSupplyResult, error) {
	return solanaRPCDo[SolanaTokenSupplyResult](ctx, strings.TrimSpace(rpcURL), "getTokenSupply", []any{strings.TrimSpace(mint)})
}

func SolanaGetTokenLargestAccounts(ctx context.Context, rpcURL string, mint string) (SolanaLargestAccountsResult, error) {
	return solanaRPCDo[SolanaLargestAccountsResult](ctx, strings.TrimSpace(rpcURL), "getTokenLargestAccounts", []any{strings.TrimSpace(mint)})
}

func SolanaGetAccountInfoJSONParsed(ctx context.Context, rpcURL string, address string) (SolanaAccountInfoResult, error) {
	return solanaRPCDo[SolanaAccountInfoResult](ctx, strings.TrimSpace(rpcURL), "getAccountInfo", []any{strings.TrimSpace(address), map[string]any{"encoding": "jsonParsed"}})
}

func solanaRPCDo[T any](ctx context.Context, rpcURL, method string, params any) (T, error) {
	var zero T
	rpcURL = strings.TrimSpace(rpcURL)
	if rpcURL == "" {
		return zero, fmt.Errorf("solana rpc url is empty")
	}
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

func solanaTokenFloat(amount SolanaTokenAmount) float64 {
	if amount.UIAmount != nil {
		return *amount.UIAmount
	}
	if strings.TrimSpace(amount.UIAmountString) != "" {
		if v, err := strconv.ParseFloat(strings.TrimSpace(amount.UIAmountString), 64); err == nil {
			return v
		}
	}
	if strings.TrimSpace(amount.Amount) == "" {
		return 0
	}
	raw, err := strconv.ParseFloat(strings.TrimSpace(amount.Amount), 64)
	if err != nil {
		return 0
	}
	if amount.Decimals <= 0 {
		return raw
	}
	divisor := 1.0
	for i := 0; i < amount.Decimals; i++ {
		divisor *= 10
	}
	if divisor <= 0 {
		return raw
	}
	return raw / divisor
}
