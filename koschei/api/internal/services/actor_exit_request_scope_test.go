package services

import (
	"testing"
	"time"
)

func TestBuildRequestScopeActorExitRecurrenceVerifiedCrossToken(t *testing.T) {
	now := time.Now().UTC()
	actor := "creator-wallet"
	evidence := []ActorDefenseEvidenceRecord{
		{
			ActorWallet: actor, Network: "solana", TokenMint: "mint-a", Relation: "liquidity_remove_activity",
			VerificationStatus: "verified", Signature: "sig-a", Slot: 101, ObservedAt: now,
			Metadata: map[string]any{"actor_signed": true, "creator_role_observed": true},
		},
		{
			ActorWallet: actor, Network: "solana", TokenMint: "mint-b", Relation: "liquidity_remove_activity",
			VerificationStatus: "verified", Signature: "sig-b", Slot: 202, ObservedAt: now.Add(time.Minute),
			Metadata: map[string]any{"actor_signed": true, "creator_role_observed": true},
		},
	}

	got := BuildRequestScopeActorExitRecurrence(actor, "solana", "mint-b", evidence)
	if !got.Available || got.Status != "verified_request_scope_exit_recurrence" || got.EvidenceStatus != "verified" {
		t.Fatalf("unexpected recurrence: %+v", got)
	}
	if len(got.OtherTargets) != 1 || got.OtherTargets[0] != "mint-a" || len(got.Events) != 2 || !got.ReferencesComplete {
		t.Fatalf("unexpected references: %+v", got)
	}
}

func TestBuildRequestScopeActorExitRecurrenceDoesNotTreatInactiveLifecycleAsExit(t *testing.T) {
	got := BuildRequestScopeActorExitRecurrence("creator-wallet", "solana", "mint-a", nil)
	if got.Available || got.Status != "no_transaction_referenced_exit_events" || got.EvidenceStatus != "not_observed" {
		t.Fatalf("unexpected empty recurrence: %+v", got)
	}
}

func TestBuildRequestScopeActorExitRecurrenceWithholdsObservedFromVerified(t *testing.T) {
	now := time.Now().UTC()
	actor := "holder-wallet"
	evidence := []ActorDefenseEvidenceRecord{
		{
			Network: "solana", TokenMint: "mint-a", Relation: "dominant_holder_first_exit",
			VerificationStatus: "observed", Signature: "sig-a", Slot: 303, ObservedAt: now,
			Metadata: map[string]any{
				"unified_rule_id": UnifiedRuleDominantHolderFirstExit,
				"metrics":         map[string]any{"holder_wallet": actor},
			},
		},
		{
			Network: "solana", TokenMint: "mint-b", Relation: "dominant_holder_first_exit",
			VerificationStatus: "observed", Signature: "sig-b", Slot: 404, ObservedAt: now.Add(time.Minute),
			Metadata: map[string]any{
				"unified_rule_id": UnifiedRuleDominantHolderFirstExit,
				"metrics":         map[string]any{"holder_wallet": actor},
			},
		},
	}

	got := BuildRequestScopeActorExitRecurrence(actor, "solana", "mint-b", evidence)
	if got.Status != "observed_request_scope_exit_recurrence" || got.EvidenceStatus != "observed" {
		t.Fatalf("observed evidence was upgraded: %+v", got)
	}
}
