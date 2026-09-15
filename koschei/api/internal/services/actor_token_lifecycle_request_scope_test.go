package services

import (
	"testing"
	"time"
)

func TestBuildRequestScopeTokenLifecycleRecurrenceVerifiedWithoutPersistence(t *testing.T) {
	now := time.Date(2026, 9, 15, 2, 0, 0, 0, time.UTC)
	observations := []ActorTokenLifecycleObservation{
		{Network: "solana-mainnet", ActorWallet: "Creator", Mint: "CurrentMint", CreationSignature: "sig-current", CreationSlot: 100, FirstObservedAt: now, LastObservedAt: now, FateStatus: ActorTokenFateActive},
		{Network: "solana-mainnet", ActorWallet: "Creator", Mint: "OldMint", CreationSignature: "sig-old", CreationSlot: 90, FirstObservedAt: now.Add(-time.Hour), LastObservedAt: now, FateStatus: ActorTokenFateInactiveOrDead},
	}

	got := BuildRequestScopeTokenLifecycleRecurrence("Creator", "solana-mainnet", "CurrentMint", observations)
	if !got.Available || got.Status != "verified_request_scope_recurrence" || got.EvidenceStatus != "verified" || !got.ReferencesComplete {
		t.Fatalf("recurrence=%#v", got)
	}
	if got.TotalTokens != 2 || got.ActiveTokens != 1 || got.InactiveOrDeadTokens != 1 {
		t.Fatalf("counts=%#v", got)
	}
	if len(got.OtherMints) != 1 || got.OtherMints[0] != "OldMint" {
		t.Fatalf("other mints=%#v", got.OtherMints)
	}
	if len(got.CreationSignatures) != 1 || got.CreationSignatures[0] != "sig-old" || len(got.CreationSlots) != 1 || got.CreationSlots[0] != 90 {
		t.Fatalf("references=%#v/%#v", got.CreationSignatures, got.CreationSlots)
	}
}

func TestBuildRequestScopeTokenLifecycleRecurrenceWithholdsVerifiedOnReferenceGap(t *testing.T) {
	now := time.Date(2026, 9, 15, 2, 0, 0, 0, time.UTC)
	observations := []ActorTokenLifecycleObservation{
		{ActorWallet: "Creator", Mint: "MintA", CreationSignature: "sig-a", CreationSlot: 100, FirstObservedAt: now, LastObservedAt: now, FateStatus: ActorTokenFateActive},
		{ActorWallet: "Creator", Mint: "MintB", CreationSlot: 90, FirstObservedAt: now, LastObservedAt: now, FateStatus: ActorTokenFateInactiveOrDead},
	}

	got := BuildRequestScopeTokenLifecycleRecurrence("Creator", "solana-mainnet", "CurrentMint", observations)
	if got.EvidenceStatus != "observed" || got.ReferencesComplete || got.Status != "observed_request_scope_recurrence" {
		t.Fatalf("reference gap upgraded incorrectly: %#v", got)
	}
}
