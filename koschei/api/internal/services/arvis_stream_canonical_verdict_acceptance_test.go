package services

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func arvisStreamCanonicalAcceptanceFixture(t *testing.T, referenced, verified bool) (arvisStreamTarget, UnifiedRadarVerdict, GlobalRadarTrustedVerdictRegistry) {
	t.Helper()
	t.Setenv("KOSCHEI_VERDICT_MODE", "")
	key := unifiedVerdictTestPrivateKey(0x61)
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "arvis-stream-test-trusted-key")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, base64.RawURLEncoding.EncodeToString(key))

	event := arvisStreamTarget{
		ID: "test-stream-event", Target: "So11111111111111111111111111111111111111112", Network: "solana-mainnet",
		Signature: "verified-tx-sig-123", ModuleID: ModulePumpSybilRadar,
	}
	evidenceStatus := "verified"
	if !verified {
		evidenceStatus = "observed"
	}
	firstSignature := "other-tx-sig-123"
	if referenced {
		firstSignature = event.Signature
	}
	verdict := FinalizeUnifiedRadarVerdictContractForNetwork(
		event.Target, event.Network, UnifiedRadarVerdict{
			RulesetVersion: UnifiedRadarRulesetVersionV140,
			TriggeredRules: []ActorDefenseRuleHit{
				{
					RuleID: ActorRuleCompoundCreatorReuse, Title: "Creator reuse",
					Tier: "compounding", EvidenceStatus: evidenceStatus,
					GradeEffect: "compounding_input", Summary: "fixture source observation",
					Signatures: []string{firstSignature}, EvidenceKeys: []string{"fixture:creator"},
				},
				{
					RuleID: ActorRuleCompoundHolderReuse, Title: "Holder reuse",
					Tier: "compounding", EvidenceStatus: "verified",
					GradeEffect: "compounding_input", Summary: "independent fixture evidence",
					Signatures: []string{"other-tx-sig-2"}, EvidenceKeys: []string{"fixture:holder"},
				},
			},
			GeneratedAt: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC),
		},
	)
	if !verdict.Signed {
		t.Fatal("test signer failed to create canonical fixture")
	}
	registry, err := NewGlobalRadarTrustedVerdictRegistry([]GlobalRadarTrustedVerdictKey{{
		KeyID:              verdict.KeyID,
		PublicKeyBase64URL: base64.RawURLEncoding.EncodeToString(key.Public().(ed25519.PublicKey)),
		ValidFrom:          verdict.GeneratedAt.Add(-time.Hour),
		ValidUntil:         verdict.GeneratedAt.Add(time.Hour),
	}})
	if err != nil {
		t.Fatal(err)
	}
	return event, verdict, registry
}

func TestArvisStreamCanonicalAcceptanceRequiresTrustedSignedBoundEvidence(t *testing.T) {
	event, verdict, registry := arvisStreamCanonicalAcceptanceFixture(t, true, true)
	if err := validateArvisStreamCanonicalVerdict(event, verdict, registry); err != nil {
		t.Fatalf("valid signed, cited canonical verdict rejected: %v", err)
	}

	crossTarget := event
	crossTarget.Target = "OtherMintEvidence123"
	if err := validateArvisStreamCanonicalVerdict(crossTarget, verdict, registry); err == nil || !strings.Contains(err.Error(), "target/network") {
		t.Fatalf("cross target unexpectedly accepted: %v", err)
	}
	crossNetwork := event
	crossNetwork.Network = "solana-devnet"
	if err := validateArvisStreamCanonicalVerdict(crossNetwork, verdict, registry); err == nil {
		t.Fatalf("cross network unexpectedly accepted: %v", err)
	}
	otherSignature := event
	otherSignature.Signature = "unknown-transaction"
	if err := validateArvisStreamCanonicalVerdict(otherSignature, verdict, registry); err == nil || !strings.Contains(err.Error(), "not cited") {
		t.Fatalf("uncited stream event unexpectedly accepted: %v", err)
	}
	unsupportedSource := event
	unsupportedSource.ModuleID = ModuleMEVShield
	if err := validateArvisStreamCanonicalVerdict(unsupportedSource, verdict, registry); err == nil {
		t.Fatalf("unsupported source unexpectedly accepted: %v", err)
	}
}

func TestArvisStreamCanonicalAcceptanceRejectsInvalidAuthorityOrMissingVerifiedCitation(t *testing.T) {
	event, verdict, registry := arvisStreamCanonicalAcceptanceFixture(t, true, true)
	tampered := verdict
	tampered.Grade = "A"
	if err := validateArvisStreamCanonicalVerdict(event, tampered, registry); err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("tampered signed decision unexpectedly accepted: %v", err)
	}
	unsigned := verdict
	unsigned.Signed = false
	if err := validateArvisStreamCanonicalVerdict(event, unsigned, registry); err == nil {
		t.Fatalf("unsigned decision unexpectedly accepted: %v", err)
	}
	outdated := verdict
	outdated.RulesetVersion = UnifiedRadarRulesetVersionV130
	if err := validateArvisStreamCanonicalVerdict(event, outdated, registry); err == nil {
		t.Fatalf("outdated ruleset unexpectedly accepted: %v", err)
	}
	unknownRegistry, _ := NewGlobalRadarTrustedVerdictRegistry(nil)
	if err := validateArvisStreamCanonicalVerdict(event, verdict, unknownRegistry); err == nil {
		t.Fatal("untrusted signing key unexpectedly accepted")
	}

	event, verdict, registry = arvisStreamCanonicalAcceptanceFixture(t, false, true)
	if err := validateArvisStreamCanonicalVerdict(event, verdict, registry); err == nil || !strings.Contains(err.Error(), "not cited") {
		t.Fatalf("unrelated signed verdict unexpectedly accepted: %v", err)
	}

	event, verdict, registry = arvisStreamCanonicalAcceptanceFixture(t, true, false)
	if err := validateArvisStreamCanonicalVerdict(event, verdict, registry); err == nil || !strings.Contains(err.Error(), "not cited") {
		t.Fatalf("observed-only source citation unexpectedly accepted: %v", err)
	}
}
