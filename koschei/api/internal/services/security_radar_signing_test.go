package services

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestSecurityRadarVerdictAuthenticationUsesEd25519(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "arm-test-key")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, base64.RawURLEncoding.EncodeToString(seed))

	verdict := finalizeSecurityRadarVerdictAuthentication(SecurityRadarVerdict{
		ModuleID:         ModuleHolderConcentration,
		Target:           "target",
		Network:          "solana-mainnet",
		Grade:            "-",
		RiskIndex:        0,
		RiskLevel:        "evidence_only",
		Verdict:          "Evidence collected.",
		Recommendation:   "evaluate_unified_rules",
		Evidence:         []string{"verified holder evidence"},
		RuleVersion:      SecurityRadarRuleVersion,
		EvidenceVerified: true,
	})
	if !verdict.Signed {
		t.Fatalf("expected authenticated verdict: %#v", verdict)
	}
	if verdict.SignatureAlgorithm != "ed25519" || verdict.KeyID != "arm-test-key" {
		t.Fatalf("unexpected auth metadata: %#v", verdict)
	}
	rawSignature, err := base64.RawURLEncoding.DecodeString(verdict.Signature)
	if err != nil || len(rawSignature) != ed25519.SignatureSize {
		t.Fatalf("invalid Ed25519 signature: len=%d err=%v", len(rawSignature), err)
	}
	payload, err := securityRadarVerdictSigningBytesV1(verdict)
	if err != nil {
		t.Fatal(err)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	if !ed25519.Verify(privateKey.Public().(ed25519.PublicKey), payload, rawSignature) {
		t.Fatal("arm verdict signature did not verify")
	}
	sum := sha256.Sum256(payload)
	if verdict.PayloadHash != "sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatalf("payload hash=%q", verdict.PayloadHash)
	}
	if verdict.Digest != securityRadarVerdictDigestPrefix+hex.EncodeToString(sum[:]) {
		t.Fatalf("digest=%q", verdict.Digest)
	}
}

func TestSecurityRadarVerdictAuthenticationFailsClosedWithoutSigner(t *testing.T) {
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, "")

	verdict := finalizeSecurityRadarVerdictAuthentication(SecurityRadarVerdict{
		ModuleID:       ModuleHolderConcentration,
		Target:         "target",
		Network:        "solana-mainnet",
		RiskLevel:      "evidence_only",
		Verdict:        "Evidence collected.",
		Recommendation: "evaluate_unified_rules",
		Evidence:       []string{"holder evidence"},
		RuleVersion:    SecurityRadarRuleVersion,
	})
	if verdict.Signed || verdict.Signature != "" || verdict.SignatureAlgorithm != "" || verdict.KeyID != "" || verdict.PayloadHash != "" {
		t.Fatalf("unsigned verdict leaked authentication metadata: %#v", verdict)
	}
	if verdict.Digest == "" {
		t.Fatal("unsigned verdict must retain deterministic digest")
	}
}

func TestSecurityRadarVerdictAuthenticationBindsEvidence(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(31 - i)
	}
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "arm-test-key")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, base64.RawURLEncoding.EncodeToString(seed))

	base := SecurityRadarVerdict{
		ModuleID:         ModuleHolderConcentration,
		Target:           "target",
		Network:          "solana-mainnet",
		RiskLevel:        "evidence_only",
		Verdict:          "Evidence collected.",
		Recommendation:   "evaluate_unified_rules",
		Evidence:         []string{"first evidence"},
		RuleVersion:      SecurityRadarRuleVersion,
		EvidenceVerified: true,
	}
	first := finalizeSecurityRadarVerdictAuthentication(base)
	base.Evidence = []string{"different evidence"}
	second := finalizeSecurityRadarVerdictAuthentication(base)
	if first.Signature == second.Signature || first.PayloadHash == second.PayloadHash {
		t.Fatal("evidence mutation must change authenticated payload")
	}
}
