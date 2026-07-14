package services

import (
	"testing"
	"time"
)

func TestActorRulesVerifiedCreatorLiquidityRemovalCapsAtD(t *testing.T) {
	track := ActorDefenseTrack{
		Network: "solana-mainnet", TargetKind: "wallet", TargetID: "ActorWallet",
		State: "correlated", CreatedTokenCount: 2, DominantHolderTokenCount: 2,
	}
	evidence := []ActorDefenseEvidenceRecord{{
		Relation: "liquidity_remove_activity", VerificationStatus: "verified",
		EvidenceKey: "sig-one:remove", Signature: "sig-one", ObservedAt: time.Unix(1700000000, 0).UTC(),
		Metadata: map[string]any{"actor_signed": true},
	}}
	verdict := EvaluateActorDefenseRules(track, evidence)
	if verdict.Grade != "D" || verdict.Verdict != "hard_trigger" {
		t.Fatalf("verdict=%#v", verdict)
	}
	if !verdict.Signed || verdict.Signature == "" {
		t.Fatal("deterministic hard-trigger verdict must be signed")
	}
	if !actorRulePresent(verdict.TriggeredRules, ActorRuleHardCreatorLiquidityRemoval) {
		t.Fatalf("missing %s", ActorRuleHardCreatorLiquidityRemoval)
	}
	if !actorRulePresent(verdict.TriggeredRules, ActorRuleCompoundCreatorReuse) || !actorRulePresent(verdict.TriggeredRules, ActorRuleCompoundHolderReuse) {
		t.Fatal("supporting compounding rules must remain visible")
	}
}

func TestActorRulesVerifiedCreatorHolderFundingCapsAtD(t *testing.T) {
	track := ActorDefenseTrack{
		Network: "solana-mainnet", TargetKind: "wallet", TargetID: "ActorWallet",
		CreatedTokenCount: 2,
	}
	evidence := []ActorDefenseEvidenceRecord{{
		Relation: "direct_sol_transfer_out", VerificationStatus: "verified",
		EvidenceKey: "sig-two:0", Signature: "sig-two", CounterpartKind: "wallet", CounterpartID: "HolderWallet",
		Metadata: map[string]any{"actor_signed": true, "known_related_actor": true},
	}}
	verdict := EvaluateActorDefenseRules(track, evidence)
	if verdict.Grade != "D" || !actorRulePresent(verdict.TriggeredRules, ActorRuleHardCreatorHolderFunding) {
		t.Fatalf("verdict=%#v", verdict)
	}
}

func TestActorRulesPreviousTokenIncidentCapsAtC(t *testing.T) {
	track := ActorDefenseTrack{Network: "solana-mainnet", TargetKind: "wallet", TargetID: "ActorWallet"}
	evidence := []ActorDefenseEvidenceRecord{{
		Relation: "prior_token_liquidity_removal", VerificationStatus: "verified",
		EvidenceKey: "old-token-sig:0", Signature: "old-token-sig",
	}}
	verdict := EvaluateActorDefenseRules(track, evidence)
	if verdict.Grade != "C" || verdict.Verdict != "hard_trigger" {
		t.Fatalf("verdict=%#v", verdict)
	}
}

func TestActorRulesTwoObservedCompoundingRulesProduceB(t *testing.T) {
	track := ActorDefenseTrack{
		Network: "solana-mainnet", TargetKind: "wallet", TargetID: "ActorWallet",
		State: "correlated", CreatedTokenCount: 2, DominantHolderTokenCount: 2,
	}
	verdict := EvaluateActorDefenseRules(track, nil)
	if verdict.Grade != "B" || verdict.Verdict != "compounding_rule" {
		t.Fatalf("verdict=%#v", verdict)
	}
	if !verdict.Signed {
		t.Fatal("deterministic compounding verdict must be signed")
	}
}

func TestActorRulesSingleObservationDoesNotIssueGrade(t *testing.T) {
	track := ActorDefenseTrack{CreatedTokenCount: 2}
	verdict := EvaluateActorDefenseRules(track, nil)
	if verdict.Grade != "-" || verdict.Verdict != "single_observation" || verdict.Signed {
		t.Fatalf("verdict=%#v", verdict)
	}
}

func TestActorRulesInferredIsWatchOnly(t *testing.T) {
	track := ActorDefenseTrack{Network: "solana-mainnet", TargetKind: "wallet", TargetID: "ActorWallet"}
	evidence := []ActorDefenseEvidenceRecord{{
		Relation: "possible_shared_funder", VerificationStatus: "inferred", EvidenceKey: "inferred-one",
	}}
	verdict := EvaluateActorDefenseRules(track, evidence)
	if verdict.Grade != "-" || verdict.Verdict != "watch_only" || verdict.Signed {
		t.Fatalf("verdict=%#v", verdict)
	}
	if len(verdict.WatchFlags) != 1 || verdict.WatchFlags[0].EvidenceStatus != "inferred" {
		t.Fatalf("watch_flags=%#v", verdict.WatchFlags)
	}
}

func TestActorRulesUnverifiedIsExcluded(t *testing.T) {
	track := ActorDefenseTrack{Network: "solana-mainnet", TargetKind: "wallet", TargetID: "ActorWallet"}
	evidence := []ActorDefenseEvidenceRecord{{
		Relation: "direct_sol_transfer_out", VerificationStatus: "unverified", EvidenceKey: "unverified-one",
	}}
	verdict := EvaluateActorDefenseRules(track, evidence)
	if verdict.Grade != "-" || len(verdict.TriggeredRules) != 0 || len(verdict.WatchFlags) != 0 {
		t.Fatalf("verdict=%#v", verdict)
	}
	if verdict.ExcludedUnverifiedEvidence != 1 {
		t.Fatalf("excluded=%d", verdict.ExcludedUnverifiedEvidence)
	}
}

func TestActorRuleSignatureIsDeterministic(t *testing.T) {
	track := ActorDefenseTrack{
		Network: "solana-mainnet", TargetKind: "wallet", TargetID: "CaseSensitiveWallet",
		State: "correlated", CreatedTokenCount: 2, DominantHolderTokenCount: 2,
	}
	first := EvaluateActorDefenseRules(track, nil)
	time.Sleep(time.Millisecond)
	second := EvaluateActorDefenseRules(track, nil)
	if first.Signature == "" || first.Signature != second.Signature {
		t.Fatalf("signatures are not deterministic: %q %q", first.Signature, second.Signature)
	}
}

func TestActorRulesNoEvidenceIsNotAGrade(t *testing.T) {
	verdict := EvaluateActorDefenseRules(ActorDefenseTrack{}, nil)
	if verdict.Grade != "-" || verdict.Verdict != "no_grade_trigger" {
		t.Fatalf("absence of evidence became a safe grade: %#v", verdict)
	}
}

func actorRulePresent(items []ActorDefenseRuleHit, id string) bool {
	for _, item := range items {
		if item.RuleID == id {
			return true
		}
	}
	return false
}
