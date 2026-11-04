package evm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Explicit synthetic fixtures: addresses and hashes are repeated bytes and do
// not refer to real wallets or transactions.
func fixtureAddress(fill byte) string { return "0x" + strings.Repeat(fmt.Sprintf("%02x", fill), 20) }
func fixtureHash(fill byte) string    { return "0x" + strings.Repeat(fmt.Sprintf("%02x", fill), 32) }
func fixtureTopic(address string) string {
	return "0x" + strings.Repeat("0", 24) + strings.TrimPrefix(address, "0x")
}
func fixtureWord(value *big.Int) string { return fmt.Sprintf("0x%064x", value) }

func fixtureLog(token string, from, to string, value int64, block, index uint64, extra map[string]any) map[string]any {
	entry := map[string]any{
		"address": token, "topics": []string{TransferTopic, fixtureTopic(from), fixtureTopic(to)}, "data": fixtureWord(big.NewInt(value)),
		"blockNumber": fmt.Sprintf("0x%x", block), "blockHash": fixtureHash(0xbb), "transactionHash": fixtureHash(byte(index + 1)),
		"logIndex": fmt.Sprintf("0x%x", index), "removed": false,
	}
	for k, v := range extra {
		entry[k] = v
	}
	return entry
}

func TestFlowABIConstantsMatchKnownSelectors(t *testing.T) {
	if TransferTopic != "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef" {
		t.Fatalf("Transfer topic = %s", TransferTopic)
	}
	for signature, want := range map[string]string{"balanceOf(address)": "0x70a08231", "isBlackListed(address)": "0xe47d6060", "isBlacklisted(address)": "0xfe575a87"} {
		if got := selector(signature); got != want {
			t.Fatalf("selector(%s) = %s, want %s", signature, got, want)
		}
	}
}

func TestFlowTokensAreCanonicalAndFreezeMetadataIsConsistent(t *testing.T) {
	for network, tokens := range FlowTokens {
		if _, ok := Chains[network]; !ok {
			t.Fatalf("%s is not a known EVM chain", network)
		}
		for _, token := range tokens {
			canonical, err := Address(token.Address)
			if err != nil || canonical != token.Address {
				t.Fatalf("%s token %s must be a lower-case canonical address", network, token.Symbol)
			}
			if token.Decimals != 6 && token.Decimals != 18 {
				t.Fatalf("%s token %s has unexpected decimals %d", network, token.Symbol, token.Decimals)
			}
			hasFreeze := token.FreezeMethod != ""
			if hasFreeze != (token.Issuer != "") || hasFreeze != (token.IssuerOption != "") {
				t.Fatalf("%s token %s must set issuer, issuer option and freeze method together", network, token.Symbol)
			}
		}
	}
	if FlowTokens["bnb-mainnet"][0].FreezeMethod != "" {
		t.Fatal("Binance-Peg tokens have no issuer freeze function and must not claim one")
	}
}

func TestParseTransferLogsDecodesSortsAndSkipsNonERC20(t *testing.T) {
	token, other := fixtureAddress(0xaa), fixtureAddress(0xcc)
	from, to := fixtureAddress(0x11), fixtureAddress(0x22)
	logs := []map[string]any{
		fixtureLog(token, from, to, 7_000_000, 101, 3, nil),
		fixtureLog(token, to, from, 5, 100, 9, nil),
		fixtureLog(token, from, to, 1, 100, 1, map[string]any{"removed": true}),
		fixtureLog(other, from, to, 1, 100, 2, nil),
		fixtureLog(token, from, to, 1, 100, 4, map[string]any{"topics": []string{TransferTopic, fixtureTopic(from), fixtureTopic(to), fixtureWord(big.NewInt(1))}}),
	}
	raw, _ := json.Marshal(logs)
	transfers, err := ParseTransferLogs(raw, map[string]struct{}{token: {}})
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 2 {
		t.Fatalf("want 2 ERC-20 transfers, got %d", len(transfers))
	}
	if transfers[0].BlockNumber != 100 || transfers[0].LogIndex != 9 || transfers[1].BlockNumber != 101 {
		t.Fatalf("transfers must be ordered by block then log index: %+v", transfers)
	}
	first := transfers[1]
	if first.From != from || first.To != to || first.Token != token || first.Value.Int64() != 7_000_000 || first.EventID() != fixtureHash(4)+":3" {
		t.Fatalf("decoded transfer = %+v", first)
	}
}

func TestParseTransferLogsRejectsMalformedERC20Log(t *testing.T) {
	token := fixtureAddress(0xaa)
	raw, _ := json.Marshal([]map[string]any{fixtureLog(token, fixtureAddress(1), fixtureAddress(2), 1, 1, 0, map[string]any{"data": "0x01"})})
	if _, err := ParseTransferLogs(raw, map[string]struct{}{token: {}}); err == nil {
		t.Fatal("a malformed Transfer log must fail the batch")
	}
	dirty := fixtureLog(token, fixtureAddress(1), fixtureAddress(2), 1, 1, 0, nil)
	dirty["topics"] = []string{TransferTopic, "0x" + strings.Repeat("f", 64), fixtureTopic(fixtureAddress(2))}
	raw, _ = json.Marshal([]map[string]any{dirty})
	if _, err := ParseTransferLogs(raw, map[string]struct{}{token: {}}); err == nil {
		t.Fatal("a non-canonical address topic must fail the batch")
	}
}

type rpcFixture struct {
	t       *testing.T
	handler func(method string, params []json.RawMessage) (int, string)
}

func (f rpcFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var request struct {
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		f.t.Errorf("bad request body: %v", err)
	}
	status, response := f.handler(request.Method, request.Params)
	w.WriteHeader(status)
	_, _ = io.WriteString(w, response)
}

func TestFlowClientReadsHeadLogsAndMapsProviderErrors(t *testing.T) {
	token := fixtureAddress(0xaa)
	server := httptest.NewServer(rpcFixture{t: t, handler: func(method string, params []json.RawMessage) (int, string) {
		switch method {
		case "eth_blockNumber":
			return 200, `{"jsonrpc":"2.0","id":1,"result":"0x1b4"}`
		case "eth_getLogs":
			var filter struct {
				FromBlock string   `json:"fromBlock"`
				ToBlock   string   `json:"toBlock"`
				Address   []string `json:"address"`
				Topics    []string `json:"topics"`
			}
			_ = json.Unmarshal(params[0], &filter)
			if len(filter.Address) != 1 || filter.Address[0] != token || len(filter.Topics) != 1 || filter.Topics[0] != TransferTopic {
				t.Errorf("unexpected filter %+v", filter)
			}
			if filter.FromBlock == "0x1" && filter.ToBlock == "0x64" {
				return 200, `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Under the Free tier plan, you can make eth_getLogs requests with up to a 10 block range."}}`
			}
			if filter.FromBlock == "0x2" {
				return 429, `{}`
			}
			logs, _ := json.Marshal([]map[string]any{fixtureLog(token, fixtureAddress(1), fixtureAddress(2), 9, 5, 0, nil)})
			return 200, `{"jsonrpc":"2.0","id":1,"result":` + string(logs) + `}`
		}
		return 200, `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`
	}})
	defer server.Close()
	client := FlowClient{URL: server.URL, HTTP: server.Client()}
	ctx := context.Background()

	if head, err := client.BlockNumber(ctx); err != nil || head != 436 {
		t.Fatalf("head=%d err=%v", head, err)
	}
	if transfers, err := client.TransferLogs(ctx, 5, 5, []string{token}); err != nil || len(transfers) != 1 {
		t.Fatalf("transfers=%v err=%v", transfers, err)
	}
	if _, err := client.TransferLogs(ctx, 1, 100, []string{token}); !errors.Is(err, ErrLogRangeTooLarge) {
		t.Fatalf("free-tier range rejection must map to ErrLogRangeTooLarge, got %v", err)
	}
	_, err := client.TransferLogs(ctx, 2, 3, []string{token})
	if err == nil || err.Error() != "evm_flow_provider_rate_limited" || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("429 must map to a URL-free category, got %v", err)
	}
	if _, err := client.TransferLogs(ctx, 6, 7, []string{token}); err == nil || err.Error() != "evm_flow_provider_log_outside_range" {
		t.Fatalf("a log outside the requested range must be rejected, got %v", err)
	}
}

func TestFlowClientFreezeAndBalanceReads(t *testing.T) {
	holder := fixtureAddress(0x33)
	server := httptest.NewServer(rpcFixture{t: t, handler: func(method string, params []json.RawMessage) (int, string) {
		var call struct {
			To   string `json:"to"`
			Data string `json:"data"`
		}
		_ = json.Unmarshal(params[0], &call)
		if !strings.HasSuffix(call.Data, strings.TrimPrefix(holder, "0x")) {
			t.Errorf("call data must end with the holder address: %s", call.Data)
		}
		switch {
		case strings.HasPrefix(call.Data, "0xe47d6060"):
			return 200, `{"jsonrpc":"2.0","id":1,"result":"` + fixtureWord(big.NewInt(1)) + `"}`
		case strings.HasPrefix(call.Data, "0xfe575a87"):
			return 200, `{"jsonrpc":"2.0","id":1,"result":"0x"}`
		case strings.HasPrefix(call.Data, "0x70a08231"):
			return 200, `{"jsonrpc":"2.0","id":1,"result":"` + fixtureWord(big.NewInt(1_500_000)) + `"}`
		}
		return 200, `{"jsonrpc":"2.0","id":1,"error":{"code":3,"message":"execution reverted"}}`
	}})
	defer server.Close()
	client := FlowClient{URL: server.URL, HTTP: server.Client()}
	ctx := context.Background()
	tether := FlowToken{Address: fixtureAddress(0xaa), FreezeMethod: "isBlackListed(address)"}
	circle := FlowToken{Address: fixtureAddress(0xab), FreezeMethod: "isBlacklisted(address)"}
	if state := client.FreezeState(ctx, tether, holder); state != FreezeFrozen {
		t.Fatalf("tether freeze state = %s", state)
	}
	if state := client.FreezeState(ctx, circle, holder); state != FreezeUnknown {
		t.Fatalf("an empty return must be unknown, never not_frozen; got %s", state)
	}
	if state := client.FreezeState(ctx, FlowToken{Address: fixtureAddress(0xac)}, holder); state != FreezeUnsupported {
		t.Fatalf("a token without a freeze function = %s", state)
	}
	if balance, err := client.TokenBalance(ctx, tether.Address, holder); err != nil || balance.Int64() != 1_500_000 {
		t.Fatalf("balance=%v err=%v", balance, err)
	}
}

func TestParseFreezeWordAndNormalization(t *testing.T) {
	word := make([]byte, 32)
	if ParseFreezeWord(word) != FreezeClear {
		t.Fatal("zero word is not frozen")
	}
	word[31] = 1
	if ParseFreezeWord(word) != FreezeFrozen {
		t.Fatal("one word is frozen")
	}
	word[0] = 1
	if ParseFreezeWord(word) != FreezeUnknown || ParseFreezeWord([]byte{1}) != FreezeUnknown {
		t.Fatal("non-boolean words are unknown")
	}
	eighteen := new(big.Int).Mul(big.NewInt(25), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	if got := NormalizeUSD6(eighteen, 18); got.Int64() != 25_000_000 {
		t.Fatalf("18-decimal normalization = %s", got)
	}
	if got := NormalizeUSD6(big.NewInt(25_000_000), 6); got.Int64() != 25_000_000 {
		t.Fatalf("6-decimal normalization = %s", got)
	}
	if FormatUnits(big.NewInt(1_234_500), 6) != "1.2345" || FormatUnits(eighteen, 18) != "25" {
		t.Fatal("FormatUnits mismatch")
	}
}
