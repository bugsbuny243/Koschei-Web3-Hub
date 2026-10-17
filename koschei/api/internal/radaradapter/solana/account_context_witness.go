package solana

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	radar "koschei/api/internal/radaradapter"
)

// uniqueJSONMember rejects duplicate keys at each checked scope. Standard
// json.Unmarshal silently accepts duplicate keys, which makes source lineage
// ambiguous when a provider returns more than one result/context/slot.
func uniqueJSONMember(raw []byte, key string) (json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return nil, errors.New("source scope must be a JSON object")
	}
	var value json.RawMessage
	found := false
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, errors.New("source JSON object key cannot be decoded")
		}
		name, ok := token.(string)
		if !ok {
			return nil, errors.New("source JSON object key is invalid")
		}
		var next json.RawMessage
		if err := decoder.Decode(&next); err != nil {
			return nil, errors.New("source JSON object value cannot be decoded")
		}
		if name != key {
			continue
		}
		if found {
			return nil, fmt.Errorf("duplicate source JSON %s key", key)
		}
		value = next
		found = true
	}
	last, err := decoder.Token()
	if err != nil || last != json.Delim('}') {
		return nil, errors.New("source JSON object is incomplete")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("source JSON has trailing content")
	}
	if !found {
		return nil, fmt.Errorf("source JSON %s is missing", key)
	}
	return value, nil
}

// accountResponseContextSlot returns the account read's own reported slot,
// not the previous getSlot value. minContextSlot is a lower bound only: the
// responses may be from different finalized slots and cannot prove an atomic
// snapshot. This check is not independent provider or consensus validation.
func accountResponseContextSlot(raw []byte, minContextSlot uint64) (uint64, error) {
	if minContextSlot == 0 {
		return 0, errors.New("account read requires a positive finalized lower bound")
	}
	if len(raw) == 0 || len(raw) > radar.RawJSONSourceWitnessMaxBytes ||
		!json.Valid(raw) {
		return 0, errors.New("raw account RPC response unavailable or invalid")
	}
	result, err := uniqueJSONMember(raw, "result")
	if err != nil {
		return 0, fmt.Errorf("getAccountInfo result: %w", err)
	}
	contextJSON, err := uniqueJSONMember(result, "context")
	if err != nil {
		return 0, fmt.Errorf("getAccountInfo context: %w", err)
	}
	rawSlot, err := uniqueJSONMember(contextJSON, "slot")
	if err != nil {
		return 0, fmt.Errorf("getAccountInfo context slot: %w", err)
	}
	// Parsing the raw JSON number directly rejects strings, floats, exponent
	// notation, negative, zero, and uint64 overflow without float rounding.
	slot, err := strconv.ParseUint(string(rawSlot), 10, 64)
	if err != nil || slot < minContextSlot || strconv.FormatUint(slot, 10) != string(rawSlot) {
		return 0, errors.New("account context slot is invalid or older than minContextSlot")
	}
	return slot, nil
}
