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
		CanonicalURL:  "https://Example.COM:443/post/1?utm_source=test&b=2&a=1#section",
		PublicActor:   "@PromoterOne",
		SourceRef:     "collector:x:post-1",
		EvidenceState: PublicPromotionObserved,
		Excerpt:       "  Big   launch   today  ",
		ObservedAt:    observedAt,
		Metadata: map[string]string{
			"Language":           "en",
			"AuthorizationToken": "must-not-persist",
		},
	})
	if err != nil {
		t.Fatalf("normalize observation: %v", err)
	}
	if observation.SchemaVersion != PublicPromotionEvidenceSchemaVersion {
		t.Fatalf("unexpected schema version: %s", observation.SchemaVersion)
	}
	if observation.Network != "solana-mainnet" || observation.Platform != "x" {
		t.Fatalf("unexpected normalized identity: network=%s platform=%s", observation.Network, observation.Platform)
	}
	if observation.CanonicalURL != "https://example.com/post/1?a=1&b=2" {
		t.Fatalf("unexpected canonical URL: %s", observation.CanonicalURL)
	}
	if observation.CanonicalDomain != "example.com" || observation.PublicActor != "promoterone" {
		t.Fatalf("unexpected public provenance: domain=%s actor=%s", observation.CanonicalDomain, observation.PublicActor)
	}
	if observation.Excerpt != "Big launch today" {
		t.Fatalf("unexpected excerpt: %q", observation.Excerpt)
	}
	if observation.Metadata["language"] != "en" {
		t.Fatalf("expected safe metadata to survive: %#v", observation.Metadata)
	}
	if _, ok := observation.Metadata["authorizationtoken"]; ok {
		t.Fatalf("sensitive metadata must be removed: %#v", observation.Metadata)
	}
	if !strings.HasPrefix(observation.ObservationRef, "KPUB1-") || len(observation.ObservationRef) != len("KPUB1-")+32 {
		t.Fatalf("unexpected observation ref: %s", observation.ObservationRef)
	}
	if !strings.HasPrefix(observation.ContentHashSHA256, "sha256:") || observation.ClaimFingerprintSHA256 == "" {
		t.Fatalf("expected content and claim fingerprints: content=%s claim=%s", observation.ContentHashSHA256, observation.ClaimFingerprintSHA256)
	}
	if observation.VerdictAuthority || observation.GradeAuthority || observation.SameOperatorClaim || observation.RealWorldIdentityClaim || observation.WrongdoingClaim {
		t.Fatal("public promotion observation must never gain authority claims")
	}
}

func TestNormalizePublicPromotionObservationRejectsUnsafeURL(t *testing.T) {
	base := PublicPromotionObservation{
		Network:       "solana-mainnet",
		AssetRef:      "MintA",
		Platform:      "website",
		SourceRef:     "collector:web:1",
		EvidenceState: PublicPromotionObserved,
		ObservedAt:    time.Now().UTC(),
	}
	for _, rawURL := range []string{
		"https://user:secret@example.com/post",
		"http://127.0.0.1/internal",
		"http://localhost/internal",
		"http://10.0.0.7/internal",
	} {
		candidate := base
		candidate.CanonicalURL = rawURL
		if _, err := NormalizePublicPromotionObservation(candidate); err == nil {
			t.Fatalf("expected unsafe URL to be rejected: %s", rawURL)
		}
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
	foundActor := false
	foundClaim := false
	for _, signal := range report.Signals {
		if signal.EvidenceState != "inferred" {
			t.Fatalf("signal must be inferred/watch-only, got %s", signal.EvidenceState)
		}
		if signal.Kind == "shared_public_domain" {
			t.Fatalf("platform host x.com must not create a domain correlation: %#v", signal)
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

func TestBuildPublicPromotionCorrelationReportAllowsProjectWebsiteDomain(t *testing.T) {
	observedAt := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	report := BuildPublicPromotionCorrelationReport([]PublicPromotionObservation{
		{
			Network:       "solana-mainnet",
			AssetRef:      "MintA",
			Platform:      "website",
			CanonicalURL:  "https://project.example/token-a",
			SourceRef:     "collector:web:a",
			EvidenceState: PublicPromotionObserved,
			ObservedAt:    observedAt,
		},
		{
			Network:       "solana-mainnet",
			AssetRef:      "MintB",
			Platform:      "website",
			CanonicalURL:  "https://project.example/token-b",
			SourceRef:     "collector:web:b",
			EvidenceState: PublicPromotionVerified,
			ObservedAt:    observedAt.Add(time.Minute),
		},
	})
	for _, signal := range report.Signals {
		if signal.Kind == "shared_public_domain" && signal.Key == "project.example" {
			return
		}
	}
	t.Fatalf("expected project website domain correlation, got %#v", report.Signals)
}

func TestPublicPromotionCampaignMaterialRequiresVerifiedRelation(t *testing.T) {
	observation := PublicPromotionObservation{
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
	}
	if _, err := PublicPromotionCampaignMaterial(observation, nil); err == nil {
		t.Fatal("expected campaign attachment without verified relation to fail closed")
	}

	material, err := PublicPromotionCampaignMaterial(observation, []string{"REL:official-project:MintA:telegram:publicchannel"})
	if err != nil {
		t.Fatalf("campaign material: %v", err)
	}
	if len(material.Actors) != 0 {
		t.Fatalf("public handle must not become an on-chain actor: %#v", material.Actors)
	}
	if len(material.ObservationRefs) != 1 || !strings.HasPrefix(material.ObservationRefs[0], "KPUB1-") {
		t.Fatalf("expected one public observation ref, got %#v", material.ObservationRefs)
	}
	if len(material.RelationRefs) != 1 || material.RelationRefs[0] != "REL:official-project:MintA:telegram:publicchannel" {
		t.Fatalf("expected only upstream verified relation anchor, got %#v", material.RelationRefs)
	}
	if material.VerifiedAnchorCount != 1 || material.ObservedAnchorCount != 0 {
		t.Fatalf("unexpected anchor counts: verified=%d observed=%d", material.VerifiedAnchorCount, material.ObservedAnchorCount)
	}
	if material.RulesetVersion != "" {
		t.Fatalf("public evidence must not override global campaign ruleset: %q", material.RulesetVersion)
	}
}
