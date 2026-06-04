package handlers

import "testing"

func TestNormalizeAndValidateSourceAddressAcceptsLowercaseEVM(t *testing.T) {
	addr, err := normalizeAndValidateSourceAddress("base", "0x1234567890abcdef1234567890abcdef12345678")
	if err != nil {
		t.Fatalf("normalizeAndValidateSourceAddress() err = %v", err)
	}
	if addr != "0x1234567890abcdef1234567890abcdef12345678" {
		t.Fatalf("normalizeAndValidateSourceAddress() = %q", addr)
	}
}

func TestNormalizeAndValidateSourceAddressLowercasesEVM(t *testing.T) {
	addr, err := normalizeAndValidateSourceAddress("base", "0x1234567890ABCDEF1234567890ABCDEF12345678")
	if err != nil {
		t.Fatalf("normalizeAndValidateSourceAddress() err = %v", err)
	}
	if addr != "0x1234567890abcdef1234567890abcdef12345678" {
		t.Fatalf("normalizeAndValidateSourceAddress() = %q", addr)
	}
}

func TestNormalizeAndValidateSourceAddressRejectsNonEVM(t *testing.T) {
	if _, err := normalizeAndValidateSourceAddress("base", "not-an-evm-address"); err == nil {
		t.Fatalf("normalizeAndValidateSourceAddress() accepted non-EVM address")
	}
}
