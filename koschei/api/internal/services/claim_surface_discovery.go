package services

import (
	"fmt"
	"strings"
)

// Only program IDs already pinned in the Koschei repository belong here.
// This first-pass lookup is intentionally limited to holder-owner program
// observations; it does not claim to inspect all distributors or all transfers.
// Additional IDs require independently captured account-response fixtures.
var knownClaimDistributorPrograms = map[string]string{
	"strmRqUCoQUgGUan5YhzUZa6KqdzwX5L6FpUxfmKg5m": "streamflow",
}

const claimHolderDiscoveryScope = "top_token_holder_owner_programs"
const claimDiscoveryLimitation = "UNKNOWN_CUSTOM_DISTRIBUTOR_NOT_RULED_OUT"

type claimHolderDiscovery struct {
	Attempted bool
	Found     bool
	ProgramID string
	Program   string
	Account   string
	Balance   float64
	Searched  string
	Scope     string
	Reason    string
}

// discoverClaimFromHolderRoles reuses real parsed owner-program observations
// collected by AnalyzeSolanaHolderRoles. A missing hit is NOT proof that the
// mint has no claim surface: this pass checks only the bounded top holder set.
func discoverClaimFromHolderRoles(p radarEvidenceProfile) claimHolderDiscovery {
	result := claimHolderDiscovery{
		Searched: fmt.Sprintf("%d repository-pinned distributor program(s)", len(knownClaimDistributorPrograms)),
		Scope:    claimHolderDiscoveryScope,
		Reason:   "claim_holder_evidence_unavailable",
	}
	if !p.IsTokenMint || !p.HolderRoles.Available || len(p.HolderRoles.Accounts) == 0 {
		return result
	}
	result.Attempted = true
	result.Reason = "no_known_distributor_program_found"
	for _, account := range p.HolderRoles.Accounts {
		programID := strings.TrimSpace(account.OwnerProgram)
		program, known := knownClaimDistributorPrograms[programID]
		if !known || strings.TrimSpace(account.TokenAccount) == "" {
			continue
		}
		result.Found = true
		result.ProgramID = programID
		result.Program = program
		result.Account = account.TokenAccount
		result.Balance = account.Balance
		result.Reason = "known_distributor_program_observed"
		return result
	}
	return result
}

// buildClaimDiscoveryArms never invents a website from a mint. When a URL is
// explicitly supplied, the existing URL-only parsers remain authoritative.
// Otherwise the two claim arms consume the same bounded on-chain observation.
func buildClaimDiscoveryArms(req SecurityRadarRequest, profile radarEvidenceProfile, generatedAt string) (SecurityRadarVerdict, SecurityRadarVerdict) {
	if urlEvidence := parseArvisClaimSurface(req.Target); urlEvidence.Available {
		return buildWalletlessClaimArm(req, urlEvidence, generatedAt), buildClaimSurfaceArm(req, urlEvidence, generatedAt)
	}
	discovery := discoverClaimFromHolderRoles(profile)
	return claimDiscoveryArm("Walletless Claim Shield", ModuleWalletlessClaimShield, req, discovery, generatedAt),
		claimDiscoveryArm("Claim Surface Risk", ModuleClaimSurfaceRisk, req, discovery, generatedAt)
}

func claimDiscoveryArm(module, moduleID string, req SecurityRadarRequest, discovery claimHolderDiscovery, generatedAt string) SecurityRadarVerdict {
	if discovery.Found {
		signals := map[string]any{
			"module_id":                 moduleID,
			"execution_status":          ArvisExecutionCompleted,
			"verified_evidence":         true,
			"real_onchain_evidence":     true,
			"real_offchain_evidence":    false,
			"arm_evidence_available":    true,
			"evidence_status":           "verified_holder_owner_program",
			"collector_attempted":       true,
			"finding_observed":          true,
			"reason_code":               discovery.Reason,
			"searched":                  discovery.Searched,
			"scope":                     discovery.Scope,
			"limitation":                claimDiscoveryLimitation,
			"distributor_program_id":    discovery.ProgramID,
			"distributor_program":       discovery.Program,
			"distributor_token_account": discovery.Account,
			"distributor_balance":       discovery.Balance,
			"observed_slot_status":      "not_captured_by_holder_role_collector",
			"offchain_origin_status":    "no_declared_origin",
			"numeric_score_disabled":    true,
			"grade_effect":              "none_at_arm_layer",
		}
		evidence := []string{
			fmt.Sprintf("Repository-pinned distributor program %s (%s) is the resolved owner program for top token account %s.", discovery.Program, discovery.ProgramID, discovery.Account),
			fmt.Sprintf("Observed account balance: %g tokens. Holder-role collector did not attach an exact observation slot.", discovery.Balance),
			"This program relation is an observation, not proof of a malicious claim surface or a complete distributor inventory.",
		}
		arm := verifiedEvidenceArm(module, moduleID, req, 0, signals, evidence, generatedAt)
		arm.Verdict = "A known distributor-program relation was observed; this arm does not issue a grade."
		return finalizeSecurityRadarVerdictAuthentication(arm)
	}
	reason := "Claim distributor discovery was not attempted because parsed holder-owner program evidence is unavailable."
	if discovery.Attempted {
		reason = "No pinned distributor program was identified among the observed top token-holder owner programs. Unobserved programs and creator outbound transfers remain unchecked."
	}
	arm := evidencePendingArm(module, moduleID, req, generatedAt, reason, discovery.Reason)
	arm.Signals["collector_attempted"] = discovery.Attempted
	arm.Signals["searched"] = discovery.Searched
	arm.Signals["scope"] = discovery.Scope
	arm.Signals["limitation"] = claimDiscoveryLimitation
	arm.Signals["offchain_origin_status"] = "no_declared_origin"
	arm.Signals["observed_slot_status"] = "not_captured_by_holder_role_collector"
	arm.Signals["numeric_score_disabled"] = true
	return arm
}
