package handlers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"koschei/api/internal/networktarget"
)

const (
	ciFixtureDelegate      = "0x63e842b054bd624a465454d533c33f573e9c6fae"
	ciFixtureOtherDelegate = "0x1111111111111111111111111111111111111111"
)

func evmSnapshot(state, delegate, codeHash string) watchlistEVMDelegationSnapshot {
	return watchlistEVMDelegationSnapshot{Network: "ethereum-mainnet", ChainID: "0x1", DelegationState: state, DelegationTarget: delegate, ContractCodeHash: codeHash}
}

func eventTypes(alerts []watchlistAlertCandidate) []string {
	out := []string{}
	for _, a := range alerts {
		out = append(out, a.EventType+":"+a.Severity)
	}
	return out
}

// Mirrors the archived Binance sequence: empty code, delegation, revocation.
func TestWatchlistEVMDelegationAlertsFollowTheBinanceSequence(t *testing.T) {
	none := evmSnapshot(networktarget.EVMDelegationStateNotObserved, "", "")
	delegated := evmSnapshot(networktarget.EVMDelegationStateObserved, ciFixtureDelegate, "")

	if got := compareWatchlistEVMDelegation(nil, none); len(got) != 0 {
		t.Fatalf("a first check without delegation must stay quiet: %v", eventTypes(got))
	}
	if got := eventTypes(compareWatchlistEVMDelegation(nil, delegated)); len(got) != 1 || got[0] != "evm_delegation_present:high" {
		t.Fatalf("a delegation present at the first check = %v", got)
	}
	added := compareWatchlistEVMDelegation(&none, delegated)
	if got := eventTypes(added); len(got) != 1 || got[0] != "evm_delegation_added:critical" {
		t.Fatalf("a new delegation must raise a critical alert, got %v", got)
	}
	if added[0].CurrentValue != ciFixtureDelegate || added[0].Evidence["identity_or_intent_claim"] != false {
		t.Fatalf("the alert must cite the delegate and claim no identity or intent: %+v", added[0])
	}
	if got := compareWatchlistEVMDelegation(&delegated, delegated); len(got) != 0 {
		t.Fatalf("an unchanged delegation must not repeat alerts: %v", eventTypes(got))
	}
	changed := evmSnapshot(networktarget.EVMDelegationStateObserved, ciFixtureOtherDelegate, "")
	if got := eventTypes(compareWatchlistEVMDelegation(&delegated, changed)); len(got) != 1 || got[0] != "evm_delegation_changed:critical" {
		t.Fatalf("a changed delegate = %v", got)
	}
	removed := compareWatchlistEVMDelegation(&delegated, none)
	if got := eventTypes(removed); len(got) != 1 || got[0] != "evm_delegation_removed:info" {
		t.Fatalf("a revocation = %v", got)
	}
}

func TestWatchlistEVMCodeChangeIsReportedForContracts(t *testing.T) {
	before := evmSnapshot(networktarget.EVMDelegationStateNotObserved, "", "aaa")
	after := evmSnapshot(networktarget.EVMDelegationStateNotObserved, "", "bbb")
	if got := eventTypes(compareWatchlistEVMDelegation(&before, after)); len(got) != 1 || got[0] != "evm_code_changed:high" {
		t.Fatalf("a contract code change = %v", got)
	}
}

func TestWatchlistWalletsRequireARegisteredEVMNetwork(t *testing.T) {
	t.Setenv("ETHEREUM_RPC_URL", "")
	if evm, _ := watchlistEVMWalletNetwork("solana-mainnet"); evm {
		t.Fatal("Solana wallets are not watched by the EVM delegation monitor")
	}
	if evm, configured := watchlistEVMWalletNetwork("ethereum-mainnet"); !evm || configured {
		t.Fatalf("ethereum without an RPC must be recognised but unconfigured: evm=%v configured=%v", evm, configured)
	}
	t.Setenv("ETHEREUM_RPC_URL", "https://ci-fixture.invalid")
	if _, configured := watchlistEVMWalletNetwork("ethereum-mainnet"); !configured {
		t.Fatal("a configured RPC must enable wallet watch")
	}
}

func TestWatchlistEVMProbeErrorsNeverCarryTheRPCURL(t *testing.T) {
	raw := fmt.Errorf(`evm_chain_id_unavailable: Post "https://rpc.example/v2/ci-fixture-secret-key": dial tcp: connection refused`)
	if got := watchlistEVMProbeError(raw).Error(); got != "evm_chain_id_unavailable" || strings.Contains(got, "secret") {
		t.Fatalf("sanitized error = %q", got)
	}
	if got := watchlistEVMProbeError(fmt.Errorf(`Post "https://rpc.example/?key=ci-fixture-secret": timeout`)).Error(); got != "evm_probe_failed" {
		t.Fatalf("an error without a category must not leak its text: %q", got)
	}
}

func TestWatchlistEVMNextCheckFitsTheMonitorTick(t *testing.T) {
	if got := watchlistEVMNextCheck(time.Minute); got != 45*time.Second {
		t.Fatalf("one-minute cadence schedules %s, want 45s so the next tick picks it up", got)
	}
	if got := watchlistEVMNextCheck(20 * time.Second); got != 15*time.Second {
		t.Fatalf("short cadence = %s", got)
	}
}
