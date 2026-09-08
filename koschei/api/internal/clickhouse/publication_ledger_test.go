package clickhouse

import (
	"strings"
	"testing"
	"time"
)

const testPublicationCaseRef = "KD1-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestDossierManifestPreservesCaseSensitiveTargetIdentity(t *testing.T) {
	base := DossierBundleManifest{
		CaseRef: testPublicationCaseRef, BundleSHA256: strings.Repeat("a", 64),
		ArtifactURI: "gdrive://file/canonical-dossier", ArtifactSHA256: strings.Repeat("b", 64),
		DossierVersion: "koschei-dossier-v1", Network: "solana-mainnet", TargetKind: "wallet",
		TargetID: "AbCdEf123", ProducedAt: time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC),
	}
	upper, err := NormalizeDossierBundleManifest(base)
	if err != nil {
		t.Fatalf("normalize upper-case target manifest: %v", err)
	}
	base.TargetID = "abcdef123"
	lower, err := NormalizeDossierBundleManifest(base)
	if err != nil {
		t.Fatalf("normalize lower-case target manifest: %v", err)
	}
	if upper.TargetID == lower.TargetID || upper.ManifestSHA256 == lower.ManifestSHA256 {
		t.Fatal("case-sensitive target identities collapsed")
	}
	if upper.ManifestID != lower.ManifestID {
		t.Fatal("same immutable case/bundle should retain one deterministic manifest identity")
	}
}

func TestDossierManifestRejectsCredentialBearingArtifactURI(t *testing.T) {
	_, err := NormalizeDossierBundleManifest(DossierBundleManifest{
		CaseRef: testPublicationCaseRef, BundleSHA256: strings.Repeat("a", 64),
		ArtifactURI: "https://drive.google.com/file/d/abc?token=secret", ArtifactSHA256: strings.Repeat("b", 64),
		DossierVersion: "koschei-dossier-v1", Network: "solana-mainnet", TargetKind: "wallet",
		TargetID: "AbCdEf123", ProducedAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("credential-bearing artifact URI unexpectedly accepted")
	}
}

func TestPublicationChainPublishHideRepublish(t *testing.T) {
	manifest := mustPublicationManifest(t)
	firstAt := time.Date(2026, 9, 8, 12, 0, 0, 123456000, time.UTC)
	first := mustPublicationTransition(t, nil, PublicationState{
		Status: "public", PublicTitle: "Evidence case", PublicSummary: "Bounded summary",
		RedactionProfile: "public-onchain-v1", PublishedBy: "owner", ManifestID: manifest.ManifestID,
		BundleSHA256: manifest.BundleSHA256,
	}, firstAt)
	if first.Action != "publish" || !first.ExposureStartedAt.Equal(firstAt.Truncate(time.Millisecond)) {
		t.Fatalf("unexpected first publish: %#v", first)
	}

	hideAt := firstAt.Add(time.Hour)
	hidden := mustPublicationTransition(t, &first, PublicationState{
		Status: "hidden", PublicTitle: first.PublicTitle, PublicSummary: first.PublicSummary,
		RedactionProfile: first.RedactionProfile, PublishedBy: "owner", ManifestID: manifest.ManifestID,
		BundleSHA256: manifest.BundleSHA256,
	}, hideAt)
	if hidden.Action != "hide" || !hidden.ExposureStartedAt.Equal(first.ExposureStartedAt) {
		t.Fatalf("hide did not preserve previous exposure start: %#v", hidden)
	}

	republishAt := hideAt.Add(time.Hour)
	republished := mustPublicationTransition(t, &hidden, PublicationState{
		Status: "public", PublicTitle: first.PublicTitle, PublicSummary: first.PublicSummary,
		RedactionProfile: first.RedactionProfile, PublishedBy: "owner", ManifestID: manifest.ManifestID,
		BundleSHA256: manifest.BundleSHA256,
	}, republishAt)
	if republished.Action != "publish" || !republished.ExposureStartedAt.Equal(republishAt.Truncate(time.Millisecond)) {
		t.Fatalf("republish did not start a new exposure interval: %#v", republished)
	}

	chain := []PublicationTransition{first, hidden, republished}
	if err := ValidatePublicationTransitionChain(chain); err != nil {
		t.Fatalf("valid publication chain rejected: %v", err)
	}
	latest, ok, err := LatestPublicationState(chain)
	if err != nil || !ok || latest.Status != "public" || !latest.ExposureStartedAt.Equal(republished.ExposureStartedAt) {
		t.Fatalf("unexpected latest publication state: state=%#v ok=%v err=%v", latest, ok, err)
	}
}

func TestPublicationTransitionIsDeterministic(t *testing.T) {
	manifest := mustPublicationManifest(t)
	state := PublicationState{
		Status: "public", RedactionProfile: "public-onchain-v1", PublishedBy: "koschei-autopublish/v1",
		ManifestID: manifest.ManifestID, BundleSHA256: manifest.BundleSHA256,
	}
	at := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	one := mustPublicationTransition(t, nil, state, at)
	two := mustPublicationTransition(t, nil, state, at)
	if one.TransitionID != two.TransitionID || one.TransitionSHA256 != two.TransitionSHA256 {
		t.Fatal("same publication transition did not converge to deterministic identity")
	}
	if one.Actor != "autopublish" {
		t.Fatalf("unexpected publisher actor mapping: %q", one.Actor)
	}
}

func TestPublicationChainFailsClosedOnTamperAndFork(t *testing.T) {
	manifest := mustPublicationManifest(t)
	first := mustPublicationTransition(t, nil, PublicationState{
		Status: "public", RedactionProfile: "public-onchain-v1", PublishedBy: "owner",
		ManifestID: manifest.ManifestID, BundleSHA256: manifest.BundleSHA256,
	}, time.Date(2026, 9, 8, 16, 0, 0, 0, time.UTC))
	second := mustPublicationTransition(t, &first, PublicationState{
		Status: "hidden", RedactionProfile: "public-onchain-v1", PublishedBy: "owner",
		ManifestID: manifest.ManifestID, BundleSHA256: manifest.BundleSHA256,
	}, first.OccurredAt.Add(time.Minute))

	tampered := second
	tampered.PublicSummary = "changed after signing"
	if err := ValidatePublicationTransitionChain([]PublicationTransition{first, tampered}); err == nil {
		t.Fatal("tampered transition unexpectedly validated")
	}

	fork := mustPublicationTransition(t, &first, PublicationState{
		Status: "draft", RedactionProfile: "public-onchain-v1", PublishedBy: "owner",
		ManifestID: manifest.ManifestID, BundleSHA256: manifest.BundleSHA256,
	}, first.OccurredAt.Add(2*time.Minute))
	if err := ValidatePublicationTransitionChain([]PublicationTransition{first, second, fork}); err == nil {
		t.Fatal("same-sequence publication fork unexpectedly validated")
	}
}

func TestPublicationChainRejectsActorPublisherMismatchEvenWithRehashedPayload(t *testing.T) {
	manifest := mustPublicationManifest(t)
	transition := mustPublicationTransition(t, nil, PublicationState{
		Status: "public", RedactionProfile: "public-onchain-v1", PublishedBy: "owner",
		ManifestID: manifest.ManifestID, BundleSHA256: manifest.BundleSHA256,
	}, time.Date(2026, 9, 8, 17, 0, 0, 0, time.UTC))
	transition.Actor = "autopublish"
	transition.TransitionID = publicationTransitionID(transition)
	transition.TransitionSHA256 = publicationTransitionSHA256(transition)
	if err := ValidatePublicationTransitionChain([]PublicationTransition{transition}); err == nil {
		t.Fatal("actor/publisher mismatch unexpectedly validated")
	}
}

func mustPublicationManifest(t *testing.T) DossierBundleManifest {
	t.Helper()
	manifest, err := NormalizeDossierBundleManifest(DossierBundleManifest{
		CaseRef: testPublicationCaseRef, BundleSHA256: strings.Repeat("a", 64),
		ArtifactURI: "gdrive://file/canonical-dossier", ArtifactSHA256: strings.Repeat("b", 64),
		DossierVersion: "koschei-dossier-v1", Network: "solana-mainnet", TargetKind: "wallet",
		TargetID: "AbCdEf123", VerdictGrade: "B", VerdictStatus: "observed", RulesetVersion: "rules-v1",
		EvidenceRows: 7, VerifiedRows: 3, ObservedRows: 2, InferredRows: 1, UnknownRows: 1,
		AcceptancePass: 2, AcceptanceFail: 1, ProducedAt: time.Date(2026, 9, 8, 11, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("normalize manifest: %v", err)
	}
	return manifest
}

func mustPublicationTransition(t *testing.T, previous *PublicationTransition, state PublicationState, at time.Time) PublicationTransition {
	t.Helper()
	transition, err := BuildPublicationTransition(testPublicationCaseRef, previous, state, at)
	if err != nil {
		t.Fatalf("build publication transition: %v", err)
	}
	return transition
}
