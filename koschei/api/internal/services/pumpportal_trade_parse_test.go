package services

import (
	"database/sql"
	"fmt"
	"testing"
)

func TestParsePumpPortalTradeEvent(t *testing.T) {
	payload := []byte(`{"txType":"buy","mint":"Mint111111111111111111111111111111111111","traderPublicKey":"Trader11111111111111111111111111111111","signature":"sig-1","solAmount":1.25,"tokenAmount":42,"slot":123,"timestamp":1700000000}`)
	event, ok := parsePumpPortalEvent(payload)
	if !ok {
		t.Fatal("event not parsed")
	}
	if event.Side != "buy" || event.SOLAmount != 1.25 || event.TokenAmount != 42 || event.Slot != 123 || event.BlockTime.IsZero() {
		t.Fatalf("unexpected event: %#v", event)
	}
	trade, ok := tokenTradeEventFromPumpPortal(event)
	if !ok || trade.Signature != "sig-1" || trade.Side != "buy" {
		t.Fatalf("unexpected trade: %#v ok=%t", trade, ok)
	}
}

func TestPumpTradeLedgerQueueNeverBlocksWhenFull(t *testing.T) {
	writer := &PumpTradeLedgerWriter{db: &sql.DB{}, queue: make(chan TokenTradeEvent, 1)}
	event := PumpPortalEvent{Mint: "Mint", Trader: "Trader", Signature: "sig", Side: "buy"}
	if !writer.EnqueuePumpPortal(event) {
		t.Fatal("first enqueue failed")
	}
	event.Signature = "sig-2"
	if writer.EnqueuePumpPortal(event) {
		t.Fatal("full queue should drop without blocking")
	}
}

func TestPumpPortalTradeSubscriptionEvictsOldestAtLimit(t *testing.T) {
	t.Setenv("PUMPPORTAL_TRADE_SUBSCRIPTION_LIMIT", "100")
	client := NewPumpPortalClient(PumpPortalConfig{})
	for i := 0; i < 100; i++ {
		added, evicted := client.rememberTradeMint(fmt.Sprintf("Mint-%03d", i))
		if !added || evicted != "" {
			t.Fatalf("unexpected add at %d: added=%t evicted=%q", i, added, evicted)
		}
	}
	added, evicted := client.rememberTradeMint("Mint-100")
	if !added || evicted != "Mint-000" {
		t.Fatalf("expected oldest eviction, added=%t evicted=%q", added, evicted)
	}
	if len(client.tradeOrder) != 100 || client.tradeMints["Mint-000"] || !client.tradeMints["Mint-100"] {
		t.Fatalf("subscription window not bounded: order=%d", len(client.tradeOrder))
	}
}
