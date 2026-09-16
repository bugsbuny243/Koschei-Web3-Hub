package handlers

import (
	"testing"
	"time"

	"koschei/api/internal/services"
)

func TestHydrateRequestScopeActorDossierUsesLiveEvidenceWithoutPersistence(t *testing.T) {
	now := time.Now().UTC()
	wallet := "ActorWallet111"
	network := "solana-mainnet"
	discovery := newActorExternalDiscoveryRun(wallet)
	discovery.CreatedMintPortfolio.VerifiedCandidates = []services.ActorCreatedMintCandidate{{
		Mint: "Mint111", Signature: "create-signature", Slot: 101, ObservedAt: now,
		Program: "fixture-program", InstructionType: "initializeMint", ActorSigned: true,
		VerificationStatus: "verified", Source: "canonical_rpc",
	}}
	coverage := actorDefenseLiveCoverage{
		Status: "complete", Evidence: []services.ActorDefenseEvidenceRecord{{
			Network: network, ActorWallet: wallet, Relation: "transferred_token",
			VerificationStatus: "verified", EvidenceKey: "request-live:one",
			Signature: "live-signature", Slot: 202, ObservedAt: now,
			Metadata: map[string]any{},
		}}, Limitations: []string{},
	}
	dossier := services.ActorDefenseDossier{
		Wallet: wallet, Network: network, Tokens: []services.ActorDefenseTokenObservation{},
		RelatedActors: []services.ActorDefenseRelatedActor{}, Evidence: []services.ActorDefenseEvidenceRecord{},
		Coverage: map[string]any{}, Policy: map[string]any{}, GeneratedAt: now,
	}
	track := services.ActorDefenseTrack{Network: network, TargetKind: "wallet", TargetID: wallet, Dossier: map[string]any{}}

	dossier, track = hydrateRequestScopeActorDossier(
		dossier, track, discovery, services.ActorFundingOrigin{}, "database_unavailable", coverage, network,
	)
	// Rehydration must be idempotent because the assembler may seed the dossier
	// before collection and finalize it again after live evidence arrives.
	dossier, track = hydrateRequestScopeActorDossier(
		dossier, track, discovery, services.ActorFundingOrigin{}, "database_unavailable", coverage, network,
	)

	if len(dossier.Tokens) != 1 || dossier.Tokens[0].Mint != "Mint111" {
		t.Fatalf("request-scope token hydration = %#v", dossier.Tokens)
	}
	if track.CreatedTokenCount != 1 {
		t.Fatalf("created token count = %d, want 1", track.CreatedTokenCount)
	}
	if track.VerifiedEvidenceCount < 1 {
		t.Fatalf("verified evidence count = %d, want request-scope verified evidence", track.VerifiedEvidenceCount)
	}
	if dossier.Track.TargetID != wallet || dossier.Track.TargetKind != "wallet" {
		t.Fatalf("dossier track = %#v", dossier.Track)
	}
	if dossier.Coverage["persistence"] != "database_unavailable" {
		t.Fatalf("persistence marker = %#v", dossier.Coverage["persistence"])
	}
	if dossier.Coverage["request_scope_actor_evidence"] != true {
		t.Fatalf("request-scope marker = %#v", dossier.Coverage["request_scope_actor_evidence"])
	}
	seenLive := 0
	for _, item := range dossier.Evidence {
		if item.EvidenceKey != "request-live:one" {
			continue
		}
		seenLive++
		if item.Metadata["persistent_actor_index"] != false || item.Metadata["request_scope_actor_evidence"] != true {
			t.Fatalf("live evidence provenance = %#v", item.Metadata)
		}
	}
	if seenLive != 1 {
		t.Fatalf("live evidence occurrences = %d, want 1", seenLive)
	}
}

func TestRequestScopeActorEvidenceIdentityPrefersEvidenceKey(t *testing.T) {
	item := services.ActorDefenseEvidenceRecord{
		EvidenceKey: "canonical-key", Signature: "signature", Slot: 99,
	}
	if got := requestScopeActorEvidenceIdentity(item); got != "key:canonical-key" {
		t.Fatalf("identity = %q", got)
	}
}
