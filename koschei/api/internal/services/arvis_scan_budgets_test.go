package services

import "testing"

func TestLoadArvisScanBudgetsDefaults(t *testing.T) {
	for _, name := range []string{"ARVIS_WALLET_SCAN_TIMEOUT_SECONDS", "ARVIS_LAUNCH_SCAN_TIMEOUT_SECONDS", "ARVIS_CREATOR_SCAN_TIMEOUT_SECONDS", "ARVIS_ACTOR_QUEUE_TIMEOUT_SECONDS", "ARVIS_SCAN_RPC_BUDGET", "ARVIS_FUNDING_RPC_BUDGET"} {
		t.Setenv(name, "")
	}
	got := LoadArvisScanBudgets()
	if got.WalletTimeoutSeconds != 28 || got.LaunchTimeoutSeconds != 24 || got.CreatorTimeoutSeconds != 20 || got.ActorQueueTimeoutSeconds != 20 || got.RPCBudget != 600 || got.FundingRPCBudget != 100 {
		t.Fatalf("defaults = %#v", got)
	}
}

func TestLoadArvisScanBudgetsAcceptsOverridesWithinClamp(t *testing.T) {
	t.Setenv("ARVIS_WALLET_SCAN_TIMEOUT_SECONDS", "240")
	t.Setenv("ARVIS_LAUNCH_SCAN_TIMEOUT_SECONDS", "120")
	t.Setenv("ARVIS_CREATOR_SCAN_TIMEOUT_SECONDS", "180")
	t.Setenv("ARVIS_ACTOR_QUEUE_TIMEOUT_SECONDS", "5")
	t.Setenv("ARVIS_SCAN_RPC_BUDGET", "3000")
	t.Setenv("ARVIS_FUNDING_RPC_BUDGET", "2000")
	got := LoadArvisScanBudgets()
	if got.WalletTimeoutSeconds != 240 || got.LaunchTimeoutSeconds != 120 || got.CreatorTimeoutSeconds != 180 || got.ActorQueueTimeoutSeconds != 5 || got.RPCBudget != 3000 || got.FundingRPCBudget != 2000 {
		t.Fatalf("overrides = %#v", got)
	}
}

func TestLoadArvisScanBudgetsRejectsOutOfRangeValues(t *testing.T) {
	t.Setenv("ARVIS_WALLET_SCAN_TIMEOUT_SECONDS", "241")
	t.Setenv("ARVIS_LAUNCH_SCAN_TIMEOUT_SECONDS", "9")
	t.Setenv("ARVIS_CREATOR_SCAN_TIMEOUT_SECONDS", "181")
	t.Setenv("ARVIS_ACTOR_QUEUE_TIMEOUT_SECONDS", "4")
	t.Setenv("ARVIS_SCAN_RPC_BUDGET", "5001")
	t.Setenv("ARVIS_FUNDING_RPC_BUDGET", "24")
	got := LoadArvisScanBudgets()
	if got.WalletTimeoutSeconds != 28 || got.LaunchTimeoutSeconds != 24 || got.CreatorTimeoutSeconds != 20 || got.ActorQueueTimeoutSeconds != 20 || got.RPCBudget != 600 || got.FundingRPCBudget != 100 {
		t.Fatalf("fallbacks = %#v", got)
	}
}
