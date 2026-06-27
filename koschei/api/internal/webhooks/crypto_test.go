package webhooks

import "testing"

func TestGenerateEncryptDecryptSecret(t *testing.T) {
	t.Setenv("USER_SESSION_SECRET", "unit-test-session-secret")
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(secret) < 20 || secret[:6] != "whsec_" {
		t.Fatalf("unexpected secret format: %q", secret)
	}
	ciphertext, err := EncryptSecret(secret)
	if err != nil {
		t.Fatal(err)
	}
	if ciphertext == secret {
		t.Fatal("secret must not be stored in plaintext")
	}
	plain, err := DecryptSecret(ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if plain != secret {
		t.Fatalf("decrypt mismatch: got %q want %q", plain, secret)
	}
}

func TestSignatureVerification(t *testing.T) {
	secret := "whsec_test"
	timestamp := "1782561600"
	payload := []byte(`{"type":"watchlist.alert.created"}`)
	signature := Signature(secret, timestamp, payload)
	if !VerifySignature(secret, timestamp, payload, signature) {
		t.Fatal("valid signature did not verify")
	}
	if VerifySignature(secret, timestamp, []byte(`{"changed":true}`), signature) {
		t.Fatal("modified payload verified")
	}
	if VerifySignature("other-secret", timestamp, payload, signature) {
		t.Fatal("wrong secret verified")
	}
}

func TestLast4(t *testing.T) {
	if got := Last4("whsec_123456"); got != "3456" {
		t.Fatalf("Last4 = %q", got)
	}
}
