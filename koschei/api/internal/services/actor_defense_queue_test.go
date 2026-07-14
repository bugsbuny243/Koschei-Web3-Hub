package services

import (
	"testing"
	"time"
)

func TestActorDefenseVerificationPriorityCorrelatedCreator(t *testing.T) {
	now := time.Date(2026, 7, 14, 2, 0, 0, 0, time.UTC)
	track := ActorDefenseTrack{
		State: "correlated", CreatedTokenCount: 3, RelatedActorCount: 2,
		LastSeenAt: now.Add(-2 * time.Hour),
	}
	score, reasons, action := ActorDefenseVerificationPriority(track, now)
	if score != 68 {
		t.Fatalf("priority=%d want=68 reasons=%v", score, reasons)
	}
	if action != "verify_creator_funding_and_transfer_chain" {
		t.Fatalf("next_action=%q", action)
	}
}

func TestActorDefenseVerificationPriorityRepeatHolder(t *testing.T) {
	now := time.Date(2026, 7, 14, 2, 0, 0, 0, time.UTC)
	track := ActorDefenseTrack{
		State: "correlated", DominantHolderTokenCount: 2,
		LastSeenAt: now.Add(-72 * time.Hour),
	}
	score, _, action := ActorDefenseVerificationPriority(track, now)
	if score != 51 {
		t.Fatalf("priority=%d want=51", score)
	}
	if action != "collect_live_transaction_evidence" {
		t.Fatalf("next_action=%q", action)
	}
}

func TestActorDefenseVerificationPriorityVerifiedEvidence(t *testing.T) {
	now := time.Date(2026, 7, 14, 2, 0, 0, 0, time.UTC)
	track := ActorDefenseTrack{
		State: "verified", VerifiedEvidenceCount: 2, ObservedEvidenceCount: 1,
		LastSeenAt: now.Add(-10 * time.Hour),
	}
	score, _, action := ActorDefenseVerificationPriority(track, now)
	if score != 29 {
		t.Fatalf("priority=%d want=29", score)
	}
	if action != "review_verified_evidence" {
		t.Fatalf("next_action=%q", action)
	}
}

func TestActorDefenseVerificationPriorityCapsAtHundred(t *testing.T) {
	now := time.Now().UTC()
	track := ActorDefenseTrack{
		State: "correlated", CreatedTokenCount: 100, DominantHolderTokenCount: 100,
		RelatedActorCount: 100, ObservedEvidenceCount: 100, VerifiedEvidenceCount: 100,
		LastSeenAt: now,
	}
	score, _, _ := ActorDefenseVerificationPriority(track, now)
	if score != 100 {
		t.Fatalf("priority=%d want=100", score)
	}
}

func TestActorDefenseVerificationPriorityDoesNotRewardStaleObservation(t *testing.T) {
	now := time.Date(2026, 7, 14, 2, 0, 0, 0, time.UTC)
	track := ActorDefenseTrack{State: "detected", LastSeenAt: now.Add(-31 * 24 * time.Hour)}
	score, reasons, action := ActorDefenseVerificationPriority(track, now)
	if score != 5 {
		t.Fatalf("priority=%d want=5 reasons=%v", score, reasons)
	}
	if action != "monitor_sensor_memory" {
		t.Fatalf("next_action=%q", action)
	}
}
