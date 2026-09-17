package handlers

import (
	"testing"
	"time"

	"koschei/api/internal/services"
)

func TestRequestScopeActorDistributionEvidenceIsNonPersistent(t *testing.T) {
	observed := time.Date(2026, 9, 17, 1, 0, 0, 0, time.UTC)
	report := services.ActorInitialRecipientReport{
		Mint: "Mint111", CreatorWallet: "Creator111", Status: "initial_recipients_resolved",
		DistributionScope: "complete_creator_token_account_history", HistoryComplete: true,
		Recipients: []services.ActorInitialRecipient{{
			Sequence: 1, Wallet: "Recipient111", Amount: 42,
			Signature: "Sig111", Slot: 123, ObservedAt: observed,
			VerificationStatus: "verified", Fate: "still_holds",
		}},
	}
	evidence := requestScopeActorDistributionEvidence(report, "solana-mainnet")
	if len(evidence) != 1 {
		t.Fatalf("evidence count = %d, want 1", len(evidence))
	}
	item := evidence[0]
	if item.VerificationStatus != "verified" || item.Relation != "initial_token_recipient" {
		t.Fatalf("unexpected evidence = %#v", item)
	}
	if item.Metadata["request_scope_actor_evidence"] != true || item.Metadata["request_scope_distribution"] != true {
		t.Fatalf("request-scope provenance = %#v", item.Metadata)
	}
	if item.Metadata["persistent_actor_index"] != false {
		t.Fatalf("request-scope evidence claimed persistent index = %#v", item.Metadata)
	}
}

func TestApplyRequestScopeActorDistributionEvidenceIsIdempotent(t *testing.T) {
	item := services.ActorDefenseEvidenceRecord{
		Network: "solana-mainnet", ActorWallet: "Creator111", CounterpartKind: "wallet",
		CounterpartID: "Recipient111", Relation: "initial_token_recipient", VerificationStatus: "verified",
		EvidenceKey: "Sig111:initial_token_recipient:1", Signature: "Sig111", Slot: 123,
		ObservedAt: time.Now().UTC(), Metadata: map[string]any{"persistent_actor_index": false},
	}
	dossier := services.ActorDefenseDossier{Evidence: []services.ActorDefenseEvidenceRecord{}}
	dossier = applyRequestScopeActorDistributionEvidence(dossier, []services.ActorDefenseEvidenceRecord{item})
	dossier = applyRequestScopeActorDistributionEvidence(dossier, []services.ActorDefenseEvidenceRecord{item})
	if len(dossier.Evidence) != 1 {
		t.Fatalf("evidence count = %d, want 1", len(dossier.Evidence))
	}
}

func TestRequestScopeDistributionObservedCreatorDoesNotProduceDerivedEvidence(t *testing.T) {
	relation := actorCreatorRelationRun{
		Status: "observed", Persistence: "request_scope",
		Target: services.ActorDistributionTarget{
			CreatorWallet: "Creator111", Mint: "Mint111", VerificationStatus: "observed",
		},
		Limitations: []string{},
	}
	out, evidence := (&Handler{}).collectRequestScopeCanonicalActorDistribution(t.Context(), relation, "solana-mainnet")
	if out.Status != "creator_mint_relation_observed_only" {
		t.Fatalf("status = %q", out.Status)
	}
	if len(evidence) != 0 || out.EvidenceProduced != 0 {
		t.Fatalf("observed-only creator produced derived evidence: %#v", evidence)
	}
}
