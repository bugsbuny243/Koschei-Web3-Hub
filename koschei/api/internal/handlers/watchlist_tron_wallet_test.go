package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"koschei/api/internal/radaradapter/tron"
)

// Explicit synthetic fixtures: these addresses and transaction IDs are
// generated from repeated bytes and do not refer to real incident wallets.
func tronFixtureBase58(t *testing.T, fill byte) (string, string) {
	t.Helper()
	payload := append([]byte{0x41}, []byte(strings.Repeat(string([]byte{fill}), 20))...)
	first := sha256.Sum256(payload)
	second := sha256.Sum256(first[:])
	full := append(append([]byte{}, payload...), second[:4]...)
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	n := new(big.Int).SetBytes(full)
	out := []byte{}
	mod := new(big.Int)
	for n.Sign() > 0 {
		n.DivMod(n, big.NewInt(58), mod)
		out = append([]byte{alphabet[mod.Int64()]}, out...)
	}
	encoded := string(out)
	hexAddress, err := tron.Address(encoded)
	if err != nil {
		t.Fatalf("fixture address did not round-trip: %v", err)
	}
	return encoded, hexAddress
}

func tronFixtureTxID(fill byte) string {
	return strings.Repeat(fmt.Sprintf("%02x", fill), 32)
}

func tronEventTypes(alerts []watchlistAlertCandidate) []string {
	out := []string{}
	for _, a := range alerts {
		out = append(out, a.EventType+":"+a.Severity)
	}
	return out
}

func TestWatchlistTronUSDTContractIsValidBase58Check(t *testing.T) {
	hexAddress, err := tron.Address(watchlistTronUSDTContract)
	if err != nil || len(hexAddress) != 42 || !strings.HasPrefix(hexAddress, "41") {
		t.Fatalf("USDT contract must decode to a 21-byte Tron address, got %q err=%v", hexAddress, err)
	}
}

func TestParseWatchlistTronTRC20KeepsOnlyValidOutboundTransfers(t *testing.T) {
	watched, watchedHex := tronFixtureBase58(t, 0x11)
	dest, destHex := tronFixtureBase58(t, 0x22)
	raw := fmt.Sprintf(`{"success":true,"data":[
		{"transaction_id":"%s","from":"%s","to":"%s","type":"Transfer","value":"612700000000000","block_timestamp":1762000000000,"token_info":{"address":"%s","symbol":"USDT","decimals":6}},
		{"transaction_id":"%s","from":"%s","to":"%s","type":"Transfer","value":"5","block_timestamp":1762000000001,"token_info":{"address":"%s","symbol":"USDT","decimals":6}},
		{"transaction_id":"%s","from":"%s","to":"%s","type":"Approval","value":"5","block_timestamp":1762000000002,"token_info":{"address":"%s"}},
		{"transaction_id":"not-hex","from":"%s","to":"%s","type":"Transfer","value":"5","block_timestamp":1762000000003,"token_info":{"address":"%s"}},
		{"transaction_id":"%s","from":"%s","to":"%s","type":"Transfer","value":"-5","block_timestamp":1762000000004,"token_info":{"address":"%s"}}
	]}`,
		tronFixtureTxID(1), watched, dest, watchlistTronUSDTContract,
		tronFixtureTxID(2), dest, watched, watchlistTronUSDTContract, // inbound: ignored
		tronFixtureTxID(3), watched, dest, watchlistTronUSDTContract, // approval: ignored
		watched, dest, watchlistTronUSDTContract, // malformed id
		tronFixtureTxID(5), watched, dest, watchlistTronUSDTContract, // negative value
	)
	transfers, err := parseWatchlistTronTRC20([]byte(raw), watchedHex, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 {
		t.Fatalf("expected exactly one valid outbound transfer, got %d", len(transfers))
	}
	got := transfers[0]
	if got.TransactionID != tronFixtureTxID(1) || got.To != destHex || got.TokenSymbol != "USDT" || got.Decimals == nil || *got.Decimals != 6 {
		t.Fatalf("unexpected transfer %+v", got)
	}
	if got.ToBase58 != dest {
		t.Fatalf("display address = %q, want %q", got.ToBase58, dest)
	}
	if amount := formatWatchlistTronAmount(got.ValueRaw, got.Decimals); amount != "612700000" {
		t.Fatalf("amount = %q", amount)
	}
	if _, err := parseWatchlistTronTRC20([]byte(`{"success":false,"data":[]}`), watchedHex, nil); err == nil {
		t.Fatal("an unsuccessful TronGrid response must be an error, not an empty quiet result")
	}
}

func TestParseWatchlistTronTRXKeepsOnlySuccessfulNativeTransfers(t *testing.T) {
	_, watchedHex := tronFixtureBase58(t, 0x11)
	_, destHex := tronFixtureBase58(t, 0x22)
	raw := fmt.Sprintf(`{"success":true,"data":[
		{"txID":"%s","block_timestamp":1762000000000,"ret":[{"contractRet":"SUCCESS"}],"raw_data":{"contract":[{"type":"TransferContract","parameter":{"value":{"amount":46400000000000,"owner_address":"%s","to_address":"%s"}}}]}},
		{"txID":"%s","block_timestamp":1762000000001,"ret":[{"contractRet":"REVERT"}],"raw_data":{"contract":[{"type":"TransferContract","parameter":{"value":{"amount":1,"owner_address":"%s","to_address":"%s"}}}]}},
		{"txID":"%s","block_timestamp":1762000000002,"ret":[{"contractRet":"SUCCESS"}],"raw_data":{"contract":[{"type":"TriggerSmartContract","parameter":{"value":{"owner_address":"%s"}}}]}}
	]}`, tronFixtureTxID(7), watchedHex, destHex, tronFixtureTxID(8), watchedHex, destHex, tronFixtureTxID(9), watchedHex)
	transfers, err := parseWatchlistTronTRX([]byte(raw), watchedHex, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 || transfers[0].Asset != "trx" || transfers[0].ValueRaw != "46400000000000" {
		t.Fatalf("unexpected native transfers %+v", transfers)
	}
	if amount := formatWatchlistTronAmount(transfers[0].ValueRaw, transfers[0].Decimals); amount != "46400000" {
		t.Fatalf("TRX amount = %q", amount)
	}
}

func TestParseWatchlistTronFreezeStateNeverGuessesNotFrozen(t *testing.T) {
	cases := map[string]string{
		`{"result":{"result":true},"constant_result":["` + strings.Repeat("0", 64) + `"]}`:   watchlistTronFreezeClear,
		`{"result":{"result":true},"constant_result":["` + strings.Repeat("0", 63) + `1"]}`:  watchlistTronFreezeFrozen,
		`{"result":{"result":false},"constant_result":["` + strings.Repeat("0", 64) + `"]}`:  watchlistTronFreezeUnknown,
		`{"result":{"result":true},"constant_result":[]}`:                                    watchlistTronFreezeUnknown,
		`{"result":{"result":true},"constant_result":["` + strings.Repeat("0", 62) + `02"]}`: watchlistTronFreezeUnknown,
		`not json`: watchlistTronFreezeUnknown,
	}
	for raw, want := range cases {
		if got := parseWatchlistTronFreezeState([]byte(raw)); got != want {
			t.Fatalf("freeze state for %s = %q, want %q", raw, got, want)
		}
	}
}

func TestWatchlistTronBridgeRegistryRequiresAddressLabelAndHTTPSSource(t *testing.T) {
	bridge, bridgeHex := tronFixtureBase58(t, 0x33)
	other, _ := tronFixtureBase58(t, 0x44)
	t.Setenv("WATCHLIST_TRON_BRIDGE_ADDRESSES_JSON", fmt.Sprintf(`{
		"%s":{"label":"Fixture Bridge","source":"https://example.invalid/attribution"},
		"%s":{"label":"Insecure","source":"http://example.invalid"},
		"not-an-address":{"label":"Bad","source":"https://example.invalid"}
	}`, bridge, other))
	bridges := watchlistTronBridges()
	if len(bridges) != 1 || bridges[bridgeHex].Label != "Fixture Bridge" {
		t.Fatalf("unexpected bridge registry %+v", bridges)
	}
	t.Setenv("WATCHLIST_TRON_BRIDGE_ADDRESSES_JSON", "{broken")
	if len(watchlistTronBridges()) != 0 {
		t.Fatal("a malformed registry must tag nothing")
	}
}

// Mirrors the incident shape: a quiet baseline, then scripted outbound USDT to
// a bridge plus ordinary transfers, then the issuer freezing remaining USDT.
func TestWatchlistTronAlertsFollowTheLaunderingSequence(t *testing.T) {
	_, watchedHex := tronFixtureBase58(t, 0x11)
	_, bridgeHex := tronFixtureBase58(t, 0x33)
	_, destHex := tronFixtureBase58(t, 0x22)
	usdt, _ := tron.Address(watchlistTronUSDTContract)
	six := 6
	transfer := func(fill byte, to string, ts int64, bridge bool) watchlistTronTransfer {
		item := watchlistTronTransfer{TransactionID: tronFixtureTxID(fill), Asset: "trc20", TokenContract: usdt, TokenSymbol: "USDT", To: to, ValueRaw: "1000000000000", Decimals: &six, BlockTimestampMS: ts}
		if bridge {
			item.BridgeLabel, item.BridgeSource = "Fixture Bridge", "https://example.invalid/attribution"
		}
		return item
	}

	baselineFetch := []watchlistTronTransfer{transfer(1, destHex, 1000, false)}
	baseline := buildWatchlistTronSnapshot(nil, watchedHex, baselineFetch, watchlistTronFreezeClear)
	if fresh := newWatchlistTronOutbound(nil, baselineFetch); len(fresh) != 0 {
		t.Fatal("the first check is a baseline and must not report history")
	}
	if got := compareWatchlistTronWallet(nil, baseline, nil); len(got) != 0 {
		t.Fatalf("a quiet baseline must not alert: %v", tronEventTypes(got))
	}

	secondFetch := append([]watchlistTronTransfer{
		transfer(2, bridgeHex, 2000, true),
		transfer(3, destHex, 2001, false),
		transfer(4, destHex, 2002, false),
		transfer(5, destHex, 2003, false),
		transfer(6, destHex, 2004, false),
	}, baselineFetch...)
	fresh := newWatchlistTronOutbound(&baseline, secondFetch)
	if len(fresh) != 5 || fresh[0].TransactionID != tronFixtureTxID(6) {
		t.Fatalf("expected the five unseen transfers newest first, got %d", len(fresh))
	}
	late := transfer(10, destHex, 2004-watchlistTronIndexLagMS+1, false)
	tooOld := transfer(11, destHex, 2004-watchlistTronIndexLagMS-1, false)
	secondLike := buildWatchlistTronSnapshot(&baseline, watchedHex, secondFetch, watchlistTronFreezeClear)
	if got := newWatchlistTronOutbound(&secondLike, append([]watchlistTronTransfer{late, tooOld}, secondFetch...)); len(got) != 1 || got[0].TransactionID != tronFixtureTxID(10) {
		t.Fatalf("a late-indexed transfer inside the lag margin must alert, one outside must not; got %d", len(got))
	}
	second := buildWatchlistTronSnapshot(&baseline, watchedHex, secondFetch, watchlistTronFreezeClear)
	alerts := compareWatchlistTronWallet(&baseline, second, fresh)
	types := tronEventTypes(alerts)
	if len(alerts) != 6 || types[0] != "tron_bridge_exit:critical" || types[1] != "tron_outbound_burst:critical" {
		t.Fatalf("unexpected laundering alerts %v", types)
	}
	bridgeAlert := alerts[0]
	options, _ := bridgeAlert.Evidence["response_options"].([]string)
	if strings.Join(options, ",") != "issuer_freeze_request:tether_usdt,bridge_operator_notice:Fixture Bridge,receiving_party_notice" {
		t.Fatalf("unexpected response options %v", options)
	}
	if bridgeAlert.Evidence["koschei_can_reverse_transfer"] != false || bridgeAlert.Evidence["identity_or_intent_claim"] != false {
		t.Fatal("alerts must not claim Koschei can reverse transfers or identify intent")
	}
	if !strings.Contains(bridgeAlert.Message, "1000000 USDT") || !strings.Contains(bridgeAlert.Message, "Only Tether can freeze USDT") {
		t.Fatalf("bridge alert message = %q", bridgeAlert.Message)
	}
	if second.LatestOutboundMS != 2004 || second.LastOutbound == nil || second.LastOutbound.TransactionID != tronFixtureTxID(6) {
		t.Fatalf("snapshot must keep the newest outbound transfer, got %+v", second.LastOutbound)
	}

	// The issuer freezes USDT; an unreadable check in between must not
	// manufacture a transition in either direction.
	unknown := buildWatchlistTronSnapshot(&second, watchedHex, secondFetch, watchlistTronFreezeUnknown)
	if unknown.USDTFreezeState != watchlistTronFreezeUnknown || unknown.USDTFreezeLastKnown != watchlistTronFreezeClear {
		t.Fatalf("unknown check must record unknown and keep last known, got %q/%q", unknown.USDTFreezeState, unknown.USDTFreezeLastKnown)
	}
	if got := compareWatchlistTronWallet(&second, unknown, nil); len(got) != 0 {
		t.Fatalf("an unreadable freeze state must stay quiet: %v", tronEventTypes(got))
	}
	frozen := buildWatchlistTronSnapshot(&unknown, watchedHex, secondFetch, watchlistTronFreezeFrozen)
	if got := tronEventTypes(compareWatchlistTronWallet(&unknown, frozen, nil)); len(got) != 1 || got[0] != "tron_usdt_frozen:info" {
		t.Fatalf("issuer freeze alerts = %v", got)
	}
	cleared := buildWatchlistTronSnapshot(&frozen, watchedHex, secondFetch, watchlistTronFreezeClear)
	if got := tronEventTypes(compareWatchlistTronWallet(&frozen, cleared, nil)); len(got) != 1 || got[0] != "tron_usdt_unfrozen:high" {
		t.Fatalf("freeze lifted alerts = %v", got)
	}
	if got := tronEventTypes(compareWatchlistTronWallet(nil, frozen, nil)); len(got) != 1 || got[0] != "tron_usdt_frozen_present:info" {
		t.Fatalf("frozen at first check = %v", got)
	}
}

func TestWatchlistTronFetchSendsKeyOnlyInHeaderAndHidesProviderDetails(t *testing.T) {
	const key = "fixture-trongrid-key-not-real"
	watched, watchedHex := tronFixtureBase58(t, 0x11)
	dest, _ := tronFixtureBase58(t, 0x22)
	var sawConstantCall bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("TRON-PRO-API-KEY") != key || strings.Contains(r.URL.String(), key) {
			t.Errorf("API key must travel only in the TRON-PRO-API-KEY header")
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/transactions/trc20"):
			if r.URL.Query().Get("only_from") != "true" || r.URL.Query().Get("only_confirmed") != "true" {
				t.Errorf("trc20 query must request confirmed outbound transfers: %s", r.URL.RawQuery)
			}
			fmt.Fprintf(w, `{"success":true,"data":[{"transaction_id":"%s","from":"%s","to":"%s","type":"Transfer","value":"1","block_timestamp":1,"token_info":{"address":"%s","symbol":"USDT","decimals":6}}]}`,
				tronFixtureTxID(1), watched, dest, watchlistTronUSDTContract)
		case strings.HasSuffix(r.URL.Path, "/wallet/triggerconstantcontract"):
			sawConstantCall = true
			body, _ := io.ReadAll(r.Body)
			var call map[string]any
			_ = json.Unmarshal(body, &call)
			if call["function_selector"] != "isBlackListed(address)" || call["parameter"] != strings.Repeat("0", 24)+watchedHex[2:] {
				t.Errorf("unexpected constant call %s", body)
			}
			fmt.Fprintf(w, `{"result":{"result":true},"constant_result":["%s1"]}`, strings.Repeat("0", 63))
		default:
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	previousURL, previousClient := watchlistTronBaseURL, watchlistTronHTTPClient
	watchlistTronBaseURL, watchlistTronHTTPClient = server.URL, server.Client()
	t.Cleanup(func() { watchlistTronBaseURL, watchlistTronHTTPClient = previousURL, previousClient })
	t.Setenv("TRONGRID_API_KEY", key)

	transfers, err := watchlistTronFetchTRC20Outbound(context.Background(), watched, watchedHex, nil)
	if err != nil || len(transfers) != 1 {
		t.Fatalf("trc20 fetch = %d transfers, err=%v", len(transfers), err)
	}
	if state := watchlistTronUSDTFreezeState(context.Background(), watchedHex); state != watchlistTronFreezeFrozen || !sawConstantCall {
		t.Fatalf("freeze state = %q", state)
	}
	_, err = watchlistTronFetchTRXOutbound(context.Background(), watched, watchedHex, nil)
	if err == nil || err.Error() != "tron_provider_http_500" || strings.Contains(err.Error(), server.URL) || strings.Contains(err.Error(), key) {
		t.Fatalf("provider failure must surface only a category, got %v", err)
	}
}
