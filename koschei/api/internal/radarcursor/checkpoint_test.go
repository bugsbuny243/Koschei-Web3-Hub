package radarcursor

import (
	"strings"
	"testing"
	"time"
)

func TestCheckpointCanonicalizesAndValidates(t *testing.T) {
	got, err := (Checkpoint{
		CursorKey:         "  global-radar/head/ethereum-mainnet ",
		NetworkID:         " Ethereum-Mainnet ",
		StreamKind:        " EVM_HEAD ",
		Height:            42,
		BlockHash:         " 0xABC ",
		ParentHash:        " 0xDEF ",
		SourceEventSHA256: strings.Repeat("a", 64),
		State:             " CANONICAL ",
		ObservedAt:        time.Date(2026, 9, 25, 12, 0, 0, 0, time.FixedZone("test", 3*60*60)),
	}).Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion || got.NetworkID != "ethereum-mainnet" || got.BlockHash != "0xabc" || got.State != StateCanonical {
		t.Fatalf("unexpected canonical checkpoint: %#v", got)
	}
	if got.ObservedAt.Location() != time.UTC {
		t.Fatalf("observed_at not UTC: %v", got.ObservedAt)
	}
}

func TestCheckpointRejectsInvalidSourceDigest(t *testing.T) {
	_, err := (Checkpoint{
		CursorKey:         "cursor",
		NetworkID:         "ethereum-mainnet",
		StreamKind:        "evm_head",
		BlockHash:         "0xabc",
		SourceEventSHA256: "not-a-digest",
		State:             StateCanonical,
		ObservedAt:        time.Now().UTC(),
	}).Canonical()
	if err == nil {
		t.Fatal("invalid digest accepted")
	}
}
