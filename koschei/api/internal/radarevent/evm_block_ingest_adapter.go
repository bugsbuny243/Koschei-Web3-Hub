package radarevent

import (
	"fmt"
	"strconv"
	"strings"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/securityevidence"
)

func BuildEVMBlockIngestEvents(producer string, result networktarget.EVMBlockIngestResult) ([]Event, error) {
	producer = strings.TrimSpace(producer)
	if producer == "" {
		return nil, fmt.Errorf("evm_block_ingest_event_producer_required")
	}
	if result.NetworkID == "" || result.Hash == "" || result.ParentHash == "" ||
		result.BlockResponseSHA256 == "" || result.ChainIDResponseSHA256 == "" || result.LogsResponseSHA256 == "" {
		return nil, fmt.Errorf("evm_block_ingest_event_incomplete")
	}

	height := strconv.FormatUint(result.Height, 10)
	blockEvent, err := (Event{
		Producer:         producer,
		Kind:             KindBlock,
		NetworkID:        result.NetworkID,
		SubjectKind:      "block",
		SubjectID:        result.Hash,
		ObservedAtUnixMS: result.ObservedAt.UnixMilli(),
		State:            securityevidence.StateObserved,
		NativeRefs: []NativeReference{
			{Kind: "block_hash", Value: result.Hash},
			{Kind: "parent_hash", Value: result.ParentHash},
			{Kind: "block_height", Value: height},
			{Kind: "chain_id", Value: result.ChainID},
		},
		SourceDigests: []string{result.ChainIDResponseSHA256, result.BlockResponseSHA256},
		Facts: []Fact{
			{Key: "block_height", Value: height, Unit: "block", EvidenceSHA256: result.BlockResponseSHA256},
			{Key: "block_hash", Value: result.Hash, EvidenceSHA256: result.BlockResponseSHA256},
			{Key: "parent_hash", Value: result.ParentHash, EvidenceSHA256: result.BlockResponseSHA256},
			{Key: "transaction_count", Value: strconv.Itoa(len(result.TransactionHashes)), Unit: "transactions", EvidenceSHA256: result.BlockResponseSHA256},
			{Key: "log_count", Value: strconv.Itoa(len(result.Logs)), Unit: "logs", EvidenceSHA256: result.LogsResponseSHA256},
			{Key: "block_timestamp_unix", Value: strconv.FormatInt(result.BlockTimestamp.Unix(), 10), Unit: "seconds", EvidenceSHA256: result.BlockResponseSHA256},
		},
	}).Seal()
	if err != nil {
		return nil, err
	}

	events := make([]Event, 0, 1+len(result.TransactionHashes)+len(result.Logs))
	events = append(events, blockEvent)

	for index, txHash := range result.TransactionHashes {
		event, err := (Event{
			Producer:         producer,
			Kind:             KindTransaction,
			NetworkID:        result.NetworkID,
			SubjectKind:      "transaction",
			SubjectID:        txHash,
			ObservedAtUnixMS: result.ObservedAt.UnixMilli(),
			State:            securityevidence.StateObserved,
			NativeRefs: []NativeReference{
				{Kind: "transaction_hash", Value: txHash},
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

	for _, log := range result.Logs {
		logIndex := strconv.FormatUint(log.LogIndex, 10)
		subjectID := log.TxHash + ":log:" + logIndex
		nativeRefs := []NativeReference{
			{Kind: "transaction_hash", Value: log.TxHash},
			{Kind: "block_hash", Value: log.BlockHash},
			{Kind: "log_index", Value: logIndex},
			{Kind: "address", Value: log.Address},
		}
		if len(log.Topics) > 0 {
			nativeRefs = append(nativeRefs, NativeReference{Kind: "topic0", Value: log.Topics[0]})
		}
		event, err := (Event{
			Producer:         producer,
			Kind:             KindLog,
			NetworkID:        result.NetworkID,
			SubjectKind:      "log",
			SubjectID:        subjectID,
			ObservedAtUnixMS: result.ObservedAt.UnixMilli(),
			State:            securityevidence.StateObserved,
			NativeRefs:       nativeRefs,
			SourceDigests:    []string{result.LogsResponseSHA256},
			Facts: []Fact{
				{Key: "address", Value: log.Address, EvidenceSHA256: result.LogsResponseSHA256},
				{Key: "block_hash", Value: log.BlockHash, EvidenceSHA256: result.LogsResponseSHA256},
				{Key: "block_height", Value: height, Unit: "block", EvidenceSHA256: result.LogsResponseSHA256},
				{Key: "data_sha256", Value: log.DataSHA256, EvidenceSHA256: result.LogsResponseSHA256},
				{Key: "log_index", Value: logIndex, Unit: "index", EvidenceSHA256: result.LogsResponseSHA256},
				{Key: "removed", Value: strconv.FormatBool(log.Removed), EvidenceSHA256: result.LogsResponseSHA256},
				{Key: "topic_count", Value: strconv.Itoa(len(log.Topics)), Unit: "topics", EvidenceSHA256: result.LogsResponseSHA256},
			},
		}).Seal()
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, nil
}
