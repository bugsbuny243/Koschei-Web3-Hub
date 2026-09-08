package handlers

import (
	"testing"
	"time"

	"koschei/api/internal/services"
)

func TestTransactionGuardUnifiedSecurityProjectsVerifiedDelegateAsProspectiveCapability(t *testing.T) {
	wallet := guardV3TestAddress(101)
	account := guardV3TestAddress(102)
	delegate := guardV3TestAddress(103)
	active := true
	now := time.Date(2026, 9, 8, 1, 0, 0, 0, time.UTC)

	authority := transactionGuardAuthoritySurfaceAnalysis{
		Requested: true,
		Required:  true,
		Available: true,
		Complete:  true,
		Events: []transactionGuardAuthorityEvent{{
			InstructionSource:     "outer",
			InstructionIndex:      2,
			ProgramID:             guardV3SPLTokenProgramID,
			Kind:                  "approve",
			Account:               account,
			Source:                account,
			CurrentAuthority:      wallet,
			Delegate:              delegate,
			Scope:                 "single_token_account_allowance",
			AmountRaw:             "5000",
			Persistent:            true,
			PostStateAvailable:    true,
			ActiveAfterSimulation: &active,
			EvidenceStatus:        "verified_final_simulated_token_account_state",
			Explanation:           "Delegate remains active after simulation.",
		}},
	}
	attackPath := transactionGuardAttackPathAnalysis{
		Version:   transactionGuardAttackPathVersion,
		Available: true,
		Complete:  true,
		Status:    "attack_path_observed",
		Paths: []transactionGuardAttackPath{{
			ID:         "preflight-path-1",
			Title:      "Evidence-linked pre-signing risk path",
			Confidence: "high",
			Impact:     "delegate authority can move tokens after approval",
			Steps: []transactionGuardAttackPathStep{{
				Sequence:     1,
				Layer:        "authority_surface",
				Kind:         "approve",
				Subject:      account,
				Counterparty: delegate,
				ProgramID:    guardV3SPLTokenProgramID,
				Severity:     "high",
				Evidence:     "delegate remains active after simulated signing",
			}},
		}},
	}

	projection := buildTransactionGuardUnifiedSecurityProjection(
		transactionGuardV2Request{Network: "solana-mainnet", Wallet: wallet},
		"req-1",
		"fingerprint-1",
		authority,
		attackPath,
		now,
	)

	if projection.BindingStatus != "consistent" || len(projection.BindingIssues) != 0 {
		t.Fatalf("projection bindings=%#v", projection.BindingIssues)
	}
	if projection.Base.Decision.Status != services.IntelligenceEvidenceUnverified || projection.Base.Decision.Action != "investigate" {
		t.Fatalf("base decision was regraded: %#v", projection.Base.Decision)
	}
	if len(projection.Capabilities) != 1 {
		t.Fatalf("capabilities=%#v", projection.Capabilities)
	}
	capability := projection.Capabilities[0]
	delegateSubject := services.ClassifyIntelligenceSubject(delegate, "solana-mainnet")
	if capability.SubjectID != delegateSubject.ID || capability.Kind != services.IntelligenceCapabilitySpend {
		t.Fatalf("capability=%#v", capability)
	}
	if capability.Status != services.IntelligenceEvidenceVerified || capability.Lifecycle != services.IntelligenceCapabilityLifecycleProspective {
		t.Fatalf("verified simulated authority must remain prospective: %#v", capability)
	}
	if len(projection.Actions) != 1 || projection.Actions[0].Status != services.IntelligenceEvidenceVerified {
		t.Fatalf("actions=%#v", projection.Actions)
	}
	if len(projection.BoundaryTransitions) != 0 {
		t.Fatalf("same-domain preflight must not fabricate a trust-boundary transition: %#v", projection.BoundaryTransitions)
	}
	if len(projection.AttackPaths) != 1 || projection.AttackPaths[0].Status != services.IntelligenceEvidenceInferred {
		t.Fatalf("pre-signing path must remain inferred: %#v", projection.AttackPaths)
	}
	if projection.AttackPaths[0].EntrySubjectID != projection.AttackPaths[0].Steps[0].SubjectID {
		t.Fatal("path entry does not identify the first projected actor")
	}
	if len(projection.Base.Evidence) != 2 {
		t.Fatalf("expected authority and attack-path evidence, got %#v", projection.Base.Evidence)
	}
}

func TestTransactionGuardUnifiedSecurityDoesNotClaimCapabilityWhenPostStateUnavailable(t *testing.T) {
	wallet := guardV3TestAddress(111)
	account := guardV3TestAddress(112)
	delegate := guardV3TestAddress(113)

	projection := buildTransactionGuardUnifiedSecurityProjection(
		transactionGuardV2Request{Network: "solana-mainnet", Wallet: wallet},
		"req-2",
		"fingerprint-2",
		transactionGuardAuthoritySurfaceAnalysis{
			Requested: true,
			Required:  true,
			Available: true,
			Complete:  false,
			Events: []transactionGuardAuthorityEvent{{
				InstructionSource: "outer",
				InstructionIndex:  1,
				ProgramID:         guardV3SPLTokenProgramID,
				Kind:              "approve",
				Account:           account,
				Source:            account,
				CurrentAuthority:  wallet,
				Delegate:          delegate,
				Scope:             "single_token_account_allowance",
				Persistent:        true,
				EvidenceStatus:    "decoded_instruction_post_state_unavailable",
				Explanation:       "Post-state was unavailable.",
			}},
		},
		transactionGuardAttackPathAnalysis{},
		time.Date(2026, 9, 8, 1, 5, 0, 0, time.UTC),
	)

	if len(projection.Capabilities) != 0 {
		t.Fatalf("missing post-state must not produce a prospective capability: %#v", projection.Capabilities)
	}
	if len(projection.Actions) != 1 || projection.Actions[0].Status != services.IntelligenceEvidenceUnverified {
		t.Fatalf("incomplete authority evidence must fail closed: %#v", projection.Actions)
	}
	if len(projection.Base.Evidence) != 1 || projection.Base.Evidence[0].Status != services.IntelligenceEvidenceUnverified {
		t.Fatalf("evidence=%#v", projection.Base.Evidence)
	}
}

func TestTransactionGuardUnifiedSecurityCanProjectObservedPermanentDelegateWithoutInventingActor(t *testing.T) {
	mint := guardV3TestAddress(121)
	delegate := guardV3TestAddress(122)

	projection := buildTransactionGuardUnifiedSecurityProjection(
		transactionGuardV2Request{Network: "solana-mainnet"},
		"req-3",
		"fingerprint-3",
		transactionGuardAuthoritySurfaceAnalysis{
			Requested: true,
			Required:  true,
			Available: true,
			Complete:  true,
			Events: []transactionGuardAuthorityEvent{{
				InstructionSource: "outer",
				InstructionIndex:  0,
				ProgramID:         guardV3Token2022ProgramID,
				Kind:              "initialize_permanent_delegate",
				Account:           mint,
				Mint:              mint,
				Delegate:          delegate,
				NewAuthority:      delegate,
				Scope:             "all_token_accounts_for_mint",
				Persistent:        true,
				MintWide:          true,
				CanTransfer:       true,
				CanBurn:           true,
				EvidenceStatus:    "decoded_instruction",
				Explanation:       "Permanent delegate is configured by the simulated instruction.",
			}},
		},
		transactionGuardAttackPathAnalysis{},
		time.Date(2026, 9, 8, 1, 10, 0, 0, time.UTC),
	)

	if len(projection.Capabilities) != 1 {
		t.Fatalf("capabilities=%#v", projection.Capabilities)
	}
	capability := projection.Capabilities[0]
	if capability.Kind != services.IntelligenceCapabilityDelegate || capability.Status != services.IntelligenceEvidenceObserved {
		t.Fatalf("capability=%#v", capability)
	}
	if capability.Lifecycle != services.IntelligenceCapabilityLifecycleProspective {
		t.Fatalf("capability=%#v", capability)
	}
	if len(projection.Actions) != 0 {
		t.Fatalf("missing current authority must not be replaced with an invented actor: %#v", projection.Actions)
	}
}

func TestTransactionGuardUnifiedSecurityRejectsAmbiguousAuthorityEvidence(t *testing.T) {
	active := true
	event := transactionGuardAuthorityEvent{
		InstructionSource:     "outer",
		InstructionIndex:      1,
		Kind:                  "approve",
		Account:               guardV3TestAddress(132),
		CurrentAuthority:      guardV3TestAddress(131),
		Delegate:              guardV3TestAddress(133),
		Scope:                 "single_token_account_allowance",
		Persistent:            true,
		PostStateAvailable:    true,
		ActiveAfterSimulation: &active,
		EvidenceStatus:        "verified_final_simulated_token_account_state",
	}
	projection := buildTransactionGuardUnifiedSecurityProjection(
		transactionGuardV2Request{Network: "solana-mainnet", Wallet: event.CurrentAuthority},
		"fixture-request", "fixture-fingerprint",
		transactionGuardAuthoritySurfaceAnalysis{Events: []transactionGuardAuthorityEvent{event, event}},
		transactionGuardAttackPathAnalysis{}, time.Now(),
	)
	if projection.BindingStatus != "unverified" || len(projection.BindingIssues) == 0 {
		t.Fatalf("ambiguous authority evidence accepted: %#v", projection)
	}
	if len(projection.Capabilities) != 2 || len(projection.Actions) != 2 {
		t.Fatal("fixture did not exercise both capability and action claims")
	}
	for _, capability := range projection.Capabilities {
		if capability.Status != services.IntelligenceEvidenceUnverified || capability.Confidence != 0 {
			t.Fatalf("ambiguous capability retained a verified claim: %#v", capability)
		}
	}
	for _, action := range projection.Actions {
		if action.Status != services.IntelligenceEvidenceUnverified || action.Confidence != 0 {
			t.Fatalf("ambiguous action retained a verified claim: %#v", action)
		}
	}
	if len(projection.Base.Evidence) != 2 || projection.Base.Evidence[0].Status != services.IntelligenceEvidenceVerified || projection.Base.Decision.Status != services.IntelligenceEvidenceUnverified {
		t.Fatal("binding rejection altered the original evidence or decision")
	}
}
