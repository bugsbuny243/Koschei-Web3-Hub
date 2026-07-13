package services

import (
	"database/sql"
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
