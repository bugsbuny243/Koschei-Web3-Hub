package services

import (
	"context"
	"strings"
	"testing"
)

func TestInsertStreamEventRejectsSignatureAsTokenTarget(t *testing.T) {
	store := &SecurityRadarStore{}
	_, err := store.InsertStreamEvent(context.Background(), SecurityRadarStreamEventRecord{
		TargetType: "token",
		Target:     strings.Repeat("1", 88),
		Signature: strings.Repeat("1", 88),
	})
	if err == nil {
		t.Fatal("signature must never be persisted as a token target")
	}
}

func TestInsertStreamEventRejectsSignatureTargetType(t *testing.T) {
	store := &SecurityRadarStore{}
	_, err := store.InsertStreamEvent(context.Background(), SecurityRadarStreamEventRecord{
		TargetType: "signature",
		Target:     strings.Repeat("1", 44),
	})
	if err == nil {
		t.Fatal("signature target type must not be accepted by mint evidence persistence")
	}
}

func TestPureRadarStreamTradeFiltered(t *testing.T) {
	for _, instruction := range []string{"Instruction: Buy", "Instruction: Sell"} {
		event := SecurityRadarStreamEventRecord{Decoded: map[string]any{"logs": []string{instruction}}}
		if !isPureRadarStreamTrade(event) {
			t.Fatalf("pure trade %q must be filtered", instruction)
		}
	}
}

func TestRadarStreamLifecycleNotFiltered(t *testing.T) {
	event := SecurityRadarStreamEventRecord{Decoded: map[string]any{"logs": []string{"Instruction: Buy", "Instruction: InitializeMint"}}}
	if isPureRadarStreamTrade(event) {
		t.Fatal("lifecycle signal must not be dropped with trade noise")
	}
}
