package handlers

import (
	"fmt"
	"strings"
	"testing"

	"koschei/api/internal/radaradapter/tron"
	"koschei/api/internal/services"
)

// Explicit synthetic fixtures: addresses and transaction IDs are generated
// from repeated bytes and do not refer to real incident wallets.
func tronFixtureHex(fill byte) string { return "41" + strings.Repeat(fmt.Sprintf("%02x", fill), 20) }
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

func usdtTransfer(tx byte, to string, valueRaw string, ts int64) tron.Transfer {
	usdt, _ := tron.Address(tron.USDTContract)
	six := 6
	return tron.Transfer{
		EventID: tronFixtureTxID(tx) + ":" + usdt + ":" + to + ":" + valueRaw, TransactionID: tronFixtureTxID(tx), Asset: "trc20",
		TokenContract: usdt, TokenSymbol: "USDT", From: tronFixtureHex(0x11), To: to, ValueRaw: valueRaw, Decimals: &six, BlockTimestampMS: ts,
	}
}

func TestDecorateWatchlistTronTransfersAddsDisplayAndBridgeTag(t *testing.T) {
	bridge := tronFixtureHex(0x33)
	items := decorateWatchlistTronTransfers([]tron.Transfer{usdtTransfer(1, bridge, "1500000", 1)}, map[string]services.TronBridge{bridge: {Label: "Fixture Bridge", Source: "https://example.invalid"}})
	if len(items) != 1 || items[0].Amount != "1.5" || items[0].ToBase58 != tron.Base58(bridge) || items[0].BridgeLabel != "Fixture Bridge" {
		t.Fatalf("unexpected decorated transfer %+v", items)
	}
	if got := formatWatchlistTronAmount("12", nil); got != "12 (raw units)" {
		t.Fatalf("missing decimals must stay raw, got %q", got)
	}
}

// Mirrors the incident shape: a quiet baseline, then scripted outbound USDT to
// a bridge plus ordinary transfers, then the issuer freezing remaining USDT.
func TestWatchlistTronAlertsFollowTheLaunderingSequence(t *testing.T) {
	watchedHex, bridgeHex, destHex := tronFixtureHex(0x11), tronFixtureHex(0x33), tronFixtureHex(0x22)
	bridges := map[string]services.TronBridge{bridgeHex: {Label: "Fixture Bridge", Source: "https://example.invalid/attribution"}}
	decorate := func(items ...tron.Transfer) []watchlistTronTransfer {
		return decorateWatchlistTronTransfers(items, bridges)
	}

	baselineFetch := decorate(usdtTransfer(1, destHex, "1000000000000", 1000))
	baseline := buildWatchlistTronSnapshot(nil, watchedHex, baselineFetch, tron.FreezeClear)
	if fresh := newWatchlistTronOutbound(nil, baselineFetch); len(fresh) != 0 {
		t.Fatal("the first check is a baseline and must not report history")
	}
	if got := compareWatchlistTronWallet(nil, baseline, nil); len(got) != 0 {
		t.Fatalf("a quiet baseline must not alert: %v", tronEventTypes(got))
	}

	secondFetch := append(decorate(
		usdtTransfer(2, bridgeHex, "1000000000000", 2000),
		usdtTransfer(3, destHex, "1000000000000", 2001),
		usdtTransfer(4, destHex, "1000000000000", 2002),
		usdtTransfer(5, destHex, "1000000000000", 2003),
		usdtTransfer(6, destHex, "1000000000000", 2004),
	), baselineFetch...)
	fresh := newWatchlistTronOutbound(&baseline, secondFetch)
	if len(fresh) != 5 || fresh[0].TransactionID != tronFixtureTxID(6) {
		t.Fatalf("expected the five unseen transfers newest first, got %d", len(fresh))
	}
	second := buildWatchlistTronSnapshot(&baseline, watchedHex, secondFetch, tron.FreezeClear)
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
	if !strings.Contains(bridgeAlert.Message, "1000000 USDT") || !strings.Contains(bridgeAlert.Message, "Only Tether can freeze USDT") || !strings.Contains(bridgeAlert.Message, tron.Base58(bridgeHex)) {
		t.Fatalf("bridge alert message = %q", bridgeAlert.Message)
	}

	late := decorate(usdtTransfer(10, destHex, "1", 2004-watchlistTronIndexLagMS+1), usdtTransfer(11, destHex, "1", 2004-watchlistTronIndexLagMS-1))
	if got := newWatchlistTronOutbound(&second, append(late, secondFetch...)); len(got) != 1 || got[0].TransactionID != tronFixtureTxID(10) {
		t.Fatalf("a late-indexed transfer inside the lag margin must alert, one outside must not; got %d", len(got))
	}

	unknown := buildWatchlistTronSnapshot(&second, watchedHex, secondFetch, tron.FreezeUnknown)
	if unknown.USDTFreezeState != tron.FreezeUnknown || unknown.USDTFreezeLastKnown != tron.FreezeClear {
		t.Fatalf("unknown check must record unknown and keep last known, got %q/%q", unknown.USDTFreezeState, unknown.USDTFreezeLastKnown)
	}
	if got := compareWatchlistTronWallet(&second, unknown, nil); len(got) != 0 {
		t.Fatalf("an unreadable freeze state must stay quiet: %v", tronEventTypes(got))
	}
	frozen := buildWatchlistTronSnapshot(&unknown, watchedHex, secondFetch, tron.FreezeFrozen)
	if got := tronEventTypes(compareWatchlistTronWallet(&unknown, frozen, nil)); len(got) != 1 || got[0] != "tron_usdt_frozen:info" {
		t.Fatalf("issuer freeze alerts = %v", got)
	}
	cleared := buildWatchlistTronSnapshot(&frozen, watchedHex, secondFetch, tron.FreezeClear)
	if got := tronEventTypes(compareWatchlistTronWallet(&frozen, cleared, nil)); len(got) != 1 || got[0] != "tron_usdt_unfrozen:high" {
		t.Fatalf("freeze lifted alerts = %v", got)
	}
	if got := tronEventTypes(compareWatchlistTronWallet(nil, frozen, nil)); len(got) != 1 || got[0] != "tron_usdt_frozen_present:info" {
		t.Fatalf("frozen at first check = %v", got)
	}
}

// One transaction can emit several TRC-20 transfers; each must stay a
// separate alert and a later-indexed sibling event must not be discarded.
func TestWatchlistTronBatchTransactionEventsStaySeparate(t *testing.T) {
	watchedHex, bridgeA, bridgeB := tronFixtureHex(0x11), tronFixtureHex(0x33), tronFixtureHex(0x44)
	bridges := map[string]services.TronBridge{
		bridgeA: {Label: "Bridge A", Source: "https://example.invalid/a"},
		bridgeB: {Label: "Bridge B", Source: "https://example.invalid/b"},
	}
	baseline := buildWatchlistTronSnapshot(nil, watchedHex, nil, tron.FreezeClear)
	first := decorateWatchlistTronTransfers([]tron.Transfer{usdtTransfer(7, bridgeA, "100", 5000)}, bridges)
	firstSnapshot := buildWatchlistTronSnapshot(&baseline, watchedHex, first, tron.FreezeClear)
	sibling := decorateWatchlistTronTransfers([]tron.Transfer{usdtTransfer(7, bridgeB, "200", 5000)}, bridges)
	fresh := newWatchlistTronOutbound(&firstSnapshot, append(sibling, first...))
	if len(fresh) != 1 || fresh[0].To != bridgeB {
		t.Fatalf("a later-indexed event of an already seen transaction must still alert, got %d", len(fresh))
	}
	both := decorateWatchlistTronTransfers([]tron.Transfer{usdtTransfer(8, bridgeA, "100", 6000), usdtTransfer(8, bridgeB, "200", 6000)}, bridges)
	alerts := compareWatchlistTronWallet(&baseline, buildWatchlistTronSnapshot(&baseline, watchedHex, both, tron.FreezeClear), newWatchlistTronOutbound(&baseline, both))
	if got := tronEventTypes(alerts); len(got) != 2 || got[0] != "tron_bridge_exit:critical" || got[1] != "tron_bridge_exit:critical" {
		t.Fatalf("two bridge exits in one transaction must both alert, got %v", got)
	}
}
