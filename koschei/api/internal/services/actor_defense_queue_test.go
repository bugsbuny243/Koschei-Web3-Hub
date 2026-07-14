package services

import "testing"

func TestActorDefenseVerificationBandHardTrigger(t *testing.T) {
	verdict := ActorDefenseRuleVerdict{
		Grade: "D", Verdict: "hard_trigger",
		TriggeredRules: []ActorDefenseRuleHit{{RuleID: ActorRuleHardCreatorHolderFunding, Tier: "hard_trigger"}},
	}
	band, reason := ActorDefenseVerificationBand(ActorDefenseTrack{}, verdict)
	if band != "hard_trigger" {
		t.Fatalf("band=%q", band)
	}
	if reason != ActorRuleHardCreatorHolderFunding {
		t.Fatalf("reason=%q", reason)
	}
}

func TestActorDefenseVerificationBandCompounding(t *testing.T) {
	verdict := ActorDefenseRuleVerdict{
		Grade: "B", Verdict: "compounding_rule",
		TriggeredRules: []ActorDefenseRuleHit{
			{RuleID: ActorRuleCompoundCreatorReuse, Tier: "compounding"},
			{RuleID: ActorRuleCompoundHolderReuse, Tier: "compounding"},
		},
	}
	band, reason := ActorDefenseVerificationBand(ActorDefenseTrack{}, verdict)
	if band != "compounding" {
		t.Fatalf("band=%q", band)
	}
	if reason != ActorRuleCompoundCreatorReuse+", "+ActorRuleCompoundHolderReuse {
		t.Fatalf("reason=%q", reason)
	}
}

func TestActorDefenseVerificationBandCorrelatedNeedsEvidence(t *testing.T) {
	track := ActorDefenseTrack{State: "correlated"}
	band, _ := ActorDefenseVerificationBand(track, ActorDefenseRuleVerdict{Verdict: "single_observation"})
	if band != "evidence_pending" {
		t.Fatalf("band=%q", band)
	}
	if !actorDefenseNeedsLiveEvidence(track, ActorDefenseRuleVerdict{Verdict: "single_observation"}) {
		t.Fatal("correlated track must request live evidence")
	}
}

func TestActorDefenseRuleNextActionUsesExplicitRule(t *testing.T) {
	verdict := ActorDefenseRuleVerdict{TriggeredRules: []ActorDefenseRuleHit{{RuleID: ActorRuleHardCreatorLiquidityRemoval}}}
	if action := ActorDefenseRuleNextAction(ActorDefenseTrack{}, verdict); action != "review_verified_creator_liquidity_removal" {
		t.Fatalf("next_action=%q", action)
	}
}

func TestActorDefenseQueuePolicyHasNoNumericPriority(t *testing.T) {
	track := ActorDefenseTrack{State: "correlated", CreatedTokenCount: 3, DominantHolderTokenCount: 2}
	verdict := EvaluateActorDefenseRules(track, nil)
	if verdict.Grade != "B" || verdict.Verdict != "compounding_rule" {
		t.Fatalf("verdict=%#v", verdict)
	}
	band, _ := ActorDefenseVerificationBand(track, verdict)
	if band != "compounding" {
		t.Fatalf("band=%q", band)
	}
}
