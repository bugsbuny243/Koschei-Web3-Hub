package utils

import (
	"strings"
	"testing"
)

func TestHashPasswordUsesArgon2id(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash prefix = %q, want Argon2id", hash)
	}
	if !IsArgon2Hash(hash) {
		t.Fatal("IsArgon2Hash rejected an Argon2id hash")
	}
	if !ComparePassword(hash, "correct horse battery staple") {
		t.Fatal("ComparePassword rejected the correct password")
	}
	if ComparePassword(hash, "wrong password") {
		t.Fatal("ComparePassword accepted a wrong password")
	}
}

func TestComparePasswordRejectsLegacyFastHash(t *testing.T) {
	legacy := "$koschei-sha256$c2FsdA$ZGlnaWVzdA"
	if ComparePassword(legacy, "password") {
		t.Fatal("legacy SHA-256 password hash must not authenticate")
	}
	if IsArgon2Hash(legacy) {
		t.Fatal("legacy SHA-256 password hash must not be reported as Argon2id")
	}
}
