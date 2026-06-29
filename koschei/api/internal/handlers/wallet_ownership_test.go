package handlers

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"math/big"
	"testing"
)

func TestDecodeSolanaPublicKey(t *testing.T) {
	decoded, err := decodeSolanaPublicKey("11111111111111111111111111111111")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		t.Fatalf("len = %d", len(decoded))
	}
	for _, value := range decoded {
		if value != 0 {
			t.Fatalf("system program key should decode to zeros")
		}
	}
}

func TestWalletSignatureRoundTrip(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	address := encodeBase58ForTest(publicKey)
	decodedPublicKey, err := decodeSolanaPublicKey(address)
	if err != nil {
		t.Fatalf("decode address: %v", err)
	}
	message := []byte("Koschei wallet verification test")
	signature := ed25519.Sign(privateKey, message)
	encodedSignature := base64.StdEncoding.EncodeToString(signature)
	decodedSignature, err := decodeWalletSignature(encodedSignature)
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	if !ed25519.Verify(ed25519.PublicKey(decodedPublicKey), message, decodedSignature) {
		t.Fatal("signature did not verify")
	}
}

func TestNormalizeWalletNetwork(t *testing.T) {
	if got, ok := normalizeWalletNetwork("mainnet-beta"); !ok || got != "solana-mainnet" {
		t.Fatalf("mainnet = %q, %v", got, ok)
	}
	if _, ok := normalizeWalletNetwork("ethereum"); ok {
		t.Fatal("unsupported network accepted")
	}
}

func encodeBase58ForTest(data []byte) string {
	value := new(big.Int).SetBytes(data)
	base := big.NewInt(58)
	zero := big.NewInt(0)
	mod := new(big.Int)
	encoded := make([]byte, 0, 44)
	for value.Cmp(zero) > 0 {
		value.DivMod(value, base, mod)
		encoded = append(encoded, solanaBase58Alphabet[mod.Int64()])
	}
	for _, item := range data {
		if item != 0 {
			break
		}
		encoded = append(encoded, '1')
	}
	for left, right := 0, len(encoded)-1; left < right; left, right = left+1, right-1 {
		encoded[left], encoded[right] = encoded[right], encoded[left]
	}
	return string(encoded)
}
