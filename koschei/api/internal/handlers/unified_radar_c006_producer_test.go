package handlers

import (
	"strings"
	"testing"
	"time"

	"koschei/api/internal/services"
)

func c006ProducerReport() services.ActorInitialRecipientReport {
	return services.ActorInitialRecipientReport{
		Mint: "MintA", CreatorWallet: "CreatorA", Status: "initial_recipients_resolved",
		Recipients: []services.ActorInitialRecipient{{
			Wallet: "HolderA", DestinationTokenAccount: "HolderATA", Amount: 650,
			Signature: "SigA", Slot: 42, ObservedAt: time.Date(2026, 10, 7, 1, 0, 0, 0, time.UTC),
			VerificationStatus: "verified",
		}},
	}
}

func TestC006ProducerUsesRequestScopeDominantHolderRestore(t *testing.T) {
	holder := services.HolderIntelligence{
		Available: true, Supply: 1000,
		Rows: []services.HolderIntelligenceRow{{
			OwnerWallet: "HolderA", OwnerResolved: true,
			RepeatDominantMatches: []services.RepeatDominantHolderMatch{{Mint: "MintB", Rank: 1, Percentage: 81.5}},
		}},
	}
	got := buildRequestScopeC006Relation(t.Context(), nil, c006ProducerReport(), holder)
	if !got.Available || got.Status != "verified" || !got.RecipientOwnerResolved {
		t.Fatalf("relation=%#v", got)
	}
	if got.TransferSignature != "SigA" || got.Slot != 42 || len(got.OtherTokens) != 1 || got.OtherTokens[0].Mint != "MintB" {
		t.Fatalf("evidence refs=%#v", got)
	}
}

func TestC006ProducerUnavailableMemoryDoesNotBecomeSafeEmpty(t *testing.T) {
	got := buildRequestScopeC006Relation(t.Context(), nil, c006ProducerReport(), services.HolderIntelligence{})
	if got.Available || got.Status != "cross_token_memory_unavailable" {
		t.Fatalf("relation=%#v", got)
	}
	if len(got.Limitations) == 0 || !strings.Contains(got.Limitations[len(got.Limitations)-1], "not evaluated") {
		t.Fatalf("limitations=%#v", got.Limitations)
	}
}

func TestC006ProducerRejectsUnverifiedTransfer(t *testing.T) {
	report := c006ProducerReport()
	report.Recipients[0].VerificationStatus = "observed"
	holder := services.HolderIntelligence{Rows: []services.HolderIntelligenceRow{{OwnerWallet: "HolderA", OwnerResolved: true}}}
	got := buildRequestScopeC006Relation(t.Context(), nil, report, holder)
	if got.Available || got.Status != "recipient_evidence_unavailable" || got.TransferSignature != "" {
		t.Fatalf("relation=%#v", got)
	}
}
