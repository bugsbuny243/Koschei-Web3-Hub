package evm

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	r "koschei/api/internal/radaradapter"
	"strconv"
	"strings"
)

type blockIdentity struct {
	Number string `json:"number"`
	Hash   string `json:"hash"`
}

func parseBlockIdentity(raw json.RawMessage) (blockIdentity, uint64, error) {
	var block blockIdentity
	if json.Unmarshal(raw, &block) != nil || len(block.Hash) != 66 || !strings.HasPrefix(block.Hash, "0x") {
		return block, 0, errors.New("block identity missing")
	}
	if _, err := hex.DecodeString(block.Hash[2:]); err != nil {
		return block, 0, errors.New("invalid block hash")
	}
	n, err := strconv.ParseUint(strings.TrimPrefix(block.Number, "0x"), 16, 64)
	if err != nil || block.Number != fmt.Sprintf("0x%x", n) {
		return block, 0, errors.New("invalid block number")
	}
	return block, n, nil
}

type controlSnapshot struct {
	address  string
	response r.Response
}

type historyRow struct {
	relation string
	response r.Response
	facts    map[string]any
}

// controlHistory compares two pinned observations, not a complete upgrade log.
// EIP-1898 requireCanonical is used for every historical storage/code read.
// Pruned or unsupported archive reads never fall back to latest state.
func (a *Adapter) controlHistory(ctx context.Context, head blockIdentity, height uint64, current map[string]controlSnapshot, add func(string, r.Response, map[string]any), limits *[]r.Limitation) {
	*limits = append(*limits, r.Limitation{Code: "bounded_control_history", Reason: "Control history compares two block snapshots up to 512 blocks apart. Equal endpoints do not exclude intermediate upgrades, changes before this window, or non-EIP-1967 authority paths. No upgrade transaction or actor is inferred."})
	if height == 0 || len(current) == 0 {
		return
	}
	from := uint64(0)
	if height > 512 {
		from = height - 512
	}
	number := fmt.Sprintf("0x%x", from)
	header, raw, err := a.rpc(ctx, "eth_getBlockByNumber", []any{number, false})
	old, oldHeight, parseErr := parseBlockIdentity(raw)
	if err != nil || parseErr != nil || oldHeight != from || old.Hash == head.Hash {
		*limits = append(*limits, r.Limitation{Code: "control_history_header_unknown", Reason: "Historical block identity could not be bound to the requested height."})
		return
	}
	anchor := map[string]any{"blockHash": old.Hash, "requireCanonical": true}
	pending := []historyRow{}
	for _, slot := range []struct{ name, key string }{{"implementation", ImplementationSlot}, {"admin", AdminSlot}, {"beacon", BeaconSlot}} {
		now, ok := current[slot.name]
		if !ok {
			continue
		}
		res, value, err := a.str(ctx, "eth_getStorageAt", []any{a.subject.NativeID, slot.key, anchor})
		before, addressErr := addressWord(value)
		if err != nil || addressErr != nil || !strings.HasPrefix(value, "0x") {
			*limits = append(*limits, r.Limitation{Code: "control_history_slot_unknown", Reason: slot.name + " historical state unavailable or malformed; no unchanged-state conclusion."})
			continue
		}
		facts := map[string]any{"slot": slot.key, "previous_address": before, "current_address": now.address, "changed_between_snapshots": before != now.address, "from_block": old.Number, "from_block_hash": old.Hash, "from_block_response_sha256": r.Digest(header.Raw), "current_state_response_sha256": r.Digest(now.response.Raw), "comparison_scope": "two_block_snapshots", "intermediate_changes": "unknown", "upgrade_transaction": "unknown", "finality": "not_established"}
		pending = append(pending, historyRow{"evm-control-history-" + slot.name, res, facts})
		if slot.name == "implementation" && !isZero(before) && !isZero(now.address) {
			oldCodeRes, oldCode, oldErr := a.str(ctx, "eth_getCode", []any{before, anchor})
			newCodeRes, newCode, newErr := a.str(ctx, "eth_getCode", []any{now.address, map[string]any{"blockHash": head.Hash, "requireCanonical": true}})
			beforeBytes, beforeErr := codeBytes(oldCode)
			nowBytes, nowErr := codeBytes(newCode)
			if oldErr != nil || newErr != nil || beforeErr != nil || nowErr != nil {
				*limits = append(*limits, r.Limitation{Code: "implementation_history_code_unknown", Reason: "Pinned implementation bytecode unavailable; code change is unknown."})
			} else {
				facts["previous_code_keccak256"] = Keccak(beforeBytes)
				facts["current_code_keccak256"] = Keccak(nowBytes)
				facts["code_changed_between_snapshots"] = Keccak(beforeBytes) != Keccak(nowBytes)
				facts["previous_code_response_sha256"] = r.Digest(oldCodeRes.Raw)
				facts["current_code_response_sha256"] = r.Digest(newCodeRes.Raw)
				facts["previous_code_bytes"] = len(beforeBytes)
				facts["current_code_bytes"] = len(nowBytes)
			}
		}
	}
	if len(pending) == 0 {
		return
	}
	// Discard only the comparison if either endpoint reorged while it was read.
	// Existing single-block observations remain observations at their own hashes.
	checks := map[string]string{}
	for _, bound := range []blockIdentity{old, head} {
		res, raw, err := a.rpc(ctx, "eth_getBlockByNumber", []any{bound.Number, false})
		observed, _, parseErr := parseBlockIdentity(raw)
		if err != nil || parseErr != nil || observed != bound {
			*limits = append(*limits, r.Limitation{Code: "control_history_reorg_or_unavailable", Reason: "Block endpoints changed or could not be rechecked; cross-block comparisons withheld."})
			return
		}
		checks[bound.Number] = r.Digest(res.Raw)
	}
	for _, row := range pending {
		row.facts["endpoint_recheck_response_sha256"] = checks
		add(row.relation, row.response, row.facts)
	}
}

func codeBytes(code string) ([]byte, error) {
	if !strings.HasPrefix(code, "0x") {
		return nil, errors.New("code lacks hex prefix")
	}
	return hex.DecodeString(code[2:])
}
