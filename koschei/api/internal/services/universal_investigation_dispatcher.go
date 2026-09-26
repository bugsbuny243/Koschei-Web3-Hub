package services

import (
	"encoding/hex"
	"errors"
	"strings"
)

const (
	UniversalDispatchSolanaCore       = "solana_live_core"
	UniversalDispatchSolanaTx         = "solana_transaction_intelligence"
	UniversalDispatchEVMProbe         = "evm_read_only_probe"
	UniversalDispatchEVMTransaction   = "evm_transaction_receipt_probe"
	UniversalDispatchBitcoinProbe     = "bitcoin_read_only_probe"
	UniversalDispatchMoveIdentity     = "move_identity_probe"
	UniversalDispatchDeclaredOnly     = "declared_not_executable"
	UniversalDispatchPlannedAdapter   = "planned_adapter"
	UniversalDispatchResearchAdapter  = "research_adapter"
	UniversalDispatchUnsupported      = "unsupported_target"
	UniversalVerdictExistingRulesOnly = "existing_deterministic_rules_only"
	UniversalVerdictEvidenceOnly      = "evidence_only_no_grade_authority"
	UniversalVerdictNone              = "no_verdict_authority"
)

const UniversalDispatchBitcoinTransaction = "bitcoin_transaction_probe"

type UniversalInvestigationPlan struct {
	Subject                 IntelligenceSubject                  `json:"subject"`
	Adapter                 UniversalInvestigationAdapterProfile `json:"adapter"`
	Route                   string                               `json:"route"`
	Executable              bool                                 `json:"executable"`
	EvidenceOnly            bool                                 `json:"evidence_only"`
	VerdictAuthority        string                               `json:"verdict_authority"`
	RequiredEvidenceSources []string                             `json:"required_evidence_sources,omitempty"`
	Limitations             []string                             `json:"limitations,omitempty"`
}

func BuildUniversalInvestigationPlan(target, network, kindHint string) (UniversalInvestigationPlan, error) {
	subject, err := ClassifyUniversalInvestigationSubject(target, network, kindHint)
	if err != nil {
		return UniversalInvestigationPlan{}, err
	}

	profile, ok := universalInvestigationProfileForSubject(subject)
	if !ok {
		return UniversalInvestigationPlan{}, errors.New("no universal investigation adapter profile for subject")
	}
	if !universalProfileSupportsKind(profile, subject.Kind) {
		return UniversalInvestigationPlan{}, errors.New("selected adapter does not declare target kind")
	}

	plan := UniversalInvestigationPlan{
		Subject:                 subject,
		Adapter:                 profile,
		Route:                   UniversalDispatchDeclaredOnly,
		Executable:              false,
		EvidenceOnly:            true,
		VerdictAuthority:        UniversalVerdictNone,
		RequiredEvidenceSources: append([]string{}, profile.EvidenceSources...),
		Limitations:             []string{},
	}

	switch profile.Status {
	case UniversalAdapterResearch:
		plan.Route = UniversalDispatchResearchAdapter
		plan.Limitations = append(plan.Limitations, "Research adapter is observe/design-only and cannot execute production collection or affect a verdict.")
		return plan, nil
	case UniversalAdapterPlanned:
		plan.Route = UniversalDispatchPlannedAdapter
		plan.Limitations = append(plan.Limitations, "Adapter is declared for planning but no production collector is connected.")
		return plan, nil
	}

	switch subject.ChainFamily {
	case IntelligenceChainFamilySolana:
		return buildSolanaUniversalDispatchPlan(plan)
	case IntelligenceChainFamilyEVM:
		return buildEVMUniversalDispatchPlan(plan)
	case IntelligenceChainFamilyUTXO:
		return buildBitcoinUniversalDispatchPlan(plan)
	case IntelligenceChainFamilyMove:
		return buildMoveUniversalDispatchPlan(plan)
	default:
		plan.Limitations = append(plan.Limitations, "No executable production dispatcher exists for this chain family.")
		return plan, nil
	}
}

func UniversalInvestigationPlanForCustomerScanTarget(target CustomerScanTarget) (UniversalInvestigationPlan, error) {
	if target.RequiresNetwork {
		return UniversalInvestigationPlan{}, errors.New("customer target requires network context before universal dispatch")
	}
	kind := ""
	switch target.Kind {
	case CustomerScanTargetEVMAddress, CustomerScanTargetSolana, CustomerScanTargetBitcoin:
		kind = IntelligenceSubjectAddress
	case CustomerScanTargetTxHash:
		kind = IntelligenceSubjectTransaction
	default:
		return UniversalInvestigationPlan{}, errors.New("customer target kind has no universal dispatch mapping")
	}
	return BuildUniversalInvestigationPlan(target.Raw, target.NetworkHint, kind)
}

func universalInvestigationProfileForSubject(subject IntelligenceSubject) (UniversalInvestigationAdapterProfile, bool) {
	if subject.ChainFamily != IntelligenceChainFamilyOffchain {
		return UniversalInvestigationProfileForNetwork(subject.Network)
	}
	for _, profile := range UniversalInvestigationAdapterProfiles() {
		if profile.ChainFamily != IntelligenceChainFamilyOffchain {
			continue
		}
		if universalProfileSupportsKind(profile, subject.Kind) {
			return profile, true
		}
	}
	return UniversalInvestigationAdapterProfile{}, false
}

func buildSolanaUniversalDispatchPlan(plan UniversalInvestigationPlan) (UniversalInvestigationPlan, error) {
	switch plan.Subject.Kind {
	case IntelligenceSubjectAddress, IntelligenceSubjectToken:
		if classified := ClassifyIntelligenceSubject(plan.Subject.Raw, plan.Subject.Network); classified.ChainFamily != IntelligenceChainFamilySolana {
			return UniversalInvestigationPlan{}, errors.New("Solana executable target failed canonical address validation")
		}
		plan.Route = UniversalDispatchSolanaCore
		plan.Executable = true
		plan.EvidenceOnly = false
		plan.VerdictAuthority = UniversalVerdictExistingRulesOnly
	case IntelligenceSubjectTransaction:
		if strings.TrimSpace(plan.Subject.Raw) == "" {
			return UniversalInvestigationPlan{}, errors.New("Solana transaction target is required")
		}
		plan.Route = UniversalDispatchSolanaTx
		plan.Executable = true
		plan.EvidenceOnly = true
		plan.VerdictAuthority = UniversalVerdictEvidenceOnly
	default:
		plan.Route = UniversalDispatchDeclaredOnly
		plan.Limitations = append(plan.Limitations, "Solana family declares this target kind, but the universal dispatcher has not connected a standalone collector for it yet.")
	}
	return plan, nil
}

func buildEVMUniversalDispatchPlan(plan UniversalInvestigationPlan) (UniversalInvestigationPlan, error) {
	switch plan.Subject.Kind {
	case IntelligenceSubjectAddress, IntelligenceSubjectContract, IntelligenceSubjectToken:
		classified := ClassifyIntelligenceSubject(plan.Subject.Raw, plan.Subject.Network)
		if classified.ChainFamily != IntelligenceChainFamilyEVM {
			return UniversalInvestigationPlan{}, errors.New("EVM executable target must be a canonical 20-byte address")
		}
		plan.Route = UniversalDispatchEVMProbe
		plan.Executable = true
		plan.EvidenceOnly = true
		plan.VerdictAuthority = UniversalVerdictEvidenceOnly
		plan.Limitations = append(plan.Limitations, "Current EVM dispatcher collects chain-identity, bytecode/delegation and authority evidence only; no deterministic EVM grade is authorized.")
	case IntelligenceSubjectTransaction:
		raw := strings.ToLower(strings.TrimSpace(plan.Subject.Raw))
		if len(raw) != 66 || !strings.HasPrefix(raw, "0x") {
			return UniversalInvestigationPlan{}, errors.New("EVM executable transaction target must be a canonical 32-byte hash")
		}
		decoded, err := hex.DecodeString(raw[2:])
		if err != nil || len(decoded) != 32 {
			return UniversalInvestigationPlan{}, errors.New("EVM executable transaction target must be a canonical 32-byte hash")
		}
		plan.Route = UniversalDispatchEVMTransaction
		plan.Executable = true
		plan.EvidenceOnly = true
		plan.VerdictAuthority = UniversalVerdictEvidenceOnly
		plan.Limitations = append(plan.Limitations, "EVM transaction and receipt evidence is observational only; execution success or revert does not imply safety or authorization.")
	default:
		plan.Route = UniversalDispatchDeclaredOnly
		plan.Limitations = append(plan.Limitations, "EVM target kind is declared in the universal model but no production transaction/block/bridge/governance collector is connected yet.")
	}
	return plan, nil
}

func buildBitcoinUniversalDispatchPlan(plan UniversalInvestigationPlan) (UniversalInvestigationPlan, error) {
	switch plan.Subject.Kind {
	case IntelligenceSubjectAddress:
		classified := ClassifyIntelligenceSubject(plan.Subject.Raw, plan.Subject.Network)
		if classified.ChainFamily != IntelligenceChainFamilyUTXO || classified.Chain != "bitcoin" {
			return UniversalInvestigationPlan{}, errors.New("Bitcoin executable target failed canonical address validation")
		}
		plan.Route = UniversalDispatchBitcoinProbe
		plan.Executable = true
		plan.EvidenceOnly = true
		plan.VerdictAuthority = UniversalVerdictEvidenceOnly
		plan.Limitations = append(plan.Limitations, "Current Bitcoin dispatcher exposes address activity evidence only; ownership and safety are not inferred.")
	case IntelligenceSubjectTransaction:
		raw := strings.ToLower(strings.TrimSpace(plan.Subject.Raw))
		if len(raw) != 64 {
			return UniversalInvestigationPlan{}, errors.New("Bitcoin executable transaction target must be a canonical 32-byte txid")
		}
		decoded, err := hex.DecodeString(raw)
		if err != nil || len(decoded) != 32 {
			return UniversalInvestigationPlan{}, errors.New("Bitcoin executable transaction target must be a canonical 32-byte txid")
		}
		plan.Route = UniversalDispatchBitcoinTransaction
		plan.Executable = true
		plan.EvidenceOnly = true
		plan.VerdictAuthority = UniversalVerdictEvidenceOnly
		plan.Limitations = append(plan.Limitations, "Bitcoin transaction evidence is observational only; confirmation or fee state does not imply safety, ownership or intent.")
	default:
		plan.Route = UniversalDispatchDeclaredOnly
		plan.Limitations = append(plan.Limitations, "Bitcoin block kinds remain declared but do not yet have a standalone customer collector.")
	}
	return plan, nil
}

func buildMoveUniversalDispatchPlan(plan UniversalInvestigationPlan) (UniversalInvestigationPlan, error) {
	switch plan.Subject.Kind {
	case IntelligenceSubjectAddress:
		classified := ClassifyIntelligenceSubject(plan.Subject.Raw, plan.Subject.Network)
		if classified.ChainFamily != IntelligenceChainFamilyMove ||
			(classified.Chain != "sui" && classified.Chain != "aptos") {
			return UniversalInvestigationPlan{}, errors.New("Move executable target failed canonical address validation")
		}
		plan.Route = UniversalDispatchMoveIdentity
		plan.Executable = true
		plan.EvidenceOnly = true
		plan.VerdictAuthority = UniversalVerdictEvidenceOnly
		plan.Limitations = append(
			plan.Limitations,
			"Move identity probes verify canonical address syntax and selected mainnet identity only; address existence, ownership, balances, objects/resources and safety are not inferred.",
		)
	default:
		plan.Route = UniversalDispatchDeclaredOnly
		plan.Limitations = append(
			plan.Limitations,
			"Move target kind is declared in the universal model but only canonical address identity probes are connected.",
		)
	}
	return plan, nil
}
