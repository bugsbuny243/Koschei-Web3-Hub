package networktarget

const GlobalRadarCapabilitySchemaVersion = "koschei.global-radar-capability.v1"

type RadarCapability struct {
	NetworkID                 string `json:"network_id"`
	Family                    string `json:"family"`
	IdentityResolution        string `json:"identity_resolution"`
	TransactionObservation    string `json:"transaction_observation"`
	StateObservation          string `json:"state_observation"`
	ContractOrProgramAnalysis string `json:"contract_or_program_analysis"`
	NodeTelemetry             string `json:"node_telemetry"`
	CrossChainCorrelation     string `json:"cross_chain_correlation"`
	VerdictAuthority          string `json:"verdict_authority"`
	EvidenceBoundary          string `json:"evidence_boundary"`
}

// GlobalRadarCapabilities is a repository-capability contract, not a deployment
// health report. Values describe implemented code paths only. A capability must
// remain fail-closed until network-specific evidence is collected by its native
// adapter; shared address syntax is never enough to promote another network.
//
// The contract is intentionally explicit so "multi-chain" cannot collapse into
// one misleading coverage percentage.
func GlobalRadarCapabilities() []RadarCapability {
	out := make([]RadarCapability, 0, len(Catalog()))
	for _, network := range Catalog() {
		capability := RadarCapability{
			NetworkID:                 network.ID,
			Family:                    network.Family,
			IdentityResolution:        "implemented",
			TransactionObservation:    "not_connected",
			StateObservation:          "not_connected",
			ContractOrProgramAnalysis: "not_connected",
			NodeTelemetry:             network.NodeTelemetryStatus,
			CrossChainCorrelation:     "not_connected",
			VerdictAuthority:          "not_connected",
			EvidenceBoundary:          "repository_capability_only_live_availability_not_checked",
		}

		switch network.Family {
		case "solana":
			capability.TransactionObservation = "existing"
			capability.StateObservation = "existing"
			capability.ContractOrProgramAnalysis = "existing"
			capability.CrossChainCorrelation = "evidence_contract_ready"
			capability.VerdictAuthority = "arvis_existing"
		case "evm":
			capability.TransactionObservation = "probe_ready"
			capability.StateObservation = "probe_ready"
			capability.ContractOrProgramAnalysis = "probe_ready"
			capability.CrossChainCorrelation = "evidence_contract_ready"
			capability.VerdictAuthority = "not_connected"
		case "utxo":
			capability.TransactionObservation = "probe_ready"
			capability.StateObservation = "probe_ready"
			capability.ContractOrProgramAnalysis = "not_applicable"
			capability.CrossChainCorrelation = "evidence_contract_ready"
			capability.VerdictAuthority = "not_connected"
		case "move":
			// Sui and Aptos currently classify identity and validate the selected
			// network, but the universal dispatcher remains fail-closed. Do not
			// promote those probes into transaction/state/verdict readiness.
			capability.TransactionObservation = "not_connected"
			capability.StateObservation = "not_connected"
			capability.ContractOrProgramAnalysis = "not_connected"
			capability.CrossChainCorrelation = "not_connected"
			capability.VerdictAuthority = "not_connected"
		}

		out = append(out, capability)
	}
	return out
}
