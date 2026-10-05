package services

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizePublicPromotionObservationCanonicalizesEvidence(t *testing.T) {
	observedAt := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	observation, err := NormalizePublicPromotionObservation(PublicPromotionObservation{
		Network:       " Solana-Mainnet ",
		AssetRef:      "MintA",
		Platform:      "Twitter",
		ExternalID:    "post-1",
		CanonicalURL:  "HTTPS://Example.COM:443/post/1?utm_source=test&b=2&a=1#section",
		PublicActor:   "@PromoterOne",
		SourceRef:     "collector:x:post-1",
		EvidenceState: PublicPromotionObserved,
		Excerpt:       "  Big   launch   today  ",
		ObservedAt:    observedAt,
	})
	if err != nil {
		t.Fatalf("normalize observation: %v", err)
	}
	if observation.SchemaVersion != PublicPromotionEvidenceSchemaVersion {
		t.Fatalf("unexpected schema version: %s", observation.SchemaVersion)
	}
	if observation.Network != "solana-mainnet" {
		t.Fatalf("unexpected network: %s", observation.Network)
	}
	if observation.Platform != "x" {
		t.Fatalf("unexpected platform: %s", observation.Platform)
	}
	if observation.CanonicalURL != "https://example.com/post/1?a=1&b=2" {
		t.Fatalf("unexpected canonical URL: %s", observation.CanonicalURL)
	}
	if observation.CanonicalDomain != "example.com" {
		t.Fatalf("unexpected canonical domain: %s", observation.CanonicalDomain)
	}
	if observation.PublicActor != "promoterone" {
		t.Fatalf("unexpected public actor: %s", observation.PublicActor)
	}
	if observation.Excerpt != "Big launch today" {
		t.Fatalf("unexpected excerpt: %q", observation.Excerpt)
	}
	if !strings.HasPrefix(observation.ObservationRef, "KPUB1-") || len(observation.ObservationRef) != len("KPUB1-")+32 {
		t.Fatalf("unexpected observation ref: %s", observation.ObservationRef)
	}
	if !strings.HasPrefix(observation.ContentHashSHA256, "sha256:") {
		t.Fatalf("unexpected content hash: %s", observation.ContentHashSHA256)
	}
	if observation.ClaimFingerprintSHA256 == "" {
		t.Fatal("expected claim fingerprint")
	}
	if observation.VerdictAuthority || observation.GradeAuthority || observation.SameOperatorClaim || observation.RealWorldIdentityClaim || observation.WrongdoingClaim {
		t.Fatal("public promotion observation must never gain authority claims")
	}
}

func TestNormalizePublicPromotionObservationRejectsCredentialURL(t *testing.T) {
	_, err := NormalizePublicPromotionObservation(PublicPromotionObservation{
		Network:       "solana-mainnet",
		AssetRef:      "MintA",
		Platform:      "website",
		CanonicalURL:  "https://user:secret@example.com/post",
		SourceRef:     "collector:web:1",
		EvidenceState: PublicPromotionObserved,
		ObservedAt:    time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected credential-bearing URL to be rejected")
	}
}

func TestBuildPublicPromotionCorrelationReportIsWatchOnly(t *testing.T) {
	observedAt := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	observations := []PublicPromotionObservation{
		{
			Network:       "solana-mainnet",
			AssetRef:      "MintA",
			Platform:      "x",
			ExternalID:    "1",
			CanonicalURL:  "https://x.com/shared/status/1",
			PublicActor:   "shared",
			SourceRef:     "collector:x:1",
			EvidenceState: PublicPromotionObserved,
			Excerpt:       "same launch claim",
			ObservedAt:    observedAt,
		},
		{
			Network:       "solana-mainnet",
			AssetRef:      "MintB",
			Platform:      "x",
			ExternalID:    "2",
			CanonicalURL:  "https://x.com/shared/status/2",
			PublicActor:   "shared",
			SourceRef:     "collector:x:2",
			EvidenceState: PublicPromotionVerified,
			Excerpt:       "same launch claim",
			ObservedAt:    observedAt.Add(time.Minute),
		},
	}

	report := BuildPublicPromotionCorrelationReport(observations)
	if len(report.Signals) < 2 {
		t.Fatalf("expected multiple cross-asset correlation signals, got %d", len(report.Signals))
	}
	foundActor := false
	foundClaim := false
	for _, signal := range report.Signals {
		if signal.EvidenceState != "inferred" {
			t.Fatalf("signal must be inferred/watch-only, got %s", signal.EvidenceState)
		}
		if signal.Kind == "shared_public_actor" {
			foundActor = true
		}
		if signal.Kind == "repeated_claim_fingerprint" {
			foundClaim = true
		}
		if len(signal.Assets) != 2 {
			t.Fatalf("expected two distinct assets for %s, got %#v", signal.Kind, signal.Assets)
		}
	}
	if !foundActor || !foundClaim {
		t.Fatalf("missing expected signals: actor=%v claim=%v", foundActor, foundClaim)
	}
	if report.SameOperatorClaim || report.RealWorldIdentityClaim || report.WrongdoingClaim {
		t.Fatal("correlation report must not assert identity, operator or wrongdoing")
	}
}

func TestPublicPromotionCampaignMaterialDoesNotPromoteHandleToActor(t *testing.T) {
	material, err := PublicPromotionCampaignMaterial(PublicPromotionObservation{
		Network:       "solana-mainnet",
		AssetRef:      "MintA",
		Platform:      "telegram",
		ExternalID:    "44",
		CanonicalURL:  "https://t.me/publicchannel/44",
		PublicActor:   "publicchannel",
		SourceRef:     "collector:telegram:44",
		EvidenceState: PublicPromotionVerified,
		Excerpt:       "launch announcement",
		ObservedAt:    time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("campaign material: %v", err)
	}
	if len(material.Actors) != 0 {
		t.Fatalf("public handle must not become an on-chain actor: %#v", material.Actors)
	}
	if len(material.ObservationRefs) != 1 || !strings.HasPrefix(material.ObservationRefs[0], "KPUB1-") {
		t.Fatalf("expected one public observation ref, got %#v", material.ObservationRefs)
	}
	if material.VerifiedAnchorCount != 1 || material.ObservedAnchorCount != 0 {
		t.Fatalf("unexpected anchor counts: verified=%d observed=%d", material.VerifiedAnchorCount, material.ObservedAnchorCount)
	}
	if len(material.RelationRefs) != 0 {
		t.Fatalf("public overlap must not manufacture a relation anchor: %#v", material.RelationRefs)
	}
}
