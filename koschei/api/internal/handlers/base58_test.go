package handlers

import "testing"

func TestIsValidSolanaAddressUsesLocalBase58Decoder(t *testing.T) {
	if !isValidSolanaAddress("11111111111111111111111111111111") {
		t.Fatal("expected Solana system program address to decode to 32 bytes")
	}
	if isValidSolanaAddress("0OIl") {
		t.Fatal("expected non-base58 characters to be rejected")
	}
	if isValidSolanaAddress("1111111111111111111111111111111") {
		t.Fatal("expected wrong decoded byte length to be rejected")
	}
}
