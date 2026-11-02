package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"koschei/api/internal/networktarget"
)

// An EVM wallet on the watchlist is re-read every minute for EIP-7702
// delegation. In the 29 October 2026 Binance incident 204 seconds separated
// the delegation block from the first large sweep; an hourly token refresh
// cannot report inside that window. The alert reports observed code state
// only: it never identifies who authorized a delegation or calls it safe.
type watchlistEVMDelegationSnapshot struct {
	Network           string `json:"network"`
	ChainID           string `json:"chain_id"`
	ContractCodeState string `json:"contract_code_state"`
	ContractCodeHash  string `json:"contract_code_sha256,omitempty"`
	DelegationState   string `json:"delegation_state"`
	DelegationTarget  string `json:"delegation_target,omitempty"`
	CheckedAt         string `json:"checked_at"`
}

// watchlistEVMWalletNetwork reports whether network is a registered EVM
// network with a configured RPC, the only case a wallet can be watched.
func watchlistEVMWalletNetwork(network string) (bool, bool) {
	registered, ok := networktarget.LookupNetwork(network)
	if !ok || registered.Family != "evm" {
		return false, false
	}
	return true, canonicalNativeRPCEnv(network) != ""
}

// watchlistEVMHTTPClient is nil in production (the probe's default client);
// tests point it at a local TLS fixture server.
var watchlistEVMHTTPClient *http.Client

func watchlistEVMCheckInterval() time.Duration {
	if parsed, err := time.ParseDuration(strings.TrimSpace(os.Getenv("WATCHLIST_EVM_DELEGATION_INTERVAL"))); err == nil && parsed >= 15*time.Second {
		return parsed
	}
	return time.Minute
}

func (h *Handler) refreshWatchlistEVMWallet(ctx context.Context, authSubject string, target watchlistTarget, oldSnapshotRaw []byte) (watchlistTarget, int, error) {
	interval := watchlistEVMCheckInterval()
	endpoint := canonicalNativeRPCEnv(target.Network)
	resolution, err := networktarget.Resolve(target.Network, target.Target)
	if err == nil && endpoint == "" {
		err = fmt.Errorf("evm rpc not configured for %s", target.Network)
	}
	var probe networktarget.EVMProbeResult
	if err == nil {
		probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		probe, err = networktarget.ProbeEVM(probeCtx, watchlistEVMHTTPClient, endpoint, resolution)
		cancel()
	}
	if err != nil {
		_, _ = h.DB.ExecContext(ctx, `UPDATE watchlist_targets SET last_checked_at=now(),next_check_at=now()+$3::interval,updated_at=now() WHERE id=$1 AND auth_subject=$2`,
			target.ID, authSubject, interval.String())
		return target, 0, err
	}
	current := watchlistEVMDelegationSnapshot{
		Network: target.Network, ChainID: probe.ChainID, ContractCodeState: probe.ContractCodeState,
		ContractCodeHash: probe.ContractCodeHash, DelegationState: probe.DelegationState,
		DelegationTarget: strings.ToLower(probe.DelegationTarget), CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	var previous *watchlistEVMDelegationSnapshot
	var stored watchlistTokenSnapshot
	if json.Unmarshal(oldSnapshotRaw, &stored) == nil && stored.EVMDelegation != nil {
		previous = stored.EVMDelegation
	}
	alerts := compareWatchlistEVMDelegation(previous, current)
	snapshot := watchlistTokenSnapshot{SourceProvider: "evm_rpc_eth_getCode", CheckedAt: current.CheckedAt, EVMDelegation: &current}
	snapshotRaw, _ := json.Marshal(snapshot)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return target, 0, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `
		UPDATE watchlist_targets SET last_score=NULL,last_risk_level=$1,last_snapshot=$2::jsonb,last_checked_at=now(),
		       next_check_at=now()+$5::interval,updated_at=now()
		WHERE id=$3 AND auth_subject=$4`, current.DelegationState, string(snapshotRaw), target.ID, authSubject, interval.String()); err != nil {
		return target, 0, err
	}
	if err := insertWatchlistAlerts(ctx, tx, target.ID, authSubject, alerts); err != nil {
		return target, 0, err
	}
	if err := tx.Commit(); err != nil {
		return target, 0, err
	}
	now := time.Now().UTC()
	next := now.Add(interval)
	target.LastScore, target.LastRiskLevel, target.LastSnapshot = nil, current.DelegationState, &snapshot
	target.LastCheckedAt, target.NextCheckAt, target.UpdatedAt = &now, &next, now
	return target, len(alerts), nil
}

// compareWatchlistEVMDelegation turns a change in observed code state into
// alerts. The first check only reports a delegation that is already present.
func compareWatchlistEVMDelegation(previous *watchlistEVMDelegationSnapshot, current watchlistEVMDelegationSnapshot) []watchlistAlertCandidate {
	delegated := current.DelegationState == networktarget.EVMDelegationStateObserved
	evidence := map[string]any{
		"network": current.Network, "chain_id": current.ChainID, "delegation_state": current.DelegationState,
		"delegation_target": current.DelegationTarget, "contract_code_state": current.ContractCodeState,
		"source": "eth_getCode at latest block", "identity_or_intent_claim": false,
	}
	if previous == nil {
		if !delegated {
			return []watchlistAlertCandidate{}
		}
		return []watchlistAlertCandidate{{
			EventType: "evm_delegation_present", Severity: "high", Title: "EIP-7702 delegation is active",
			Message:       "This wallet's code currently delegates execution to " + current.DelegationTarget + ". Confirm you authorized it.",
			PreviousValue: nil, CurrentValue: current.DelegationTarget, Evidence: evidence,
		}}
	}
	wasDelegated := previous.DelegationState == networktarget.EVMDelegationStateObserved
	alerts := []watchlistAlertCandidate{}
	switch {
	case delegated && !wasDelegated:
		alerts = append(alerts, watchlistAlertCandidate{
			EventType: "evm_delegation_added", Severity: "critical", Title: "New EIP-7702 delegation on a watched wallet",
			Message:       "The wallet started delegating execution to " + current.DelegationTarget + ". If you did not authorize this, treat the wallet as compromised and move funds you still control.",
			PreviousValue: previous.DelegationState, CurrentValue: current.DelegationTarget, Evidence: evidence,
		})
	case delegated && wasDelegated && previous.DelegationTarget != current.DelegationTarget:
		alerts = append(alerts, watchlistAlertCandidate{
			EventType: "evm_delegation_changed", Severity: "critical", Title: "EIP-7702 delegate changed",
			Message:       "The wallet now delegates to " + current.DelegationTarget + " instead of " + previous.DelegationTarget + ".",
			PreviousValue: previous.DelegationTarget, CurrentValue: current.DelegationTarget, Evidence: evidence,
		})
	case !delegated && wasDelegated:
		alerts = append(alerts, watchlistAlertCandidate{
			EventType: "evm_delegation_removed", Severity: "info", Title: "EIP-7702 delegation removed",
			Message:       "The wallet no longer delegates execution. Empty code now does not prove that nothing happened while the delegation was active.",
			PreviousValue: previous.DelegationTarget, CurrentValue: current.DelegationState, Evidence: evidence,
		})
	}
	if !delegated && previous.ContractCodeHash != "" && current.ContractCodeHash != "" && previous.ContractCodeHash != current.ContractCodeHash {
		alerts = append(alerts, watchlistAlertCandidate{
			EventType: "evm_code_changed", Severity: "high", Title: "Contract code changed",
			Message:       "The code stored at this address differs from the previous check.",
			PreviousValue: previous.ContractCodeHash, CurrentValue: current.ContractCodeHash, Evidence: evidence,
		})
	}
	return deduplicateWatchlistAlerts(alerts)
}
