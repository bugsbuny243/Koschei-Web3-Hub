package services

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// SolanaRPCReadWitness is an in-memory, read-only copy of the exact JSON-RPC
// result bytes seen by the decoder. It is never source authentication, a
// consensus proof, an atomic snapshot or evidence-verdict authority.
type SolanaRPCReadWitness struct {
	RawResult  []byte    `json:"-"`
	Slot       uint64    `json:"-"`
	ObservedAt time.Time `json:"-"`
}

func (w SolanaRPCReadWitness) SHA256() string {
	if len(w.RawResult) == 0 {
		return ""
	}
	sum := sha256.Sum256(w.RawResult)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// solanaWitnessObjectField rejects ambiguous targeted object keys. The parser
// never manufactures a result from unmarshaled/re-marshaled object fields.
func solanaWitnessObjectField(raw []byte, key string) (json.RawMessage, bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return nil, false, errors.New("Solana RPC witness scope is not an object")
	}
	found := false
	var value json.RawMessage
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, false, errors.New("Solana RPC witness key is malformed")
		}
		name, ok := token.(string)
		if !ok {
			return nil, false, errors.New("Solana RPC witness key is invalid")
		}
		var next json.RawMessage
		if err := decoder.Decode(&next); err != nil {
			return nil, false, errors.New("Solana RPC witness value is invalid")
		}
		if name == key {
			if found {
				return nil, false, fmt.Errorf("ambiguous duplicate Solana RPC witness field %s", key)
			}
			found, value = true, next
		}
	}
	if last, err := decoder.Token(); err != nil || last != json.Delim('}') {
		return nil, false, errors.New("Solana RPC witness object is incomplete")
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, false, errors.New("Solana RPC witness contains trailing JSON")
	}
	return value, found, nil
}

// solanaRPCWitnessSlot extracts only the reported context slot. A missing
// context is represented by zero; consumers must reject zero for holder proof.
// Explicitly present malformed/duplicate slots fail closed.
func solanaRPCWitnessSlot(raw []byte) (uint64, error) {
	contextRaw, found, err := solanaWitnessObjectField(raw, "context")
	if err != nil || !found {
		return 0, err
	}
	if bytes.Equal(bytes.TrimSpace(contextRaw), []byte("null")) {
		return 0, errors.New("Solana RPC context is null")
	}
	slotRaw, found, err := solanaWitnessObjectField(contextRaw, "slot")
	if err != nil || !found {
		return 0, err
	}
	representation := string(slotRaw)
	slot, err := strconv.ParseUint(representation, 10, 64)
	if err != nil || slot == 0 || strconv.FormatUint(slot, 10) != representation {
		return 0, errors.New("Solana RPC slot is not a canonical positive uint64")
	}
	return slot, nil
}

// captureSolanaRPCReadWitness may return a zero slot only when the legacy
// result genuinely omitted context. This preserves existing callers; later
// review gates must explicitly reject incomplete witness metadata.
func captureSolanaRPCReadWitness(raw []byte) (SolanaRPCReadWitness, error) {
	if len(raw) == 0 || len(raw) > 2<<20 || !json.Valid(raw) {
		return SolanaRPCReadWitness{}, errors.New("Solana RPC witness result invalid or oversized")
	}
	slot, err := solanaRPCWitnessSlot(raw)
	if err != nil {
		return SolanaRPCReadWitness{}, err
	}
	return SolanaRPCReadWitness{
		RawResult: append([]byte(nil), raw...),
		Slot: slot,
		ObservedAt: time.Now().UTC(),
	}, nil
}

// VerifySource checks against the bytes retained at result decode time and
// refuses altered or contextless observations. Matching bytes and slots do
// not independently establish provider truth, consensus or account ownership.
func (w SolanaRPCReadWitness) VerifySource() error {
	if w.Slot == 0 || w.ObservedAt.IsZero() || len(w.RawResult) == 0 {
		return errors.New("Solana RPC witness is missing a source, slot or observation time")
	}
	slot, err := solanaRPCWitnessSlot(w.RawResult)
	if err != nil || slot != w.Slot {
		return errors.New("Solana RPC witness slot does not match original result bytes")
	}
	return nil
}

func (out *SolanaTokenSupplyResult) UnmarshalJSON(raw []byte) error {
	type bare SolanaTokenSupplyResult
	var parsed bare
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	witness, err := captureSolanaRPCReadWitness(raw)
	if err != nil {
		return err
	}
	*out = SolanaTokenSupplyResult(parsed)
	out.Witness = witness
	return nil
}

func (out *SolanaLargestAccountsResult) UnmarshalJSON(raw []byte) error {
	type bare SolanaLargestAccountsResult
	var parsed bare
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	witness, err := captureSolanaRPCReadWitness(raw)
	if err != nil {
		return err
	}
	*out = SolanaLargestAccountsResult(parsed)
	out.Witness = witness
	return nil
}

func (out *SolanaMultipleAccountInfoResult) UnmarshalJSON(raw []byte) error {
	type bare SolanaMultipleAccountInfoResult
	var parsed bare
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	witness, err := captureSolanaRPCReadWitness(raw)
	if err != nil {
		return err
	}
	*out = SolanaMultipleAccountInfoResult(parsed)
	out.Witness = witness
	return nil
}

// HolderRPCSourceSet is a four-read source set. Same-slot is a *required
// consistency check*, not proof that separate RPC calls were atomic.
type HolderRPCSourceSet struct {
	Supply        SolanaRPCReadWitness
	Largest       SolanaRPCReadWitness
	TokenAccounts SolanaRPCReadWitness
	OwnerAccounts SolanaRPCReadWitness
}

func (s HolderRPCSourceSet) RequireSameSlot() error {
	reads := []struct {
		name    string
		witness SolanaRPCReadWitness
	}{
		{"getTokenSupply", s.Supply},
		{"getTokenLargestAccounts", s.Largest},
		{"getMultipleAccounts(token accounts)", s.TokenAccounts},
		{"getMultipleAccounts(owner accounts)", s.OwnerAccounts},
	}
	var expected uint64
	for _, read := range reads {
		if err := read.witness.VerifySource(); err != nil {
			return fmt.Errorf("%s: %w", read.name, err)
		}
		if expected == 0 {
			expected = read.witness.Slot
		} else if expected != read.witness.Slot {
			return fmt.Errorf("non-atomic holder observations at different slots: %s", strings.TrimSpace(read.name))
		}
	}
	return nil
}
