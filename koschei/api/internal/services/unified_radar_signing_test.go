package services

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestFinalizeUnifiedVerdictUsesRealEd25519WhenConfigured(t *testing.T) {
	privateKey := unifiedVerdictTestPrivateKey(0x51)
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "test-verdict-key-v1")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, base64.RawURLEncoding.EncodeToString(privateKey))

	verdict := FinalizeUnifiedRadarVerdictContractForNetwork("MintOne", "solana-mainnet", UnifiedRadarVerdict{
		RulesetVersion: UnifiedRadarRulesetVersion,
		TriggeredRules: []ActorDefenseRuleHit{
			{RuleID: ActorRuleCompoundCreatorReuse, Title: "Creator reuse", Tier: "compounding", EvidenceStatus: "verified", GradeEffect: "compounding_input", Count: 1, Summary: "creator reused", EvidenceKeys: []string{"creator:MintOne"}, Signatures: []string{"sig-1"}},
			{RuleID: ActorRuleCompoundHolderReuse, Title: "Holder reuse", Tier: "compounding", EvidenceStatus: "verified", GradeEffect: "compounding_input", Count: 1, Summary: "holder reused", EvidenceKeys: []string{"holder:MintOne"}, Signatures: []string{"sig-2"}},
		},
	})
	if !verdict.Signed {
		t.Fatal("configured verdict signer did not authenticate finalized verdict")
	}
	if verdict.SignatureAlgorithm != UnifiedVerdictSignatureAlgorithmV1 {
		t.Fatalf("signature algorithm=%q", verdict.SignatureAlgorithm)
	}
	if verdict.KeyID != "test-verdict-key-v1" {
		t.Fatalf("key id=%q", verdict.KeyID)
	}
	if !strings.HasPrefix(verdict.PayloadHash, "sha256:") {
		t.Fatalf("payload hash=%q", verdict.PayloadHash)
	}
	if !strings.HasPrefix(verdict.Digest, "koschei-unified:") {
		t.Fatalf("deterministic digest=%q", verdict.Digest)
	}
	if strings.HasPrefix(verdict.Signature, "koschei-unified:") {
		t.Fatalf("digest leaked into signature field: %q", verdict.Signature)
	}

	payload, err := unifiedVerdictSigningBytesV1(verdict)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	if verdict.PayloadHash != "sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatalf("payload hash mismatch: %q", verdict.PayloadHash)
	}
	signature, err := base64.RawURLEncoding.DecodeString(verdict.Signature)
	if err != nil {
		t.Fatal(err)
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	if !ed25519.Verify(publicKey, payload, signature) {
		t.Fatal("finalized verdict signature did not verify")
	}

	raw, err := json.Marshal(verdict)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if wire["signed"] != true || wire["signature_algorithm"] != "ed25519" || wire["key_id"] != "test-verdict-key-v1" {
		t.Fatalf("authenticated wire fields=%s", raw)
	}
	if wire["target"] != "MintOne" || wire["network"] != "solana-mainnet" {
		t.Fatalf("target/network not bound in wire contract: %s", raw)
	}
}

func TestFinalizeUnifiedVerdictFailsClosedWithoutSigner(t *testing.T) {
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, "")

	verdict := FinalizeUnifiedRadarVerdictContract("MintUnsigned", UnifiedRadarVerdict{
		RulesetVersion: UnifiedRadarRulesetVersion,
		TriggeredRules: []ActorDefenseRuleHit{
			{RuleID: ActorRuleCompoundCreatorReuse, Tier: "compounding", EvidenceStatus: "verified", Summary: "one"},
			{RuleID: ActorRuleCompoundHolderReuse, Tier: "compounding", EvidenceStatus: "verified", Summary: "two"},
		},
	})
	if verdict.Signed || verdict.Signature != "" || verdict.SignatureAlgorithm != "" || verdict.KeyID != "" || verdict.PayloadHash != "" {
		t.Fatalf("unsigned verdict claimed authentication: %#v", verdict)
	}
	if verdict.Digest == "" {
		t.Fatal("unsigned verdict lost deterministic digest identity")
	}
}

func TestFinalizeUnifiedVerdictFailsClosedOnPartialSignerConfiguration(t *testing.T) {
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "missing-private-key")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, "")
	verdict := FinalizeUnifiedRadarVerdictContract("MintPartial", UnifiedRadarVerdict{
		RulesetVersion: UnifiedRadarRulesetVersion,
	})
	if verdict.Signed || verdict.Signature != "" {
		t.Fatalf("partial signer configuration produced signed verdict: %#v", verdict)
	}
}

func TestUnifiedVerdictSignatureBindsTargetAndNetwork(t *testing.T) {
	privateKey := unifiedVerdictTestPrivateKey(0x52)
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "binding-key-v1")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, base64.RawURLEncoding.EncodeToString(privateKey))
	base := UnifiedRadarVerdict{
		RulesetVersion: UnifiedRadarRulesetVersion,
		TriggeredRules: []ActorDefenseRuleHit{
			{RuleID: ActorRuleCompoundCreatorReuse, Tier: "compounding", EvidenceStatus: "verified", Summary: "one"},
			{RuleID: ActorRuleCompoundHolderReuse, Tier: "compounding", EvidenceStatus: "verified", Summary: "two"},
		},
	}
	a := FinalizeUnifiedRadarVerdictContractForNetwork("MintA", "solana-mainnet", base)
	b := FinalizeUnifiedRadarVerdictContractForNetwork("MintB", "solana-mainnet", base)
	c := FinalizeUnifiedRadarVerdictContractForNetwork("MintA", "solana-devnet", base)
	if a.Signature == b.Signature || a.Signature == c.Signature {
		t.Fatal("signature did not bind target and network")
	}
}

func unifiedVerdictTestPrivateKey(fill byte) ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(bytes.Repeat([]byte{fill}, ed25519.SeedSize))
}
