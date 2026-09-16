package handlers

import (
	"testing"
	"time"

	"koschei/api/internal/services"
)

func TestRequestScopeCanonicalCreatorRelationPreservesVerifiedEvidence(t *testing.T) {
	observed := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	core := holderIntelligenceCoreResult{
		Request: holderIntelligenceCoreRequest{Target: "Mint111", Network: "solana-mainnet"},
		SourceContext: map[string]any{
			"creator_wallet":            "Creator111",
			"creator_relation_verified": true,
			"creation_signature":        "Sig111",
			"slot":                      int64(444),
			"observed_at":               observed.Format(time.RFC3339),
			"source":                    "solana_rpc_create_transaction",
		},
	}
	relation := buildRequestScopeCanonicalCreatorMintRelation(core, "Creator111", "solana-mainnet")
	if relation.Status != "verified" || relation.Persistence != "request_scope" {
		t.Fatalf("relation status/persistence = %q/%q", relation.Status, relation.Persistence)
	}
	if relation.Evidence.VerificationStatus != "verified" || relation.Evidence.Signature != "Sig111" || relation.Evidence.Slot != 444 {
		t.Fatalf("verified relation evidence = %#v", relation.Evidence)
	}
	if relation.Evidence.Metadata["request_scope_actor_evidence"] != true || relation.Evidence.Metadata["persistent_actor_index"] != false {
		t.Fatalf("request-scope provenance = %#v", relation.Evidence.Metadata)
	}

	dossier := services.ActorDefenseDossier{Tokens: []services.ActorDefenseTokenObservation{}, Evidence: []services.ActorDefenseEvidenceRecord{}}
	dossier = applyRequestScopeCanonicalCreatorRelation(dossier, relation)
	dossier = applyRequestScopeCanonicalCreatorRelation(dossier, relation)
	if len(dossier.Tokens) != 1 || dossier.Tokens[0].Mint != "Mint111" {
		t.Fatalf("token projection = %#v", dossier.Tokens)
	}
	if len(dossier.Evidence) != 1 || dossier.Evidence[0].EvidenceKey != "canonical_creator_relation:Mint111" {
		t.Fatalf("evidence projection = %#v", dossier.Evidence)
	}
}

func TestRequestScopeCanonicalCreatorRelationNeverUpgradesIncompleteSource(t *testing.T) {
	core := holderIntelligenceCoreResult{
		Request: holderIntelligenceCoreRequest{Target: "Mint222", Network: "solana-mainnet"},
		SourceContext: map[string]any{
			"creator_wallet":            "Creator222",
			"creator_relation_verified": true,
			"creation_signature":        "Sig222",
			"slot":                      int64(0),
		},
	}
	relation := buildRequestScopeCanonicalCreatorMintRelation(core, "Creator222", "solana-mainnet")
	if relation.Status != "observed" || relation.Evidence.VerificationStatus != "observed" {
		t.Fatalf("incomplete source upgraded to verified: %#v", relation)
	}
	if relation.Evidence.Metadata["persistent_actor_index"] != false {
		t.Fatalf("incomplete source claimed persistent memory: %#v", relation.Evidence.Metadata)
	}
}

func TestRequestScopeCanonicalCreatorRelationExternalObservationStaysObserved(t *testing.T) {
	core := holderIntelligenceCoreResult{
		Request: holderIntelligenceCoreRequest{Target: "Mint333", Network: "solana-mainnet"},
		SourceContext: map[string]any{
			"creator_wallet":            "Creator333",
			"creator_relation_verified": false,
			"creation_signature":        "ExternalSig333",
			"slot":                      int64(333),
			"source":                    "helius_target_mint_archival",
		},
	}
	relation := buildRequestScopeCanonicalCreatorMintRelation(core, "Creator333", "solana-mainnet")
	if relation.Status != "observed" || relation.Evidence.VerificationStatus != "observed" {
		t.Fatalf("external attribution upgraded: %#v", relation)
	}
	if relation.Evidence.Source != "helius_target_mint_archival" {
		t.Fatalf("evidence source = %q", relation.Evidence.Source)
	}
}
