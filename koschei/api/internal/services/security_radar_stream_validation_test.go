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
		Target: strings.Repeat("1", 88),
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
		Target: strings.Repeat("1", 44),
	})
	if err == nil {
		t.Fatal("signature target type must not be accepted by mint evidence persistence")
	}
}
