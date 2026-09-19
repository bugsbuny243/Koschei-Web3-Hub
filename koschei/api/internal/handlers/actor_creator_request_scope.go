package handlers

import (
	"strings"
	"time"

	"koschei/api/internal/services"
)

// buildRequestScopeCanonicalCreatorMintRelation projects the canonical creator
// relation already resolved by the token radar into the current actor request.
// It never queries or writes persistent actor memory. VERIFIED is allowed only
// when canonical source context already carries the verified flag, signature
// and positive slot produced by canonical RPC verification.
func buildRequestScopeCanonicalCreatorMintRelation(core holderIntelligenceCoreResult, creator, network string) actorCreatorRelationRun {
	mint := strings.TrimSpace(core.Request.Target)
	creator = strings.TrimSpace(creator)
	network = strings.TrimSpace(network)
	if network == "" {
		network = "solana-mainnet"
	}
	out := newActorCreatorRelationRun(creator, mint)
	out.Persistence = "request_scope"
	if creator == "" || mint == "" {
		out.Status = "target_unavailable"
		out.Limitations = append(out.Limitations, "Creator wallet veya token mint çözümlenemedi.")
		return out
	}

	source := core.SourceContext
	signature := strings.TrimSpace(firstNonEmptyString(
		creatorIntelCleanString(source["creation_signature"]),
		creatorIntelCleanString(source["launch_signature"]),
		creatorIntelCleanString(source["first_mint_signature"]),
		creatorIntelCleanString(source["signature"]),
	))
	slot := creatorIntelInt64(source["slot"])
	observedAt := time.Now().UTC()
	if value := strings.TrimSpace(creatorIntelCleanString(source["observed_at"])); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			observedAt = parsed.UTC()
		}
	}

	verifiedFlag, _ := source["creator_relation_verified"].(bool)
	verificationStatus := "observed"
	if verifiedFlag && signature != "" && slot > 0 {
		verificationStatus = "verified"
	}
	program := strings.TrimSpace(firstNonEmptyString(
		creatorIntelCleanString(source["launch_platform"]),
		creatorIntelCleanString(source["program"]),
	))
	if program == "" {
		program = "pump.fun"
	}
	evidenceSource := strings.TrimSpace(creatorIntelCleanString(source["source"]))
	if evidenceSource == "" {
		evidenceSource = "canonical_token_radar"
	}

	item := services.ActorDefenseEvidenceRecord{
		Network: network, ActorWallet: creator,
		CounterpartKind: "token", CounterpartID: mint,
		Relation: "created_token", VerificationStatus: verificationStatus,
		EvidenceKey: "canonical_creator_relation:" + mint,
		Source:      evidenceSource, Signature: signature, Slot: slot, ObservedAt: observedAt,
		TokenMint: mint, OccurrenceCount: 1,
		Metadata: map[string]any{
			"actor_role":                     "creator_deployer",
			"source_wallet":                  creator,
			"destination_wallet":             mint,
			"program":                        program,
			"creator_relation_verified_flag": verifiedFlag,
			"creator_relation_scope":         creatorIntelCleanString(source["creator_scope"]),
			"source_event_type":              creatorIntelCleanString(source["event_type"]),
			"source_module_id":               creatorIntelCleanString(source["module_id"]),
			"request_scope_actor_evidence":   true,
			"persistent_actor_index":         false,
			"identity_or_wrongdoing_claim":   false,
		},
	}
	out.Evidence = item
	creationSignature := ""
	if verificationStatus == "verified" {
		creationSignature = signature
	}
	out.Target = services.ActorDistributionTarget{
		CreatorWallet: creator, Mint: mint, CreationSignature: creationSignature,
		VerificationStatus: verificationStatus, FirstObservedAt: observedAt, LastObservedAt: observedAt,
	}
	out.Status = verificationStatus
	if verificationStatus != "verified" {
		gaps := []string{}
		if !verifiedFlag {
			gaps = append(gaps, "verified_source_flag")
		}
		if signature == "" {
			gaps = append(gaps, "signature")
		}
		if slot <= 0 {
			gaps = append(gaps, "slot")
		}
		out.Limitations = append(out.Limitations, "Request-scope creator → mint ilişkisi OBSERVED kaldı; eksik doğrulama alanları: "+strings.Join(gaps, ", ")+".")
	}
	return out
}

// applyRequestScopeCanonicalCreatorRelation adds the current token relation to
// an in-memory dossier without creating persistent-memory semantics.
func applyRequestScopeCanonicalCreatorRelation(dossier services.ActorDefenseDossier, relation actorCreatorRelationRun) services.ActorDefenseDossier {
	mint := strings.TrimSpace(relation.Target.Mint)
	if mint == "" || strings.TrimSpace(relation.Evidence.EvidenceKey) == "" {
		return dossier
	}
	if dossier.Tokens == nil {
		dossier.Tokens = []services.ActorDefenseTokenObservation{}
	}
	if dossier.Evidence == nil {
		dossier.Evidence = []services.ActorDefenseEvidenceRecord{}
	}

	tokenSeen := false
	for _, token := range dossier.Tokens {
		if strings.TrimSpace(token.Mint) == mint {
			tokenSeen = true
			break
		}
	}
	if !tokenSeen {
		dossier.Tokens = append(dossier.Tokens, services.ActorDefenseTokenObservation{
			Mint: mint, Roles: []string{"creator"}, CreatorSignature: strings.TrimSpace(relation.Target.CreationSignature),
			FirstObservedAt: relation.Target.FirstObservedAt, LastObservedAt: relation.Target.LastObservedAt,
		})
	}

	identity := requestScopeActorEvidenceIdentity(relation.Evidence)
	for _, item := range dossier.Evidence {
		if requestScopeActorEvidenceIdentity(item) == identity {
			return dossier
		}
	}
	dossier.Evidence = append(dossier.Evidence, relation.Evidence)
	return dossier
}
