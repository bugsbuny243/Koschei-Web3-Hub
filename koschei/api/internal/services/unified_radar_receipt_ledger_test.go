package services

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestImmutableSignedReceiptIndependentHistoricalVerification(t *testing.T) {
	private := unifiedVerdictTestPrivateKey(0x75)
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "historic-2026-key")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, base64.RawURLEncoding.EncodeToString(private))
	final := FinalizeUnifiedRadarVerdictContractForNetwork("MintLedger", "solana-mainnet", UnifiedRadarVerdict{
		RulesetVersion: UnifiedRadarRulesetVersion,
		ActorRuleset:   ActorDefenseRulesetVersion,
		TriggeredRules: []ActorDefenseRuleHit{{
			RuleID: ActorRuleCompoundCreatorReuse, Tier: "compounding",
			EvidenceStatus: "verified", Summary: "creator evidence",
		}},
	})
	if !final.Signed {
		t.Fatal("test signer did not produce a signed verdict")
	}
	payload, err := unifiedVerdictSigningBytesV1(final)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewGlobalRadarTrustedVerdictRegistry([]GlobalRadarTrustedVerdictKey{{
		KeyID:              final.KeyID,
		PublicKeyBase64URL: base64.RawURLEncoding.EncodeToString(private.Public().(ed25519.PublicKey)),
	}})
	if err != nil {
		t.Fatal(err)
	}
	// Verification must work after the signing private key is rotated away.
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "rotated")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, base64.RawURLEncoding.EncodeToString(unifiedVerdictTestPrivateKey(0x76)))
	identity, err := VerifyUnifiedRadarSignedReceipt(payload, final.Signature, final.PayloadHash, final.KeyID, registry)
	if err != nil || identity.Target != "MintLedger" || identity.Network != "solana-mainnet" {
		t.Fatalf("independent historical verification failed: identity=%+v err=%v", identity, err)
	}
	if _, err := VerifyUnifiedRadarSignedReceipt(payload, final.Signature, final.PayloadHash, "untrusted", registry); err == nil {
		t.Fatal("unknown trusted key was accepted")
	}
	empty, _ := NewGlobalRadarTrustedVerdictRegistry(nil)
	if _, err := VerifyUnifiedRadarSignedReceipt(payload, final.Signature, final.PayloadHash, final.KeyID, empty); err == nil {
		t.Fatal("stored signature passed without independent trust")
	}
	if _, err := VerifyUnifiedRadarSignedReceipt(append(append([]byte(nil), payload...), ' '), final.Signature, final.PayloadHash, final.KeyID, registry); err == nil {
		t.Fatal("noncanonical signed payload passed")
	}
	modified := append([]byte(nil), payload...)
	for i := range modified {
		if i+len("MintLedger") <= len(modified) && string(modified[i:i+len("MintLedger")]) == "MintLedger" {
			copy(modified[i:], "OtherMint!")
			break
		}
	}
	sum := sha256.Sum256(modified)
	if _, err := VerifyUnifiedRadarSignedReceipt(modified, final.Signature, "sha256:"+hex.EncodeToString(sum[:]), final.KeyID, registry); err == nil {
		t.Fatal("tampered payload and matching new hash passed original signature")
	}
	if _, err := VerifyUnifiedRadarSignedReceipt(payload, final.Signature, "sha256:"+hex.EncodeToString(make([]byte, 32)), final.KeyID, registry); err == nil {
		t.Fatal("wrong payload hash passed verification")
	}
}
