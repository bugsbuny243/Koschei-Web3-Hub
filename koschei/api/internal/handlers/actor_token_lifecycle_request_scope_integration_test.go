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
	got := applyRequestScopeActorLifecycleRecurrence(&core, current, external, "Creator111", "solana-mainnet", "MintA")
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
