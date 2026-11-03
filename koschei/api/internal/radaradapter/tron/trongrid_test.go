package tron

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Explicit synthetic fixtures: addresses and transaction IDs are repeated
// bytes and do not refer to real wallets or transactions.
func gridAddr(fill byte) string { return "41" + strings.Repeat(fmt.Sprintf("%02x", fill), 20) }
func gridTx(fill byte) string   { return strings.Repeat(fmt.Sprintf("%02x", fill), 32) }

func gridUSDTLog(from, to string, value *big.Int) string {
	usdt, _ := Address(USDTContract)
	return fmt.Sprintf(`{"address":"%s","topics":["%s","%s%s","%s%s"],"data":"%064x"}`,
		usdt[2:], transferTopic, strings.Repeat("0", 24), from[2:], strings.Repeat("0", 24), to[2:], value)
}

func TestBase58RoundTripsAndRejectsNonTronInput(t *testing.T) {
	for _, fill := range []byte{0x00, 0x11, 0xab, 0xff} {
		address := gridAddr(fill)
		encoded := Base58(address)
		back, err := Address(encoded)
		if err != nil || back != address || !strings.HasPrefix(encoded, "T") {
			t.Fatalf("round trip %s -> %q -> %q err=%v", address, encoded, back, err)
		}
	}
	usdt, _ := Address(USDTContract)
	if Base58(usdt) != USDTContract {
		t.Fatal("USDT contract must render back to its published Base58Check form")
	}
	for _, bad := range []string{"", "42" + strings.Repeat("11", 20), "41zz", gridAddr(0x11)[:40]} {
		if Base58(bad) != "" {
			t.Fatalf("Base58(%q) must be empty", bad)
		}
	}
}

func TestFormatUnitsKeepsExactFractions(t *testing.T) {
	cases := map[string]string{"612700000000000": "612700000", "1500000": "1.5", "5": "0.000005", "0": "0"}
	for raw, want := range cases {
		value, _ := new(big.Int).SetString(raw, 10)
		if got := FormatUnits(value, USDTDecimals); got != want {
			t.Fatalf("FormatUnits(%s) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseOutboundTRC20UsesEventLevelIdentity(t *testing.T) {
	watched, dest, other := Base58(gridAddr(0x11)), Base58(gridAddr(0x22)), Base58(gridAddr(0x33))
	raw := fmt.Sprintf(`{"success":true,"data":[
		{"transaction_id":"%[1]s","from":"%[2]s","to":"%[3]s","type":"Transfer","value":"100","block_timestamp":10,"token_info":{"address":"%[5]s","symbol":"USDT","decimals":6}},
		{"transaction_id":"%[1]s","from":"%[2]s","to":"%[4]s","type":"Transfer","value":"200","block_timestamp":10,"token_info":{"address":"%[5]s","symbol":"USDT","decimals":6}},
		{"transaction_id":"%[6]s","from":"%[3]s","to":"%[2]s","type":"Transfer","value":"5","block_timestamp":11,"token_info":{"address":"%[5]s"}},
		{"transaction_id":"%[6]s","from":"%[2]s","to":"%[3]s","type":"Approval","value":"5","block_timestamp":11,"token_info":{"address":"%[5]s"}},
		{"transaction_id":"not-hex","from":"%[2]s","to":"%[3]s","type":"Transfer","value":"5","block_timestamp":12,"token_info":{"address":"%[5]s"}},
		{"transaction_id":"%[6]s","from":"%[2]s","to":"%[3]s","type":"Transfer","value":"007","block_timestamp":12,"token_info":{"address":"%[5]s"}}
	]}`, gridTx(1), watched, dest, other, USDTContract, gridTx(2))
	transfers, err := ParseOutboundTRC20([]byte(raw), gridAddr(0x11))
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 2 {
		t.Fatalf("expected the two outbound events of one batch transaction, got %d", len(transfers))
	}
	if transfers[0].TransactionID != transfers[1].TransactionID || transfers[0].EventID == transfers[1].EventID {
		t.Fatalf("events of one transaction must share the transaction ID but not the event ID: %q %q", transfers[0].EventID, transfers[1].EventID)
	}
	if transfers[0].To != gridAddr(0x22) || transfers[1].To != gridAddr(0x33) || *transfers[0].Decimals != 6 {
		t.Fatalf("unexpected transfers %+v", transfers)
	}
	if _, err := ParseOutboundTRC20([]byte(`{"success":false}`), gridAddr(0x11)); err == nil {
		t.Fatal("an unsuccessful response must be an error, never an empty quiet result")
	}
}

func TestParseOutboundTRXKeepsSuccessfulNativeTransfers(t *testing.T) {
	watched, dest := gridAddr(0x11), gridAddr(0x22)
	raw := fmt.Sprintf(`{"success":true,"data":[
		{"txID":"%s","block_timestamp":10,"ret":[{"contractRet":"SUCCESS"}],"raw_data":{"contract":[{"type":"TransferContract","parameter":{"value":{"amount":46400000000000,"owner_address":"%s","to_address":"%s"}}}]}},
		{"txID":"%s","block_timestamp":11,"ret":[{"contractRet":"REVERT"}],"raw_data":{"contract":[{"type":"TransferContract","parameter":{"value":{"amount":1,"owner_address":"%s","to_address":"%s"}}}]}}
	]}`, gridTx(7), watched, dest, gridTx(8), watched, dest)
	transfers, err := ParseOutboundTRX([]byte(raw), watched)
	if err != nil || len(transfers) != 1 || transfers[0].ValueRaw != "46400000000000" || transfers[0].EventID != gridTx(7) || *transfers[0].Decimals != 6 {
		t.Fatalf("unexpected native transfers %+v err=%v", transfers, err)
	}
}

func TestConstantWordsNeverGuess(t *testing.T) {
	word := func(hexWord string, ok bool) []byte {
		return []byte(fmt.Sprintf(`{"result":{"result":%t},"constant_result":["%s"]}`, ok, hexWord))
	}
	zero, one := strings.Repeat("0", 64), strings.Repeat("0", 63)+"1"
	if ParseUSDTFreezeWord(word(zero, true)) != FreezeClear || ParseUSDTFreezeWord(word(one, true)) != FreezeFrozen ||
		ParseUSDTFreezeWord(word(zero, false)) != FreezeUnknown || ParseUSDTFreezeWord(word(strings.Repeat("0", 62)+"02", true)) != FreezeUnknown ||
		ParseUSDTFreezeWord([]byte("not json")) != FreezeUnknown {
		t.Fatal("freeze word mapping must be exact and default to unknown")
	}
	balance, err := ParseUSDTBalanceWord(word(fmt.Sprintf("%064x", big.NewInt(2_400_000_000_000)), true))
	if err != nil || balance.Cmp(big.NewInt(2_400_000_000_000)) != 0 {
		t.Fatalf("balance = %v err=%v", balance, err)
	}
	if _, err := ParseUSDTBalanceWord(word("zz", true)); err == nil {
		t.Fatal("a malformed balance must be an error")
	}
}

func TestParseBlockUSDTTransfersCarriesEventIndex(t *testing.T) {
	a, b, c := gridAddr(0x11), gridAddr(0x22), gridAddr(0x33)
	value := big.NewInt(612_700_000_000_000)
	raw := fmt.Sprintf(`[
		{"id":"%s","blockNumber":7,"blockTimeStamp":1000,"receipt":{"result":"SUCCESS"},"log":[%s,{"address":"%s","topics":["%s","%s%s","%s%s"],"data":"%064x"},%s]},
		{"id":"%s","blockNumber":7,"blockTimeStamp":1000,"result":"FAILED","receipt":{"result":"REVERT"},"log":[%s]}
	]`, gridTx(1), gridUSDTLog(a, b, value), strings.Repeat("ab", 20), transferTopic, strings.Repeat("0", 24), a[2:], strings.Repeat("0", 24), b[2:], value, gridUSDTLog(a, c, big.NewInt(1)),
		gridTx(2), gridUSDTLog(a, b, big.NewInt(1)))
	transfers, err := ParseBlockUSDTTransfers([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 2 || transfers[0].EventIndex != 0 || transfers[1].EventIndex != 2 || transfers[0].EventID() == transfers[1].EventID() {
		t.Fatalf("unexpected block transfers %+v", transfers)
	}
	if transfers[0].From != a || transfers[0].To != b || transfers[0].Value.Cmp(value) != 0 || transfers[0].BlockNumber != 7 {
		t.Fatalf("unexpected first transfer %+v", transfers[0])
	}
	if empty, err := ParseBlockUSDTTransfers([]byte(`{}`)); err != nil || len(empty) != 0 {
		t.Fatal("an empty block must parse as no transfers")
	}
	if _, err := ParseBlockUSDTTransfers([]byte(`{"Error":"x"}`)); err == nil {
		t.Fatal("a provider error object must not read as an empty block")
	}
}

func TestGridClientSendsKeyOnlyInHeaderAndReturnsCategories(t *testing.T) {
	const key = "fixture-trongrid-key-not-real"
	watched := gridAddr(0x11)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("TRON-PRO-API-KEY") != key || strings.Contains(r.URL.String(), key) {
			t.Errorf("API key must travel only in the TRON-PRO-API-KEY header")
		}
		switch r.URL.Path {
		case "/wallet/triggerconstantcontract":
			body, _ := io.ReadAll(r.Body)
			var call map[string]any
			_ = json.Unmarshal(body, &call)
			if call["parameter"] != strings.Repeat("0", 24)+watched[2:] {
				t.Errorf("unexpected constant call %s", body)
			}
			switch call["function_selector"] {
			case "isBlackListed(address)":
				fmt.Fprintf(w, `{"result":{"result":true},"constant_result":["%s"]}`, strings.Repeat("0", 64))
			case "balanceOf(address)":
				fmt.Fprintf(w, `{"result":{"result":true},"constant_result":["%064x"]}`, big.NewInt(42))
			}
		case "/walletsolidity/getnowblock":
			fmt.Fprint(w, `{"block_header":{"raw_data":{"number":100}}}`)
		default:
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	client := GridClient{BaseURL: server.URL, APIKey: key, HTTP: server.Client()}
	ctx := context.Background()
	if state := client.USDTFreezeState(ctx, watched); state != FreezeClear {
		t.Fatalf("freeze = %q", state)
	}
	if balance, err := client.USDTBalance(ctx, watched); err != nil || balance.Int64() != 42 {
		t.Fatalf("balance = %v err=%v", balance, err)
	}
	if head, err := client.SolidHead(ctx); err != nil || head != 100 {
		t.Fatalf("head = %d err=%v", head, err)
	}
	_, err := client.OutboundTRX(ctx, Base58(watched), watched)
	if err == nil || err.Error() != "tron_provider_http_500" || strings.Contains(err.Error(), server.URL) || strings.Contains(err.Error(), key) {
		t.Fatalf("provider failure must surface only a category, got %v", err)
	}
}
