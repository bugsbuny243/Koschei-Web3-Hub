package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"koschei/api/internal/services"
)

func buildTransactionGuardUnifiedSecurityProjection(
	input transactionGuardV2Request,
	requestID string,
	fingerprint string,
	authority transactionGuardAuthoritySurfaceAnalysis,
	attackPath transactionGuardAttackPathAnalysis,
	now time.Time,
) services.UnifiedSecurityInvestigation {
	network := strings.TrimSpace(input.Network)
	if network == "" {
		network = "solana-mainnet"
	}

	subjects := []services.IntelligenceSubject{}
	walletSubject, walletOK := transactionGuardUnifiedSubject(input.Wallet, network)
	if walletOK {
		subjects = append(subjects, walletSubject)
	}
	base := services.BuildIntelligenceInvestigation(subjects, now)
	unified := services.BuildUnifiedSecurityInvestigation(base, now)

	if network != "solana-mainnet" {
		return services.FinalizeUnifiedSecurityInvestigation(unified)
	}

	for _, event := range authority.Events {
		status := transactionGuardUnifiedEvidenceStatus(event.EvidenceStatus)
		evidenceID := transactionGuardUnifiedAuthorityEvidenceID(fingerprint, event)
		evidenceSubject, evidenceSubjectOK := transactionGuardUnifiedAuthorityEvidenceSubject(event, network)
		if evidenceSubjectOK {
			appendIntelligenceSubjectIfMissing(&base, evidenceSubject)
		}

		attributes := map[string]any{
			"request_id":                strings.TrimSpace(requestID),
			"transaction_fingerprint":   strings.TrimSpace(fingerprint),
			"instruction_source":        strings.TrimSpace(event.InstructionSource),
			"instruction_index":         event.InstructionIndex,
			"inner_sequence":            event.InnerSequence,
			"authority_evidence_status": strings.TrimSpace(event.EvidenceStatus),
			"persistent":                event.Persistent,
			"mint_wide":                 event.MintWide,
			"can_transfer":              event.CanTransfer,
			"can_burn":                  event.CanBurn,
		}
		if event.ActiveAfterSimulation != nil {
			attributes["active_after_simulation"] = *event.ActiveAfterSimulation
		}
		if event.AmountRaw != "" {
			attributes["amount_raw"] = event.AmountRaw
		}
		if event.AuthorityTypeName != "" {
			attributes["authority_type_name"] = event.AuthorityTypeName
		}

		base.Evidence = append(base.Evidence, services.IntelligenceEvidence{
			ID:          evidenceID,
			SubjectID:   evidenceSubject.ID,
			ChainFamily: services.IntelligenceChainFamilySolana,
			Chain:       "solana",
			Network:     network,
			Source:      "transaction_guard_v3_authority_surface",
			Status:      status,
			Address:     evidenceSubject.Raw,
			Contract:    strings.TrimSpace(event.ProgramID),
			Method:      strings.TrimSpace(event.Kind),
			StateChange: firstNonEmptyString(strings.TrimSpace(event.Explanation), guardV3AuthorityEvidence(event)),
			Provenance:  "transaction_guard_pre_signing_authority_evidence",
			Confidence:  transactionGuardUnifiedEvidenceConfidence(status),
			Attributes:  attributes,
		})

		actorSubject, actorOK := transactionGuardUnifiedSubject(event.CurrentAuthority, network)
		if actorOK {
			appendIntelligenceSubjectIfMissing(&base, actorSubject)
		}
		targetRaw := transactionGuardUnifiedAuthorityTarget(event)
		targetSubject, targetOK := transactionGuardUnifiedSubject(targetRaw, network)
		if targetOK {
			appendIntelligenceSubjectIfMissing(&base, targetSubject)
		}

		if holderRaw, capabilityKind, ok := transactionGuardUnifiedProspectiveCapability(event, status); ok {
			holderSubject, holderOK := transactionGuardUnifiedSubject(holderRaw, network)
			if holderOK {
				appendIntelligenceSubjectIfMissing(&base, holderSubject)
				delegatedBy := ""
				if actorOK {
					delegatedBy = actorSubject.ID
				}
				capability := services.BuildProspectiveIntelligenceCapability(
					holderSubject.ID,
					services.UnifiedSecurityDomainBlockchain,
					capabilityKind,
					transactionGuardUnifiedScope(event),
					transactionGuardUnifiedAuthorityResource(event),
					delegatedBy,
					status,
					transactionGuardUnifiedCapabilityConstraints(event),
					[]string{evidenceID},
					transactionGuardUnifiedEvidenceConfidence(status),
				)
				if capability.Status != services.IntelligenceEvidenceUnverified {
					unified.Capabilities = append(unified.Capabilities, capability)
				}
			}
		}

		if actorOK && strings.TrimSpace(event.Kind) != "" {
			targetID := ""
			if targetOK {
				targetID = targetSubject.ID
			}
			action := services.BuildIntelligenceAction(
				actorSubject.ID,
				"",
				targetID,
				services.UnifiedSecurityDomainBlockchain,
				strings.TrimSpace(event.Kind),
				transactionGuardUnifiedAuthorityResource(event),
				"",
				firstNonEmptyString(strings.TrimSpace(event.Explanation), guardV3AuthorityEvidence(event)),
				status,
				[]string{evidenceID},
				transactionGuardUnifiedEvidenceConfidence(status),
			)
			unified.Actions = append(unified.Actions, action)
		}
	}

	for _, path := range attackPath.Paths {
		steps := make([]services.UnifiedSecurityAttackPathStep, 0, len(path.Steps))
		pathEvidenceRefs := make([]string, 0, len(path.Steps))
		entrySubjectID := ""

		for _, step := range path.Steps {
			subject, subjectOK := transactionGuardUnifiedSubject(step.Subject, network)
			if !subjectOK && walletOK {
				subject, subjectOK = walletSubject, true
			}
			if !subjectOK || strings.TrimSpace(step.Evidence) == "" {
				continue
			}
			appendIntelligenceSubjectIfMissing(&base, subject)
			if entrySubjectID == "" {
				entrySubjectID = subject.ID
			}

			targetID := ""
			if target, ok := transactionGuardUnifiedSubject(step.Counterparty, network); ok {
				appendIntelligenceSubjectIfMissing(&base, target)
				targetID = target.ID
			}

			evidenceID := fmt.Sprintf(
				"txguard_path:%s:%s:%d",
				strings.TrimSpace(fingerprint),
				strings.TrimSpace(path.ID),
				step.Sequence,
			)
			pathEvidenceRefs = append(pathEvidenceRefs, evidenceID)
			base.Evidence = append(base.Evidence, services.IntelligenceEvidence{
				ID:          evidenceID,
				SubjectID:   subject.ID,
				ChainFamily: services.IntelligenceChainFamilySolana,
				Chain:       "solana",
				Network:     network,
				Source:      "transaction_guard_v3_attack_path",
				Status:      services.IntelligenceEvidenceObserved,
				Address:     subject.Raw,
				Contract:    strings.TrimSpace(step.ProgramID),
				Method:      strings.TrimSpace(step.Kind),
				StateChange: strings.TrimSpace(step.Evidence),
				Provenance:  "transaction_guard_pre_signing_attack_path_evidence",
				Confidence:  transactionGuardUnifiedPathConfidence(path.Confidence),
				Attributes: map[string]any{
					"request_id":              strings.TrimSpace(requestID),
					"transaction_fingerprint": strings.TrimSpace(fingerprint),
					"path_id":                 strings.TrimSpace(path.ID),
					"layer":                   strings.TrimSpace(step.Layer),
					"severity":                strings.TrimSpace(step.Severity),
					"counterparty":            strings.TrimSpace(step.Counterparty),
					"pre_signing":             true,
				},
			})

			steps = append(steps, services.UnifiedSecurityAttackPathStep{
				SubjectID:       subject.ID,
				TargetSubjectID: targetID,
				Effect:          strings.TrimSpace(step.Evidence),
				EvidenceRefs:    []string{evidenceID},
			})
		}

		if len(steps) == 0 || entrySubjectID == "" {
			continue
		}
		consequences := []services.IntelligenceConsequence{}
		if strings.TrimSpace(path.Impact) != "" {
			consequences = append(consequences, services.BuildIntelligenceConsequence(
				services.UnifiedSecurityDomainBlockchain,
				"pre_signing_impact",
				strings.TrimSpace(input.Wallet),
				strings.TrimSpace(path.Impact),
				"pre_signing_simulated_or_inferred_effect",
				services.IntelligenceEvidenceInferred,
				pathEvidenceRefs,
				transactionGuardUnifiedPathConfidence(path.Confidence),
			))
		}
		unified.AttackPaths = append(unified.AttackPaths, services.BuildUnifiedSecurityAttackPath(
			strings.TrimSpace(path.Title),
			entrySubjectID,
			services.IntelligenceEvidenceInferred,
			[]string{"pre_signing_only", "not_proof_of_malicious_intent_or_post_signing_causation"},
			steps,
			consequences,
			pathEvidenceRefs,
			transactionGuardUnifiedPathConfidence(path.Confidence),
		))
	}

	unified.Base = base
	return services.FinalizeUnifiedSecurityInvestigation(unified)
}

func transactionGuardUnifiedSubject(raw, network string) (services.IntelligenceSubject, bool) {
	raw = strings.TrimSpace(raw)
	if !looksLikeGuardPubkey(raw) {
		return services.IntelligenceSubject{}, false
	}
	subject := services.ClassifyIntelligenceSubject(raw, network)
	if subject.ChainFamily != services.IntelligenceChainFamilySolana {
		return services.IntelligenceSubject{}, false
	}
	return subject, true
}

func transactionGuardUnifiedAuthorityEvidenceSubject(event transactionGuardAuthorityEvent, network string) (services.IntelligenceSubject, bool) {
	for _, value := range []string{
		event.CurrentAuthority,
		event.Account,
		event.Source,
		event.Mint,
		event.Delegate,
		event.NewAuthority,
		event.TransferHookProgramID,
		event.ProgramID,
	} {
		if subject, ok := transactionGuardUnifiedSubject(value, network); ok {
			return subject, true
		}
	}
	return services.IntelligenceSubject{}, false
}

func transactionGuardUnifiedEvidenceStatus(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.HasPrefix(raw, "verified_"):
		return services.IntelligenceEvidenceVerified
	case raw == "decoded_instruction":
		return services.IntelligenceEvidenceObserved
	default:
		return services.IntelligenceEvidenceUnverified
	}
}

func transactionGuardUnifiedEvidenceConfidence(status string) float64 {
	switch status {
	case services.IntelligenceEvidenceVerified:
		return 1
	case services.IntelligenceEvidenceObserved:
		return 0.8
	case services.IntelligenceEvidenceInferred:
		return 0.6
	default:
		return 0
	}
}

func transactionGuardUnifiedPathConfidence(raw string) float64 {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "high":
		return 1
	case "medium":
		return 0.6
	case "low":
		return 0.3
	default:
		return 0
	}
}

func transactionGuardUnifiedAuthorityEvidenceID(fingerprint string, event transactionGuardAuthorityEvent) string {
	return fmt.Sprintf(
		"txguard_authority:%s:%s:%d:%d",
		strings.TrimSpace(fingerprint),
		strings.TrimSpace(event.InstructionSource),
		event.InstructionIndex,
		event.InnerSequence,
	)
}

func transactionGuardUnifiedAuthorityTarget(event transactionGuardAuthorityEvent) string {
	for _, value := range []string{event.NewAuthority, event.Delegate, event.TransferHookProgramID, event.Destination} {
		value = strings.TrimSpace(value)
		if value != "" && value != "revoked" {
			return value
		}
	}
	return ""
}

func transactionGuardUnifiedAuthorityResource(event transactionGuardAuthorityEvent) string {
	return firstNonEmptyString(event.Account, event.Mint, event.Source)
}

func transactionGuardUnifiedScope(event transactionGuardAuthorityEvent) string {
	if scope := strings.TrimSpace(event.Scope); scope != "" && scope != "not_applicable" {
		return scope
	}
	return "authority_surface"
}

func transactionGuardUnifiedProspectiveCapability(event transactionGuardAuthorityEvent, status string) (holder string, kind string, ok bool) {
	if status == services.IntelligenceEvidenceUnverified {
		return "", "", false
	}
	if event.ActiveAfterSimulation != nil && !*event.ActiveAfterSimulation {
		return "", "", false
	}

	switch event.Kind {
	case "approve", "approve_checked":
		if !event.Persistent || event.ActiveAfterSimulation == nil || !*event.ActiveAfterSimulation {
			return "", "", false
		}
		return strings.TrimSpace(event.Delegate), services.IntelligenceCapabilitySpend, looksLikeGuardPubkey(event.Delegate)
	case "set_authority":
		if !event.Persistent || strings.TrimSpace(event.NewAuthority) == "revoked" {
			return "", "", false
		}
		return strings.TrimSpace(event.NewAuthority), services.IntelligenceCapabilityTokenAuthority, looksLikeGuardPubkey(event.NewAuthority)
	case "initialize_permanent_delegate":
		if !event.Persistent {
			return "", "", false
		}
		holder = firstNonEmptyString(event.Delegate, event.NewAuthority)
		return holder, services.IntelligenceCapabilityDelegate, looksLikeGuardPubkey(holder)
	case "initialize_transfer_hook", "update_transfer_hook":
		if !event.Persistent {
			return "", "", false
		}
		return strings.TrimSpace(event.TransferHookProgramID), services.IntelligenceCapabilityExecutionHook, looksLikeGuardPubkey(event.TransferHookProgramID)
	default:
		return "", "", false
	}
}

func transactionGuardUnifiedCapabilityConstraints(event transactionGuardAuthorityEvent) []string {
	constraints := []string{}
	if event.AmountRaw != "" {
		constraints = append(constraints, "amount_raw="+event.AmountRaw)
	}
	if event.EffectivelyUnlimited {
		constraints = append(constraints, "effectively_unlimited=true")
	}
	if event.MintWide {
		constraints = append(constraints, "mint_wide=true")
	}
	if event.CanTransfer {
		constraints = append(constraints, "can_transfer=true")
	}
	if event.CanBurn {
		constraints = append(constraints, "can_burn=true")
	}
	if event.ActiveAfterSimulation != nil {
		constraints = append(constraints, "active_after_simulation="+strconv.FormatBool(*event.ActiveAfterSimulation))
	}
	return constraints
}
