package radarevent

import (
	"fmt"
	"strconv"
	"strings"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/securityevidence"
)

func BuildBitcoinBlockIngestEvents(producer string, result networktarget.BitcoinBlockIngestResult) ([]Event, error) {
	producer = strings.TrimSpace(producer)
	if producer == "" {
		return nil, fmt.Errorf("bitcoin_block_ingest_event_producer_required")
	}
	if result.NetworkID != "bitcoin-mainnet" || result.Hash == "" ||
		result.BlockchainInfoResponseSHA256 == "" || result.BlockHashResponseSHA256 == "" || result.BlockResponseSHA256 == "" {
		return nil, fmt.Errorf("bitcoin_block_ingest_event_incomplete")
	}

	height := strconv.FormatUint(result.Height, 10)
	nativeRefs := []NativeReference{
		{Kind: "block_hash", Value: result.Hash},
		{Kind: "block_height", Value: height},
	}
	if result.PreviousHash != "" {
		nativeRefs = append(nativeRefs, NativeReference{Kind: "parent_hash", Value: result.PreviousHash})
	}
	blockEvent, err := (Event{
		Producer:         producer,
		Kind:             KindBlock,
		NetworkID:        result.NetworkID,
		SubjectKind:      "block",
		SubjectID:        result.Hash,
		ObservedAtUnixMS: result.ObservedAt.UnixMilli(),
		State:            securityevidence.StateObserved,
		NativeRefs:       nativeRefs,
		SourceDigests:    []string{result.BlockchainInfoResponseSHA256, result.BlockHashResponseSHA256, result.BlockResponseSHA256},
		Facts: []Fact{
			{Key: "block_height", Value: height, Unit: "block", EvidenceSHA256: result.BlockResponseSHA256},
			{Key: "block_hash", Value: result.Hash, EvidenceSHA256: result.BlockResponseSHA256},
			{Key: "block_timestamp_unix", Value: strconv.FormatInt(result.BlockTime.Unix(), 10), Unit: "seconds", EvidenceSHA256: result.BlockResponseSHA256},
			{Key: "transaction_count", Value: strconv.Itoa(len(result.TransactionIDs)), Unit: "transactions", EvidenceSHA256: result.BlockResponseSHA256},
		},
	}).Seal()
	if err != nil {
		return nil, err
	}

	events := make([]Event, 0, 1+len(result.TransactionIDs))
	events = append(events, blockEvent)
	for index, txid := range result.TransactionIDs {
		event, err := (Event{
			Producer:         producer,
			Kind:             KindTransaction,
			NetworkID:        result.NetworkID,
			SubjectKind:      "transaction",
			SubjectID:        txid,
			ObservedAtUnixMS: result.ObservedAt.UnixMilli(),
			State:            securityevidence.StateObserved,
			NativeRefs: []NativeReference{
				{Kind: "transaction_id", Value: txid},
				{Kind: "block_hash", Value: result.Hash},
				{Kind: "block_height", Value: height},
			},
			SourceDigests: []string{result.BlockResponseSHA256},
			Facts: []Fact{
				{Key: "block_hash", Value: result.Hash, EvidenceSHA256: result.BlockResponseSHA256},
				{Key: "block_height", Value: height, Unit: "block", EvidenceSHA256: result.BlockResponseSHA256},
				{Key: "transaction_index", Value: strconv.Itoa(index), Unit: "index", EvidenceSHA256: result.BlockResponseSHA256},
			},
		}).Seal()
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}
