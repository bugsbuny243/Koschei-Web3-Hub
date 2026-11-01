package handlers

import (
	"strconv"
	"strings"
)

const transactionGuardSOCPlaybook02Version = "koschei.soc-pb02-pre-sign.v1"

type transactionGuardSOCPlaybook02Event struct {
	InstructionSource string `json:"instruction_source"`
	InstructionIndex  int    `json:"instruction_index"`
	InnerSequence     int    `json:"inner_sequence"`
	ProgramID         string `json:"program_id"`
	Kind              string `json:"kind"`
	Account           string `json:"account"`
	AuthorityType     string `json:"authority_type"`
	NewAuthority      string `json:"new_authority"`
	PostStateVerified bool   `json:"post_state_verified"`
}

type transactionGuardSOCPlaybook02 struct {
	SchemaVersion     string                               `json:"schema_version"`
	PlaybookID        string                               `json:"playbook_id"`
	ExecutionState    string                               `json:"execution_state"`
	EvidenceState     string                               `json:"evidence_state"`
	FindingState      string                               `json:"finding_state"`
	Severity          string                               `json:"severity"`
	RecommendedAction string                               `json:"recommended_action"`
	EnforcementState  string                               `json:"enforcement_state"`
	ReportIntegrity   string                               `json:"report_integrity"`
	Events            []transactionGuardSOCPlaybook02Event `json:"events"`
	ReasonCodes       []string                             `json:"reason_codes"`
}

// buildTransactionGuardSOCPlaybook02 uses the actual decoded pre-sign bytes
// and the successful simulation's CPI and post-state authority analysis.
// Evidence is prospective, not a claim that a chain transaction executed.
// An outer or nested SetAuthority change may be entirely legitimate.
func buildTransactionGuardSOCPlaybook02(
	assessment transactionFirewallAssessment,
	decoded transactionGuardDecodedTransaction,
	cpi transactionGuardCPIFlowAnalysis,
	authority transactionGuardAuthoritySurfaceAnalysis,
) transactionGuardSOCPlaybook02 {
	out := transactionGuardSOCPlaybook02{
		SchemaVersion:     transactionGuardSOCPlaybook02Version,
		PlaybookID:        "SOC-PB-02",
		ExecutionState:    "pre_sign_simulation_only",
		EvidenceState:     "incomplete",
		FindingState:      "withheld",
		Severity:          "unknown",
		RecommendedAction: "withhold_signature_until_evidence_complete",
		EnforcementState:  "signer_not_connected",
		ReportIntegrity:   "unsigned_preflight_observation",
		Events:            []transactionGuardSOCPlaybook02Event{},
		ReasonCodes:       []string{},
	}
	complete := assessment.SimulationOK && decoded.Available && decoded.Complete &&
		authority.Available && authority.Complete &&
		(!cpi.Required || (cpi.Available && cpi.Complete))
	if !complete {
		out.ExecutionState = "incomplete"
		out.ReasonCodes = append(out.ReasonCodes, "PB02_REQUIRED_DECODE_CPI_OR_SIMULATION_UNAVAILABLE")
	}
	for _, event := range authority.Events {
		if event.Kind != "set_authority" && event.Kind != "initialize_permanent_delegate" {
			continue
		}
		if len(out.Events) >= 64 {
			complete = false
			out.ReasonCodes = append(out.ReasonCodes, "PB02_AUTHORITY_EVENT_LIMIT_EXCEEDED")
			break
		}
		if (event.ProgramID != guardV3SPLTokenProgramID &&
			event.ProgramID != guardV3Token2022ProgramID) ||
			(event.InstructionSource != "outer" && event.InstructionSource != "cpi") ||
			(event.InstructionSource == "cpi" && !cpi.Available) ||
			!looksLikeGuardPubkey(event.Account) {
			complete = false
			out.ReasonCodes = append(out.ReasonCodes, "PB02_AUTHORITY_EVENT_ORIGIN_INVALID")
			continue
		}
		typeName := event.AuthorityTypeName
		if event.Kind == "set_authority" {
			if event.AuthorityType == nil || *event.AuthorityType < 0 || *event.AuthorityType > 17 ||
				event.NewAuthority == "" ||
				(event.NewAuthority != "revoked" && !looksLikeGuardPubkey(event.NewAuthority)) {
				complete = false
				out.ReasonCodes = append(out.ReasonCodes, "PB02_AUTHORITY_EVENT_FIELDS_INVALID")
				continue
			}
			expectedName, _, _, _, _ := guardV3AuthorityTypeSemantics(*event.AuthorityType)
			if expectedName != typeName {
				complete = false
				out.ReasonCodes = append(out.ReasonCodes, "PB02_AUTHORITY_TYPE_MISMATCH")
				continue
			}
		} else {
			typeName = "permanent_delegate"
			if event.ProgramID != guardV3Token2022ProgramID || !looksLikeGuardPubkey(event.Delegate) {
				complete = false
				out.ReasonCodes = append(out.ReasonCodes, "PB02_DELEGATE_IDENTITY_INVALID")
				continue
			}
		}
		active := event.ActiveAfterSimulation != nil && *event.ActiveAfterSimulation
		out.Events = append(out.Events, transactionGuardSOCPlaybook02Event{
			InstructionSource: event.InstructionSource, InstructionIndex: event.InstructionIndex,
			InnerSequence: event.InnerSequence, ProgramID: event.ProgramID, Kind: event.Kind,
			Account: event.Account, AuthorityType: typeName, NewAuthority: event.NewAuthority,
			PostStateVerified: event.PostStateAvailable && active,
		})
	}
	if !complete {
		out.EvidenceState = "incomplete_or_conflicting"
		return out
	}
	out.EvidenceState = "decoded_and_simulated"
	out.FindingState = "no_pb02_authority_mutation_observed"
	out.Severity = "none"
	out.RecommendedAction = "continue_standard_guard_checks"
	for _, event := range out.Events {
		if event.NewAuthority == "revoked" {
			out.ReasonCodes = append(out.ReasonCodes, "PB02_AUTHORITY_REVOCATION_PROPOSED")
			continue
		}
		out.FindingState = "token_authority_mutation_proposed"
		out.Severity = "high"
		out.RecommendedAction = "deny_pre_sign_permit_and_request_owner_review"
		out.ReasonCodes = append(out.ReasonCodes, "PB02_NEW_TOKEN_AUTHORITY_PROPOSED")
		if event.InstructionSource == "cpi" {
			out.ReasonCodes = append(out.ReasonCodes, "PB02_NESTED_CPI_AUTHORITY_CHANGE")
		}
		if strings.Contains(event.AuthorityType, "permanent_delegate") ||
			event.AuthorityType == "mint_tokens" || event.AuthorityType == "freeze_account" ||
			event.AuthorityType == "pause" || event.AuthorityType == "permissioned_burn" {
			out.Severity = "critical"
			out.ReasonCodes = append(out.ReasonCodes, "PB02_MINT_WIDE_AUTHORITY_CAPABILITY")
		}
		if !event.PostStateVerified {
			out.ReasonCodes = append(out.ReasonCodes, "PB02_POST_STATE_NOT_INDEPENDENTLY_CONFIRMED")
		}
	}
	if len(out.Events) > 0 {
		out.ReasonCodes = append(out.ReasonCodes, "PB02_MATCHED_INSTRUCTIONS_"+strconv.Itoa(len(out.Events)))
	}
	return out
}
