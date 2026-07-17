package services

// ArvisScanBudgets contains independent per-stage limits for a manual ARVIS
// investigation. Keeping these limits separate prevents one slow collector from
// consuming the time reserved for subsequent stages.
type ArvisScanBudgets struct {
	WalletTimeoutSeconds     int
	LaunchTimeoutSeconds     int
	CreatorTimeoutSeconds    int
	ActorQueueTimeoutSeconds int
	RPCBudget                int
	FundingRPCBudget         int
}

func LoadArvisScanBudgets() ArvisScanBudgets {
	return ArvisScanBudgets{
		WalletTimeoutSeconds:     holderScanEnvInt("ARVIS_WALLET_SCAN_TIMEOUT_SECONDS", 28, 10, 240),
		LaunchTimeoutSeconds:     holderScanEnvInt("ARVIS_LAUNCH_SCAN_TIMEOUT_SECONDS", 24, 10, 240),
		CreatorTimeoutSeconds:    holderScanEnvInt("ARVIS_CREATOR_SCAN_TIMEOUT_SECONDS", 20, 10, 180),
		ActorQueueTimeoutSeconds: holderScanEnvInt("ARVIS_ACTOR_QUEUE_TIMEOUT_SECONDS", 20, 5, 120),
		RPCBudget:                holderScanEnvInt("ARVIS_SCAN_RPC_BUDGET", 600, 25, 5000),
		FundingRPCBudget:         holderScanEnvInt("ARVIS_FUNDING_RPC_BUDGET", 100, 25, 2000),
	}
}
