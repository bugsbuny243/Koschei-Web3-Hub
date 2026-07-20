package services

import (
	"encoding/base64"
	"encoding/binary"
	"testing"
)

func TestSolanaTokenAccountRawAmount(t *testing.T) {
	data := make([]byte, minimumTokenAccountSize)
	binary.LittleEndian.PutUint64(data[64:72], 987654321)
	info := &SolanaAccountInfo{Owner: splTokenProgramID, Data: []any{base64.StdEncoding.EncodeToString(data), "base64"}}
	amount, err := SolanaTokenAccountRawAmount(info)
	if err != nil {
		t.Fatalf("SolanaTokenAccountRawAmount() error = %v", err)
	}
	if amount != 987654321 {
		t.Fatalf("amount = %d, want 987654321", amount)
	}
}

func TestSolanaTokenAccountRawAmountAcceptsToken2022(t *testing.T) {
	data := make([]byte, minimumTokenAccountSize)
	binary.LittleEndian.PutUint64(data[64:72], 42)
	info := &SolanaAccountInfo{Owner: token2022ProgramID, Data: []any{base64.StdEncoding.EncodeToString(data), "base64"}}
	amount, err := SolanaTokenAccountRawAmount(info)
	if err != nil || amount != 42 {
		t.Fatalf("amount=%d error=%v", amount, err)
	}
}

func TestSolanaTokenAccountRawAmountRejectsShortData(t *testing.T) {
	info := &SolanaAccountInfo{Owner: splTokenProgramID, Data: []any{base64.StdEncoding.EncodeToString(make([]byte, 40)), "base64"}}
	if _, err := SolanaTokenAccountRawAmount(info); err == nil {
		t.Fatal("expected short token account data to be rejected")
	}
}

func TestSolanaTokenAccountRawAmountRejectsNonTokenOwner(t *testing.T) {
	data := make([]byte, minimumTokenAccountSize)
	binary.LittleEndian.PutUint64(data[64:72], 100)
	info := &SolanaAccountInfo{Owner: "11111111111111111111111111111111", Data: []any{base64.StdEncoding.EncodeToString(data), "base64"}}
	if _, err := SolanaTokenAccountRawAmount(info); err == nil {
		t.Fatal("expected non-token account owner to be rejected")
	}
}

func TestCleanGuardAccountAddressesDeduplicatesAndCaps(t *testing.T) {
	input := []string{"A", " A ", "B", "", "C"}
	got := cleanGuardAccountAddresses(input)
	if len(got) != 3 || got[0] != "A" || got[1] != "B" || got[2] != "C" {
		t.Fatalf("cleanGuardAccountAddresses() = %#v", got)
	}
}
