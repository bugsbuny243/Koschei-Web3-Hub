package services

import (
	"testing"
	"time"
)

func TestPublicPromotionObservationFromVerifiedSocialBinding(t *testing.T) {
	observation, err := PublicPromotionObservationFromSourceItem(PublicPromotionSourceBinding{
		Network:      "solana-mainnet",
		AssetRef:     "MintA",
		Platform:     "x",
		PublicActor:  "@OfficialProject",
		CanonicalURL: "https://x.com/officialproject",
		RelationRef:  "REL:official-project:MintA:x:officialproject",
		Verified:     true,
	}, "x-api", PublicPromotionSourceItem{
		ExternalID:   "123",
		CanonicalURL: "https://x.com/officialproject/status/123",
		PublicActor:  "officialproject",
		Visibility:   "public",
		Excerpt:      "launch announcement",
		ObservedAt:   time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("source item: %v", err)
	}
	if observation.EvidenceState != PublicPromotionVerified {
		t.Fatalf("expected verified evidence state, got %s", observation.EvidenceState)
	}
	if observation.SourceRef != "public-source:x-api:123" {
		t.Fatalf("unexpected source ref: %s", observation.SourceRef)
	}
	if observation.PublicActor != "officialproject" {
		t.Fatalf("unexpected actor: %s", observation.PublicActor)
	}
}

func TestPublicPromotionObservationRejectsPrivateOrMismatchedSocialItem(t *testing.T) {
	binding := PublicPromotionSourceBinding{
		Network:      "solana-mainnet",
		AssetRef:     "MintA",
		Platform:     "telegram",
		PublicActor:  "officialchannel",
		CanonicalURL: "https://t.me/officialchannel",
	}
	privateItem := PublicPromotionSourceItem{
		CanonicalURL: "https://t.me/officialchannel/1",
		PublicActor:  "officialchannel",
		Visibility:   "private",
		ObservedAt:   time.Now().UTC(),
	}
	if _, err := PublicPromotionObservationFromSourceItem(binding, "telegram-public", privateItem); err == nil {
		t.Fatal("expected private item to fail closed")
	}
	mismatch := privateItem
	mismatch.Visibility = "public"
	mismatch.PublicActor = "differentchannel"
	if _, err := PublicPromotionObservationFromSourceItem(binding, "telegram-public", mismatch); err == nil {
		t.Fatal("expected mismatched public actor to fail closed")
	}
}

func TestPublicPromotionObservationRejectsWebsiteDomainEscape(t *testing.T) {
	binding := PublicPromotionSourceBinding{
		Network:      "solana-mainnet",
		AssetRef:     "MintA",
		Platform:     "website",
		CanonicalURL: "https://project.example/",
	}
	item := PublicPromotionSourceItem{
		CanonicalURL: "https://attacker.example/copied-page",
		Visibility:   "public",
		ObservedAt:   time.Now().UTC(),
	}
	if _, err := PublicPromotionObservationFromSourceItem(binding, "website-crawler", item); err == nil {
		t.Fatal("expected website domain escape to fail closed")
	}
}

func TestVerifiedPublicPromotionBindingRequiresRelationRef(t *testing.T) {
	_, err := normalizePublicPromotionSourceBinding(PublicPromotionSourceBinding{
		Network:      "solana-mainnet",
		AssetRef:     "MintA",
		Platform:     "x",
		PublicActor:  "officialproject",
		CanonicalURL: "https://x.com/officialproject",
		Verified:     true,
	})
	if err == nil {
		t.Fatal("expected verified binding without relation ref to fail closed")
	}
}
