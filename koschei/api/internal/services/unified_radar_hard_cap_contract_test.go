package services

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUnifiedContractPreservesVerifiedHardCapGradeEffect(t *testing.T) {
	configureUnifiedVerdictTestSigner(t)
	raw := UnifiedRadarVerdict{
		Target:         "MintHardCap",
		Network:        "solana-mainnet",
		Grade:          "B",
		Verdict:        "compounding_rule",
		RulesetVersion: UnifiedRadarRulesetVersionV110,
		ActorRuleset:   ActorDefenseRulesetVersion,
		TriggeredRules: []ActorDefenseRuleHit{
			{
				RuleID:         ActorRuleCompoundRepeatedTransfer,
				Title:          "Repeated transfer group one",
				Tier:           "compounding",
				EvidenceStatus: "verified",
				GradeEffect:    "compounding_input",
				EvidenceKeys:   []string{"group:one"},
			},
			{
				RuleID:         ActorRuleCompoundRepeatedTransfer,
				Title:          "Repeated transfer group two",
				Tier:           "compounding",
				EvidenceStatus: "verified",
				GradeEffect:    "compounding_input",
				EvidenceKeys:   []string{"group:two"},
			},
			{
				RuleID:         UnifiedRuleOwnerConcentration,
				Title:          "Owner-resolved dominant concentration",
				Tier:           "compounding",
				EvidenceStatus: "verified",
				GradeEffect:    "hard_cap_F",
				EvidenceKeys:   []string{"owner:dominant"},
			},
		},
		Signed:    true,
		Signature: "koschei-unified:stale-b",
	}

	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal hard-cap verdict: %v", err)
	}
	var contract map[string]any
	if err := json.Unmarshal(encoded, &contract); err != nil {
		t.Fatalf("decode hard-cap contract: %v", err)
	}
	if contract["grade"] != "F" || contract["verdict"] != "hard_trigger" {
		t.Fatalf("hard cap was rewritten during serialization: %s", encoded)
	}
	if contract["signed"] != true || contract["signature_algorithm"] != "ed25519" || contract["key_id"] != "test-suite-verdict-key-v1" {
		t.Fatalf("hard-cap verdict was not authenticated with Ed25519: %s", encoded)
	}
	if signature, _ := contract["signature"].(string); signature == "koschei-unified:stale-b" || strings.HasPrefix(signature, "koschei-unified-contract:") {
		t.Fatalf("changed hard-cap decision retained legacy digest semantics: %q", signature)
	}
	if payloadHash, _ := contract["payload_hash"].(string); !strings.HasPrefix(payloadHash, "sha256:") {
		t.Fatalf("hard-cap payload hash missing: %q", payloadHash)
	}
	decision, _ := contract["decision_path"].([]any)
	foundHardCap := false
	for _, item := range decision {
		if strings.Contains(strings.ToLower(item.(string)), "hard-trigger ceiling applied: grade f") {
			foundHardCap = true
			break
		}
	}
	if !foundHardCap {
		t.Fatalf("hard-cap decision path missing: %#v", decision)
	}
}

func TestFinalizeUnifiedContractPreservesV120HardCapAndRuleset(t *testing.T) {
	configureUnifiedVerdictTestSigner(t)
	raw := UnifiedRadarVerdict{
		Grade:          "D",
		Verdict:        "hard_trigger",
		RulesetVersion: UnifiedRadarRulesetVersionV120,
		ActorRuleset:   ActorDefenseRulesetVersion,
		TriggeredRules: []ActorDefenseRuleHit{
			{
				RuleID:         UnifiedRuleCrossTokenCreatorHolderTransfer,
				Title:          "Cross-token creator to dominant-holder transfer",
				Tier:           "compounding",
				EvidenceStatus: "verified",
				GradeEffect:    "hard_cap_D",
				EvidenceKeys:   []string{"creator-holder-transfer:signature"},
				Signatures:     []string{"signature"},
			},
		},
	}

	finalized := FinalizeUnifiedRadarVerdictContract("MintV120", raw)
	if finalized.Grade != "D" || finalized.Verdict != "hard_trigger" {
		t.Fatalf("v1.2 hard cap changed: %#v", finalized)
	}
	if finalized.RulesetVersion != UnifiedRadarRulesetVersionV120 {
		t.Fatalf("v1.2 ruleset downgraded: %q", finalized.RulesetVersion)
	}
	if !finalized.Signed || finalized.SignatureAlgorithm != "ed25519" || finalized.KeyID != "test-suite-verdict-key-v1" || !strings.HasPrefix(finalized.PayloadHash, "sha256:") {
		t.Fatalf("v1.2 verdict was not cryptographically target-bound: %#v", finalized)
	}
}
