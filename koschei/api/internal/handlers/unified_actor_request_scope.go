package handlers

import (
	"fmt"
	"strings"

	"koschei/api/internal/services"
)

// hydrateRequestScopeActorDossier assembles only evidence already produced in
// the current live request. It does not read or write persistent actor memory.
// This keeps token investigations useful when the actor database is unavailable
// without making historical, identity, or persistence claims.
func hydrateRequestScopeActorDossier(
	dossier services.ActorDefenseDossier,
	track services.ActorDefenseTrack,
	discovery actorExternalDiscoveryRun,
	funding services.ActorFundingOrigin,
	fundingPersistence string,
	coverage actorDefenseLiveCoverage,
	network string,
) (services.ActorDefenseDossier, services.ActorDefenseTrack) {
	wallet := strings.TrimSpace(track.TargetID)
	if wallet == "" {
		wallet = strings.TrimSpace(dossier.Wallet)
	}
	network = strings.TrimSpace(network)
	if network == "" {
		network = strings.TrimSpace(dossier.Network)
	}

	dossier.Wallet = wallet
	dossier.Network = network
	if dossier.Tokens == nil {
		dossier.Tokens = []services.ActorDefenseTokenObservation{}
	}
	if dossier.RelatedActors == nil {
		dossier.RelatedActors = []services.ActorDefenseRelatedActor{}
	}
	if dossier.Evidence == nil {
		dossier.Evidence = []services.ActorDefenseEvidenceRecord{}
	}
	if dossier.Coverage == nil {
		dossier.Coverage = map[string]any{}
	}
	if dossier.Policy == nil {
		dossier.Policy = map[string]any{}
	}
	if track.Dossier == nil {
		track.Dossier = map[string]any{}
	}
	track.Network = network
	track.TargetKind = "wallet"
	track.TargetID = wallet

	tokenIndex := map[string]bool{}
	for _, token := range dossier.Tokens {
		mint := strings.TrimSpace(token.Mint)
		if mint != "" {
			tokenIndex[mint] = true
		}
	}
	for _, candidate := range discovery.CreatedMintPortfolio.VerifiedCandidates {
		mint := strings.TrimSpace(candidate.Mint)
		if mint == "" || tokenIndex[mint] {
			continue
		}
		dossier.Tokens = append(dossier.Tokens, services.ActorDefenseTokenObservation{
			Mint: mint, Roles: []string{"creator"}, CreatorSignature: strings.TrimSpace(candidate.Signature),
			FirstObservedAt: candidate.ObservedAt, LastObservedAt: candidate.ObservedAt,
		})
		tokenIndex[mint] = true
	}

	appendEvidence := func(items ...services.ActorDefenseEvidenceRecord) {
		seen := map[string]bool{}
		for _, item := range dossier.Evidence {
			seen[requestScopeActorEvidenceIdentity(item)] = true
		}
		for _, item := range items {
			key := requestScopeActorEvidenceIdentity(item)
			if key == "" || seen[key] {
				continue
			}
			if item.Metadata == nil {
				item.Metadata = map[string]any{}
			}
			item.Metadata["request_scope_actor_evidence"] = true
			item.Metadata["persistent_actor_index"] = false
			dossier.Evidence = append(dossier.Evidence, item)
			seen[key] = true
		}
	}

	appendEvidence(services.ActorCreatedMintCandidateEvidence(wallet, network, discovery.CreatedMintPortfolio.VerifiedCandidates)...)
	if fundingEvidence, ok := services.ActorFundingOriginEvidence(funding, network); ok {
		appendEvidence(fundingEvidence)
	}
	appendEvidence(coverage.Evidence...)

	verified, observed := 0, 0
	for _, item := range dossier.Evidence {
		switch strings.ToLower(strings.TrimSpace(item.VerificationStatus)) {
		case "verified":
			verified++
		case "observed":
			observed++
		}
	}
	track.CreatedTokenCount = len(dossier.Tokens)
	track.VerifiedEvidenceCount = verified
	track.ObservedEvidenceCount = observed
	track.State = services.DeriveActorDefenseTrackState(track, dossier.RelatedActors)
	track.Dossier["state_basis"] = []string{"request_scope_live_rpc_evidence", "bounded_created_mint_discovery"}
	track.Dossier["persistence"] = "database_unavailable"
	track.Dossier["token_count"] = len(dossier.Tokens)
	track.Dossier["direct_evidence_count"] = len(dossier.Evidence)
	track.Dossier["no_identity_or_intent_claim"] = true

	dossier.Track = track
	dossier.Coverage["external_discovery"] = discovery
	dossier.Coverage["funding_origin"] = funding
	dossier.Coverage["funding_origin_persistence"] = fundingPersistence
	dossier.Coverage["live_evidence"] = coverage
	dossier.Coverage["persistence"] = "database_unavailable"
	dossier.Coverage["request_scope_actor_evidence"] = true
	dossier.Coverage["numeric_score_disabled"] = true
	dossier.Policy["no_evidence_no_claim"] = true
	dossier.Policy["verified_requires_transaction_or_owner_resolved_chain_evidence"] = true
	dossier.Policy["identity_or_wrongdoing_claim"] = false
	dossier.Policy["raw_request_scope_evidence_persisted"] = false
	return dossier, track
}

func requestScopeActorEvidenceIdentity(item services.ActorDefenseEvidenceRecord) string {
	if key := strings.TrimSpace(item.EvidenceKey); key != "" {
		return "key:" + key
	}
	parts := []string{
		strings.TrimSpace(item.Network),
		strings.TrimSpace(item.ActorWallet),
		strings.TrimSpace(item.Relation),
		strings.TrimSpace(item.Signature),
		strings.TrimSpace(item.TokenMint),
		strings.TrimSpace(item.CounterpartKind),
		strings.TrimSpace(item.CounterpartID),
	}
	allEmpty := true
	for _, part := range parts {
		if part != "" {
			allEmpty = false
			break
		}
	}
	if allEmpty {
		return ""
	}
	return fmt.Sprintf("fallback:%s:%d", strings.Join(parts, "|"), item.Slot)
}
