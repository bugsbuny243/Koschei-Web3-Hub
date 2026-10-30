package services

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

func TestVerifyUnifiedRadarVerdictWithCurrentSigner(t *testing.T) {
	private := unifiedVerdictTestPrivateKey(0x72)
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "customer-ready-key")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, base64.RawURLEncoding.EncodeToString(private))
	verdict := FinalizeUnifiedRadarVerdictContractForNetwork("MintReady", "solana-mainnet", UnifiedRadarVerdict{
		RulesetVersion: UnifiedRadarRulesetVersion,
		ActorRuleset:   ActorDefenseRulesetVersion,
		TriggeredRules: []ActorDefenseRuleHit{{
			RuleID: ActorRuleCompoundCreatorReuse, Tier: "compounding",
			EvidenceStatus: "verified", Summary: "verified evidence",
		}},
	})
	if !verdict.Signed {
		t.Fatal("configured signer failed to produce fixture")
	}
	if err := VerifyUnifiedRadarVerdictWithCurrentSigner(verdict); err != nil {
		t.Fatalf("real signed verdict not verified: %v", err)
	}
	cases := map[string]func(UnifiedRadarVerdict) UnifiedRadarVerdict{
		"fake signed flag": func(v UnifiedRadarVerdict) UnifiedRadarVerdict { v.Signature = ""; return v },
		"wrong target":     func(v UnifiedRadarVerdict) UnifiedRadarVerdict { v.Target = "OtherMint"; return v },
		"wrong network":    func(v UnifiedRadarVerdict) UnifiedRadarVerdict { v.Network = "solana-devnet"; return v },
		"altered rule facts": func(v UnifiedRadarVerdict) UnifiedRadarVerdict {
			v.TriggeredRules = append([]ActorDefenseRuleHit(nil), v.TriggeredRules...)
			v.TriggeredRules[0].Summary = "altered fact"
			return v
		},
		"wrong signature": func(v UnifiedRadarVerdict) UnifiedRadarVerdict { v.Signature = "not-ed25519"; return v },
		"wrong key ID":    func(v UnifiedRadarVerdict) UnifiedRadarVerdict { v.KeyID = "unknown"; return v },
		"unsigned":        func(v UnifiedRadarVerdict) UnifiedRadarVerdict { v.Signed = false; return v },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			if err := VerifyUnifiedRadarVerdictWithCurrentSigner(mutate(verdict)); err == nil {
				t.Fatal("malformed or tampered final passed live cryptographic gate")
			}
		})
	}
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, "")
	if err := VerifyUnifiedRadarVerdictWithCurrentSigner(verdict); err == nil {
		t.Fatal("missing signer must fail closed")
	}
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "customer-ready-key")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, base64.RawURLEncoding.EncodeToString(ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))))
	if err := VerifyUnifiedRadarVerdictWithCurrentSigner(verdict); err == nil {
		t.Fatal("different public key must fail closed")
	}
}
