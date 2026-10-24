package services

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestActiveVerdictVerificationKeyPublishesOnlyThePublicKey(t *testing.T) {
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, "")
	if _, configured, err := ActiveVerdictVerificationKey(); configured || err != nil {
		t.Fatalf("unconfigured signer: configured=%v err=%v", configured, err)
	}

	key := unifiedVerdictTestPrivateKey(0x41)
	seed := base64.RawURLEncoding.EncodeToString(key.Seed())
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "published-test-key")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, seed)
	published, configured, err := ActiveVerdictVerificationKey()
	if !configured || err != nil {
		t.Fatalf("configured signer: configured=%v err=%v", configured, err)
	}
	want := base64.RawURLEncoding.EncodeToString(key.Public().(ed25519.PublicKey))
	if published.KeyID != "published-test-key" || published.PublicKeyBase64URL != want || published.Algorithm != "ed25519" {
		t.Fatalf("unexpected published key: %#v", published)
	}
	encoded, _ := json.Marshal(published)
	if strings.Contains(string(encoded), seed) || strings.Contains(string(encoded), base64.RawURLEncoding.EncodeToString(key)) {
		t.Fatal("published verification key leaked private key material")
	}

	// Pasting the published key into the stream trusted registry makes the
	// signed-final path ready.
	t.Setenv(arvisStreamTrustedKeysEnv, `[{"key_id":"published-test-key","public_key_base64url":"`+published.PublicKeyBase64URL+`"}]`)
	registry, registryConfigured, err := arvisStreamRecoveryTrustRegistry()
	if err != nil || !registryConfigured {
		t.Fatalf("published key must be accepted by the trusted registry: configured=%v err=%v", registryConfigured, err)
	}
	if trusted, err := registry.Resolve("published-test-key", time.Now().UTC()); err != nil || !trusted.Equal(key.Public().(ed25519.PublicKey)) {
		t.Fatalf("registry did not resolve the published key: %v", err)
	}
	t.Setenv("SOLANA_RPC_URL", "https://rpc.example.invalid")
	t.Setenv(arvisStreamCanonicalJobsEnv, "true")
	if ready, reason := ArvisStreamCanonicalJobsReady(); !ready {
		t.Fatalf("published key did not make stream signed finals ready: %s", reason)
	}
}

func TestActiveVerdictVerificationKeyHidesInvalidKeyDetails(t *testing.T) {
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "broken-key")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, "c2hvcnQ")
	if _, configured, err := ActiveVerdictVerificationKey(); !configured || err == nil || err.Error() != "verdict signer configuration is invalid" {
		t.Fatalf("invalid signer must fail with a generic error: configured=%v err=%v", configured, err)
	}
}
