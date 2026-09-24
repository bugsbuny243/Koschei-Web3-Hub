package services

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func signedGlobalRadarVerdictFixture(t *testing.T) (IntelligenceSubject, UnifiedRadarVerdict, IntelligenceDecision, GlobalRadarTrustedVerdictRegistry) {
	t.Helper()
	subject := ClassifyIntelligenceSubject("So11111111111111111111111111111111111111112", "solana-mainnet")
	subject.Kind = IntelligenceSubjectToken
	verdict, decision := arvisVerdictReferenceFixture(subject)
	verdict.TriggeredRules = []ActorDefenseRuleHit{{
		RuleID:         "ARD-C001",
		EvidenceStatus: "verified",
		Title:          "Creator reuse",
		Summary:        "Verified deterministic evidence.",
	}}
	verdict.DecisionPath = []string{"ARD-C001 verified evidence triggered the deterministic verdict."}

	seed := bytes.Repeat([]byte{0x42}, ed25519.SeedSize)
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	verdict.KeyID = "verdict-key-2026-01"
	payload, err := unifiedVerdictSigningBytesV1(verdict)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	verdict.PayloadHash = "sha256:" + hex.EncodeToString(sum[:])
	verdict.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, payload))

	registry, err := NewGlobalRadarTrustedVerdictRegistry([]GlobalRadarTrustedVerdictKey{{
		KeyID:              verdict.KeyID,
		PublicKeyBase64URL: base64.RawURLEncoding.EncodeToString(publicKey),
		ValidFrom:          verdict.GeneratedAt.Add(-time.Hour),
		ValidUntil:         verdict.GeneratedAt.Add(time.Hour),
	}})
	if err != nil {
		t.Fatal(err)
	}
	return subject, verdict, decision, registry
}

func TestProjectVerifiedARVISSignedVerdictToGlobalRadarReverifiesSignature(t *testing.T) {
	subject, verdict, decision, registry := signedGlobalRadarVerdictFixture(t)
	got, err := ProjectVerifiedARVISSignedVerdictToGlobalRadar(subject, verdict, decision, registry)
	if err != nil {
		t.Fatal(err)
	}
	if got.SignatureVerification != GlobalRadarVerifiedSignatureState {
		t.Fatalf("signature verification=%q", got.SignatureVerification)
	}
	if got.ProjectionState != "authoritative_reference_signature_verified" ||
		got.TrustBoundary != "signature_verified_against_server_owned_trusted_key_registry" {
		t.Fatalf("verified trust boundary missing: %#v", got)
	}
}

func TestVerifyUnifiedRadarVerdictWithTrustedRegistryRejectsTampering(t *testing.T) {
	_, verdict, _, registry := signedGlobalRadarVerdictFixture(t)
	verdict.Grade = "A"
	if err := VerifyUnifiedRadarVerdictWithTrustedRegistry(verdict, registry); err == nil ||
		!strings.Contains(err.Error(), "payload_hash") {
		t.Fatalf("tampered verdict err=%v", err)
	}
}

func TestTrustedVerdictRegistryRejectsUnknownExpiredAndRevokedKeys(t *testing.T) {
	_, verdict, _, registry := signedGlobalRadarVerdictFixture(t)
	verdict.KeyID = "unknown"
	if err := VerifyUnifiedRadarVerdictWithTrustedRegistry(verdict, registry); err == nil {
		t.Fatal("unknown key was accepted")
	}

	_, verdict, _, _ = signedGlobalRadarVerdictFixture(t)
	seed := bytes.Repeat([]byte{0x42}, ed25519.SeedSize)
	publicKey := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)

	expired, err := NewGlobalRadarTrustedVerdictRegistry([]GlobalRadarTrustedVerdictKey{{
		KeyID:              verdict.KeyID,
		PublicKeyBase64URL: base64.RawURLEncoding.EncodeToString(publicKey),
		ValidUntil:         verdict.GeneratedAt,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyUnifiedRadarVerdictWithTrustedRegistry(verdict, expired); err == nil ||
		!strings.Contains(err.Error(), "expired") {
		t.Fatalf("expired key err=%v", err)
	}

	revoked, err := NewGlobalRadarTrustedVerdictRegistry([]GlobalRadarTrustedVerdictKey{{
		KeyID:              verdict.KeyID,
		PublicKeyBase64URL: base64.RawURLEncoding.EncodeToString(publicKey),
		RevokedAt:          verdict.GeneratedAt,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyUnifiedRadarVerdictWithTrustedRegistry(verdict, revoked); err == nil ||
		!strings.Contains(err.Error(), "revoked") {
		t.Fatalf("revoked key err=%v", err)
	}
}

func TestTrustedVerdictRegistryRequiresCanonicalPublicKeyEncoding(t *testing.T) {
	publicKey := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x11}, ed25519.PublicKeySize))
	if _, err := NewGlobalRadarTrustedVerdictRegistry([]GlobalRadarTrustedVerdictKey{{
		KeyID:              "key-1",
		PublicKeyBase64URL: publicKey + "=",
	}}); err == nil {
		t.Fatal("padded trusted public key was accepted")
	}
}
