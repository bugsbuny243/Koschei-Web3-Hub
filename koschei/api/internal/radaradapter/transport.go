package radaradapter

import (
	"context"
	"encoding/json"
	"errors"
)

// Request endpoints are operator configuration. User-controlled targets are path/query values only.
type Request struct {
	Network, Provider, Method, Path string
	Origin                          string
	Body                            any
}
type Response struct {
	Raw          []byte
	Host         string
	Certificates []string
}
type Transport interface {
	Do(context.Context, Request) (Response, error)
}
type TransportFunc func(context.Context, Request) (Response, error)

func (f TransportFunc) Do(c context.Context, r Request) (Response, error) { return f(c, r) }
func RPC(ctx context.Context, t Transport, n, p, method string, params any) (Response, json.RawMessage, error) {
	if !ReadOnlyRPCMethod(method) {
		return Response{}, nil, errors.New("RPC method outside read-only evidence contract")
	}
	r, e := t.Do(ctx, Request{Network: n, Provider: p, Method: "POST", Body: map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params}})
	if e != nil {
		return r, nil, e
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if e = Decode(r, &envelope); e != nil {
		return r, nil, e
	}
	if len(envelope.Error) > 0 && string(envelope.Error) != "null" {
		return r, nil, errors.New("provider RPC error")
	}
	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return r, nil, errors.New("provider result missing")
	}
	return r, envelope.Result, nil
}

// Positive allowlist: new methods require an explicit review. No signing,
// broadcasting, wallet unlocking, bundling or simulation with side effects.
func ReadOnlyRPCMethod(method string) bool {
	switch method {
	case "eth_chainId", "eth_getBlockByNumber", "eth_getCode", "eth_getStorageAt", "eth_call", "eth_getLogs", "eth_getTransactionByHash", "eth_getTransactionReceipt", "getNetwork", "getLedgerEntries":
		return true
	}
	return false
}

func ReadOnlyRequest(q Request) bool {
	if q.Method == "GET" {
		return true
	}
	if q.Method != "POST" {
		return false
	}
	if q.Path == "" {
		body, ok := q.Body.(map[string]any)
		if !ok {
			return false
		}
		method, _ := body["method"].(string)
		return ReadOnlyRPCMethod(method)
	}
	switch q.Path {
	case "/wallet/getblockbynum", "/walletsolidity/getnowblock", "/walletsolidity/getaccount", "/wallet/getcontract", "/walletsolidity/getcontract", "/walletsolidity/triggerconstantcontract", "/api/v2/runGetMethod":
		return true
	}
	return false
}
