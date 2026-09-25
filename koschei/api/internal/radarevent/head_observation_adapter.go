package radarevent

import (
	"fmt"
	"strconv"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/securityevidence"
)

func BuildEVMHeadBlockEvent(producer string, result networktarget.EVMHeadObservationResult) (Event, error) {
	if result.NetworkID == "" || result.HeadBlockResponseSHA256 == "" || result.ChainIDResponseSHA256 == "" {
		return Event{}, fmt.Errorf("evm_head_event_incomplete")
	}
	height := strconv.FormatUint(result.HeadBlock, 10)
	return (Event{
		Producer: producer,
		Kind: KindBlock,
		NetworkID: result.NetworkID,
		SubjectKind: "block",
		SubjectID: result.NetworkID + ":height:" + height,
		ObservedAtUnixMS: result.ObservedAt.UnixMilli(),
		State: securityevidence.StateObserved,
		NativeRefs: []NativeReference{
			{Kind: "block_height", Value: height},
			{Kind: "chain_id", Value: result.ChainID},
		},
		SourceDigests: []string{result.ChainIDResponseSHA256, result.HeadBlockResponseSHA256},
		Facts: []Fact{
			{Key: "head_height", Value: height, Unit: "block", EvidenceSHA256: result.HeadBlockResponseSHA256},
			{Key: "observation_scope", Value: "single_rpc_endpoint_head"},
		},
	}).Seal()
}

func BuildBitcoinHeadBlockEvent(producer string, result networktarget.BitcoinHeadObservationResult) (Event, error) {
	if result.NetworkID != "bitcoin-mainnet" || result.BlockchainInfoResponseSHA256 == "" {
		return Event{}, fmt.Errorf("bitcoin_head_event_incomplete")
	}
	height := strconv.FormatUint(result.HeadBlock, 10)
	return (Event{
		Producer: producer,
		Kind: KindBlock,
		NetworkID: result.NetworkID,
		SubjectKind: "block",
		SubjectID: result.NetworkID + ":height:" + height,
		ObservedAtUnixMS: result.ObservedAt.UnixMilli(),
		State: securityevidence.StateObserved,
		NativeRefs: []NativeReference{{Kind: "block_height", Value: height}},
		SourceDigests: []string{result.BlockchainInfoResponseSHA256},
		Facts: []Fact{
			{Key: "head_height", Value: height, Unit: "block", EvidenceSHA256: result.BlockchainInfoResponseSHA256},
			{Key: "observation_scope", Value: "single_bitcoin_core_head"},
		},
	}).Seal()
}
