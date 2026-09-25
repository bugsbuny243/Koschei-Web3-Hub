package radarevent

import (
	"strings"
	"testing"
	"time"

	"koschei/api/internal/networktarget"
)

func TestBuildEVMBlockIngestEventsProducesCanonicalBlockTransactionAndLogEvents(t *testing.T) {
	blockHash := "0x" + strings.Repeat("a", 64)
	parentHash := "0x" + strings.Repeat("b", 64)
	txHash := "0x" + strings.Repeat("c", 64)
	result := networktarget.EVMBlockIngestResult{
		NetworkID:         "ethereum-mainnet",
		ChainID:           "0x1",
		Height:            42,
		Hash:              blockHash,
		ParentHash:        parentHash,
		BlockTimestamp:    time.Unix(1700000000, 0).UTC(),
		ObservedAt:        time.Date(2026, 9, 25, 13, 0, 0, 0, time.UTC),
		TransactionHashes: []string{txHash},
		Logs: []networktarget.EVMLogObservation{{
			Address:     "0x" + strings.Repeat("d", 40),
			Topics:      []string{"0x" + strings.Repeat("e", 64)},
			DataSHA256:  strings.Repeat("f", 64),
			TxHash:      txHash,
			BlockHash:   blockHash,
			BlockNumber: 42,
			LogIndex:    0,
		}},
		ChainIDResponseSHA256: strings.Repeat("1", 64),
		BlockResponseSHA256:   strings.Repeat("2", 64),
		LogsResponseSHA256:    strings.Repeat("3", 64),
	}

	events, err := BuildEVMBlockIngestEvents("evm-block-ingest-adapter", result)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].Kind != KindBlock || events[1].Kind != KindTransaction || events[2].Kind != KindLog {
		t.Fatalf("unexpected events: %#v", events)
	}
	for _, event := range events {
		if event.ObservedAtUnixMS != result.BlockTimestamp.UnixMilli() {
			t.Fatalf("event timestamp is not native block time: %#v", event)
		}
		if err := event.Verify(); err != nil {
			t.Fatalf("event did not verify: %v %#v", err, event)
		}
	}
	if events[0].SubjectID != blockHash || events[1].SubjectID != txHash || events[2].SubjectID != txHash+":log:0" {
		t.Fatalf("unexpected identities: %#v", events)
	}
}

func TestBuildBitcoinBlockIngestEventsProducesCanonicalBlockAndTransactionEvents(t *testing.T) {
	result := networktarget.BitcoinBlockIngestResult{
		NetworkID:                    "bitcoin-mainnet",
		Height:                       100,
		Hash:                         strings.Repeat("a", 64),
		PreviousHash:                 strings.Repeat("b", 64),
		BlockTime:                    time.Unix(1700000000, 0).UTC(),
		ObservedAt:                   time.Date(2026, 9, 25, 13, 0, 0, 0, time.UTC),
		TransactionIDs:               []string{strings.Repeat("c", 64)},
		BlockchainInfoResponseSHA256: strings.Repeat("1", 64),
		BlockHashResponseSHA256:      strings.Repeat("2", 64),
		BlockResponseSHA256:          strings.Repeat("3", 64),
	}

	events, err := BuildBitcoinBlockIngestEvents("bitcoin-block-ingest-adapter", result)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Kind != KindBlock || events[1].Kind != KindTransaction {
		t.Fatalf("unexpected events: %#v", events)
	}
	for _, event := range events {
		if event.ObservedAtUnixMS != result.BlockTime.UnixMilli() {
			t.Fatalf("event timestamp is not native block time: %#v", event)
		}
		if err := event.Verify(); err != nil {
			t.Fatalf("event did not verify: %v %#v", err, event)
		}
	}
}
