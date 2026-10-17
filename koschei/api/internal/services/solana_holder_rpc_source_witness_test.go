package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestHolderRPCReadWitnessOriginalBytesAndNoJSONLeak(t *testing.T) {
	raw := []byte(`{ "context": {"slot": 5511}, "value": {"amount":"1000000","decimals":6,"uiAmount":1} }`)
	var supply SolanaTokenSupplyResult
	if err := json.Unmarshal(raw, &supply); err != nil {
		t.Fatal(err)
	}
	if supply.Witness.Slot != 5511 || string(supply.Witness.RawResult) != string(raw) ||
		supply.Witness.ObservedAt.IsZero() {
		t.Fatalf("original RPC result was not retained exactly: %#v", supply.Witness)
	}
	hash := sha256.Sum256(raw)
	if supply.Witness.SHA256() != "sha256:"+hex.EncodeToString(hash[:]) {
		t.Fatal("witness digest did not cover actual source bytes")
	}
	if err := supply.Witness.VerifySource(); err != nil {
		t.Fatal(err)
	}
	output, err := json.Marshal(supply)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"RawResult", "ObservedAt", "5511", "Witness", "witness"} {
		if strings.Contains(string(output), forbidden) {
			t.Fatalf("internal RPC source witness escaped to public response: %s", output)
		}
	}
	// Changing the local source copy after decode must not alter the witness.
	raw[0] = '['
	if err := supply.Witness.VerifySource(); err != nil {
		t.Fatalf("decode did not retain independent byte copy: %v", err)
	}
}

func TestHolderRPCReadWitnessAllThreeResultTypesAndMissingContext(t *testing.T) {
	var largest SolanaLargestAccountsResult
	if err := json.Unmarshal([]byte(`{"context":{"slot":991},"value":[{"address":"token-account","amount":"1","decimals":0,"uiAmount":1}]}`), &largest); err != nil {
		t.Fatal(err)
	}
	var accounts SolanaMultipleAccountInfoResult
	if err := json.Unmarshal([]byte(`{"context":{"slot":991},"value":[null]}`), &accounts); err != nil {
		t.Fatal(err)
	}
	if largest.Witness.Slot != 991 || accounts.Witness.Slot != 991 ||
		largest.Witness.SHA256() == "" || accounts.Witness.SHA256() == "" {
		t.Fatalf("RPC result witness absent: %#v %#v", largest.Witness, accounts.Witness)
	}
	var legacy SolanaTokenSupplyResult
	if err := json.Unmarshal([]byte(`{"value":{"amount":"123","decimals":6}}`), &legacy); err != nil {
		t.Fatal("legacy decoded result without context must remain compatible: ", err)
	}
	if legacy.Witness.Slot != 0 || legacy.Witness.SHA256() == "" ||
		legacy.Witness.VerifySource() == nil {
		t.Fatalf("missing context was fabricated into a verified source: %#v", legacy.Witness)
	}
}

func TestHolderRPCSourceSetRequiresFourExactSameSlotWitnesses(t *testing.T) {
	makeWitness := func(slot uint64) SolanaRPCReadWitness {
		t.Helper()
		raw := []byte(`{"context":{"slot":991},"value":[]}`)
		w, err := captureSolanaRPCReadWitness(raw)
		if err != nil {
			t.Fatal(err)
		}
		w.Slot = slot
		return w
	}
	good := makeWitness(991)
	set := HolderRPCSourceSet{
		Supply: good, Largest: good, TokenAccounts: good, OwnerAccounts: good,
	}
	if err := set.RequireSameSlot(); err != nil {
		t.Fatalf("identical source slots incorrectly rejected: %v", err)
	}
	mismatch := set
	mismatch.TokenAccounts = makeWitness(992)
	if err := mismatch.RequireSameSlot(); err == nil {
		t.Fatal("mutated cached slot was accepted")
	}
	missing := set
	missing.OwnerAccounts = SolanaRPCReadWitness{}
	if err := missing.RequireSameSlot(); err == nil {
		t.Fatal("three-source set claimed four-source readiness")
	}
	different := set
	raw := []byte(`{"context":{"slot":992},"value":[]}`)
	var err error
	different.OwnerAccounts, err = captureSolanaRPCReadWitness(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := different.RequireSameSlot(); err == nil {
		t.Fatal("non-atomic different-slot holder reads were treated as one snapshot")
	}
	altered := set
	altered.Supply.RawResult = []byte(`{"context":{"slot":999},"value":[]}`)
	if err := altered.RequireSameSlot(); err == nil {
		t.Fatal("changed original source bytes still passed the slot witness gate")
	}
}

func TestHolderRPCReadWitnessRejectsAmbiguousAndInvalidContext(t *testing.T) {
	for _, raw := range []string{
		`{"context":{"slot":10,"slot":11},"value":[]}`,
		`{"context":{"slot":10},"context":{"slot":11},"value":[]}`,
		`{"context":{"slot":"10"},"value":[]}`,
		`{"context":{"slot":1e1},"value":[]}`,
		`{"context":{"slot":10.0},"value":[]}`,
		`{"context":{"slot":-1},"value":[]}`,
		`{"context":{"slot":18446744073709551616},"value":[]}`,
		`{"context":null,"value":[]}`,
	} {
		var result SolanaLargestAccountsResult
		if err := json.Unmarshal([]byte(raw), &result); err == nil {
			t.Fatalf("invalid/ambiguous source context accepted: %s", raw)
		}
	}
}

func TestHolderRPCWitnessCapturedThroughOriginalEnvelopedDecoder(t *testing.T) {
	resultBytes := []byte(`{ "context":{"slot":993}, "value":{"amount":"9007199254740993","decimals":6} }`)
	envelope := append([]byte(`{"jsonrpc":"2.0","id":1,"result":`), resultBytes...)
	envelope = append(envelope, '}')
	var decoded struct {
		Result SolanaTokenSupplyResult `json:"result"`
	}
	if err := json.Unmarshal(envelope, &decoded); err != nil {
		t.Fatal(err)
	}
	if string(decoded.Result.Witness.RawResult) != string(resultBytes) ||
		decoded.Result.Witness.Slot != 993 ||
		decoded.Result.Value.Amount != "9007199254740993" {
		t.Fatalf("RPC envelope discarded source bytes or rounded the integer amount: %#v", decoded.Result)
	}
}

func TestHolderRPCWitnessOversizePreservesLegacyDecodeWithoutGrantingProof(t *testing.T) {
	raw := []byte(`{"context":{"slot":991},"value":{"amount":"1","decimals":0},"unused":"` +
		strings.Repeat("x", (2<<20)+1) + `"}`)
	var result SolanaTokenSupplyResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("existing valid oversized response must remain readable: %v", err)
	}
	if result.Value.Amount != "1" || result.Witness.Slot != 0 ||
		len(result.Witness.RawResult) != 0 || result.Witness.VerifySource() == nil {
		t.Fatalf("oversized response gained unproven witness or lost decoded result: %#v", result)
	}
}
