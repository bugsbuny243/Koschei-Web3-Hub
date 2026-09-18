package handlers

import (
	"testing"
	"time"

	"koschei/api/internal/services"
)

func TestPreferRequestScopeActorLifecycleUsesVerifiedLiveRecurrenceWhenPersistenceUnavailable(t *testing.T) {
	current := services.ActorTokenLifecycleRecurrence{Status: "unavailable", EvidenceStatus: "unavailable", TotalTokens: 0}
	request := services.ActorTokenLifecycleRecurrence{Status: "verified_request_scope_recurrence", EvidenceStatus: "verified", TotalTokens: 2, OtherMints: []string{"MintB"}, ReferencesComplete: true}
	if !preferRequestScopeActorLifecycle(current, request) {
		t.Fatal("verified request-scope recurrence should restore Repeat Actor when persistence is unavailable")
	}
}

func TestPreferRequestScopeActorLifecycleDoesNotDowngradePersistentVerified(t *testing.T) {
	current := services.ActorTokenLifecycleRecurrence{Status: "verified_recurrence", EvidenceStatus: "verified", TotalTokens: 3, OtherMints: []string{"MintB", "MintC"}, ReferencesComplete: true}
	request := services.ActorTokenLifecycleRecurrence{Status: "verified_request_scope_recurrence", EvidenceStatus: "verified", TotalTokens: 2, OtherMints: []string{"MintB"}, ReferencesComplete: true}
	if preferRequestScopeActorLifecycle(current, request) {
		t.Fatal("smaller request-scope evidence must not replace stronger persistent VERIFIED recurrence")
	}
}

func TestApplyRequestScopeActorLifecycleRecurrenceEnrichesRepeatActorArm(t *testing.T) {
	now := time.Now().UTC()
	core := holderIntelligenceCoreResult{}
	core.Analysis = services.AnalyzeArvisRadars(services.SecurityRadarRequest{Target: "MintA", Network: "solana-mainnet"})
	core.Bundle = core.Analysis.Bundle
	core.Arms = core.Analysis.Arms
	external := newActorExternalDiscoveryRun("Creator111")
	external.CreatedMintPortfolio.LifecycleObservations = []services.ActorTokenLifecycleObservation{
		{ActorWallet: "Creator111", Mint: "MintA", CreationSignature: "sig-a", CreationSlot: 101, FirstObservedAt: now, LastObservedAt: now, FateStatus: services.ActorTokenFateActive},
		{ActorWallet: "Creator111", Mint: "MintB", CreationSignature: "sig-b", CreationSlot: 202, FirstObservedAt: now, LastObservedAt: now, FateStatus: services.ActorTokenFateInactiveOrDead},
	}
	current := services.ActorTokenLifecycleRecurrence{Status: "unavailable", EvidenceStatus: "unavailable", ActorWallet: "Creator111", Network: "solana-mainnet", CurrentMint: "MintA"}
	got := applyRequestScopeActorLifecycleRecurrence(&core, current, external, services.ActorDefenseDossier{}, "Creator111", "solana-mainnet", "MintA")
	if got.EvidenceStatus != "verified" || got.Status != "verified_request_scope_recurrence" {
		t.Fatalf("request-scope recurrence = %+v", got)
	}
	found := false
	for _, arm := range core.Arms {
		if arm.ModuleID != services.ModuleRepeatActorScan {
			continue
		}
		found = true
		if observed, _ := arm.Signals["creator_token_recurrence"].(bool); !observed {
			t.Fatalf("repeat actor recurrence signal missing: %+v", arm.Signals)
		}
		if status, _ := arm.Signals["evidence_status"].(string); status != "verified" {
			t.Fatalf("repeat actor evidence status = %q", status)
		}
	}
	if !found {
		t.Fatal("repeat_actor_scan arm not found")
	}
}


func TestApplyRequestScopeActorLifecycleUsesVerifiedGraphCurrentMint(t *testing.T) {
	now := time.Now().UTC()
	core := holderIntelligenceCoreResult{}
	core.Analysis = services.AnalyzeArvisRadars(services.SecurityRadarRequest{Target: "MintA", Network: "solana-mainnet"})
	core.Bundle = core.Analysis.Bundle
	core.Arms = core.Analysis.Arms
	dossier := services.ActorDefenseDossier{
		Wallet: "Creator111", Network: "solana-mainnet",
		Evidence: []services.ActorDefenseEvidenceRecord{{
			Network: "solana-mainnet", ActorWallet: "Creator111",
			CounterpartKind: "token", CounterpartID: "MintA", TokenMint: "MintA",
			Relation: "created_token", VerificationStatus: "verified",
			EvidenceKey: "canonical_creator_relation:MintA", Signature: "sig-a", Slot: 101, ObservedAt: now,
		}},
	}
	current := services.ActorTokenLifecycleRecurrence{Status: "not_investigated", EvidenceStatus: "not_investigated", ActorWallet: "Creator111", Network: "solana-mainnet", CurrentMint: "MintA"}
	got := applyRequestScopeActorLifecycleRecurrence(&core, current, newActorExternalDiscoveryRun("Creator111"), dossier, "Creator111", "solana-mainnet", "MintA")
	if got.TotalTokens != 1 || got.Status != "single_token_only" {
		t.Fatalf("verified graph current mint must surface lifecycle visibility, got %+v", got)
	}
	for _, arm := range core.Arms {
		if arm.ModuleID != services.ModuleRepeatActorScan {
			continue
		}
		if total, _ := arm.Signals["creator_total_tokens"].(int); total != 1 {
			t.Fatalf("repeat actor lifecycle token count = %v", arm.Signals["creator_total_tokens"])
		}
		if recurrence, _ := arm.Signals["creator_token_recurrence"].(bool); recurrence {
			t.Fatal("single verified current mint must not fabricate recurrence")
		}
		return
	}
	t.Fatal("repeat_actor_scan arm not found")
}

func TestVerifiedLifecycleObservationsFromActorGraphRejectsObservedEdge(t *testing.T) {
	now := time.Now().UTC()
	dossier := services.ActorDefenseDossier{
		Wallet: "Creator111", Network: "solana-mainnet",
		Evidence: []services.ActorDefenseEvidenceRecord{{
			Network: "solana-mainnet", ActorWallet: "Creator111",
			CounterpartKind: "token", CounterpartID: "MintA", TokenMint: "MintA",
			Relation: "created_token", VerificationStatus: "observed",
			EvidenceKey: "observed_creator_relation:MintA", Signature: "sig-a", Slot: 101, ObservedAt: now,
		}},
	}
	if got := verifiedLifecycleObservationsFromActorGraph(dossier, "Creator111"); len(got) != 0 {
		t.Fatalf("observed graph edge must not become lifecycle observation: %+v", got)
	}
}
