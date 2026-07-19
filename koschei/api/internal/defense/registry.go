package defense

func InitialToolRegistry() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:             "resolve_program_surface",
			Purpose:          "Resolve program identifiers already present in the signed investigation file and classify missing source, IDL and bytecode artifacts.",
			ReadOnly:         true,
			NetworkAccess:    "none",
			AllowedRoles:     []string{RoleProgramArchaeologist},
			ProducesEvidence: true,
			CanChangeVerdict: false,
			RequiresSandbox:  false,
		},
		{
			Name:             "extract_instruction_graph",
			Purpose:          "Build instruction, account, PDA and CPI relationships from a verified source, IDL or bytecode artifact.",
			ReadOnly:         true,
			NetworkAccess:    "allowlisted_artifact_sources",
			AllowedRoles:     []string{RoleProgramArchaeologist, RoleStaticAnalyzer},
			ProducesEvidence: true,
			CanChangeVerdict: false,
			RequiresSandbox:  true,
		},
		{
			Name:             "run_static_detectors",
			Purpose:          "Run Solana and Anchor-specific account validation, PDA, CPI, authority and arithmetic detectors.",
			ReadOnly:         true,
			NetworkAccess:    "none",
			AllowedRoles:     []string{RoleStaticAnalyzer},
			ProducesEvidence: true,
			CanChangeVerdict: false,
			RequiresSandbox:  true,
		},
		{
			Name:              "prepare_reproduction_plan",
			Purpose:           "Prepare a non-executing local-SVM reproduction plan for a verified, reachable finding.",
			ReadOnly:          true,
			NetworkAccess:     "none",
			AllowedRoles:      []string{RoleReproductionAgent},
			ProducesEvidence:  false,
			CanChangeVerdict:  false,
			RequiresSandbox:   true,
			RequiresHumanGate: true,
		},
	}
}
