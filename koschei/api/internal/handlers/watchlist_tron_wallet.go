package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"koschei/api/internal/radaradapter/tron"
	"koschei/api/internal/services"
)

// A Tron wallet on the watchlist is re-read every minute for new outbound
// transfers and for the USDT issuer freeze state. In the October/November 2026
// Binance incident about $242M of Tron funds left through bridges within two
// days while Tether's freeze covered the rest; an hourly refresh cannot report
// inside that window. Collection and decoding live in the Tron adapter
// (radaradapter/tron); this consumer only compares normalized observations.
// Alerts never identify who controls the wallet, never claim a transfer was
// malicious and never claim Koschei can stop or reverse an on-chain transfer.

const (
	watchlistTronNetwork        = "tron-mainnet"
	watchlistTronPageLimit      = 50
	watchlistTronMaxAlerts      = 10
	watchlistTronBurstThreshold = 5
	// TronGrid can index a confirmed transfer after newer ones; an unseen
	// transfer up to this much older than the newest seen one still alerts.
	watchlistTronIndexLagMS = int64(10 * 60 * 1000)
)

// watchlistTronBaseURL and watchlistTronHTTPClient are overridden only by tests.
var (
	watchlistTronBaseURL    = "https://api.trongrid.io"
	watchlistTronHTTPClient *http.Client
)

func watchlistTronGrid() tron.GridClient {
	return tron.GridClient{BaseURL: watchlistTronBaseURL, APIKey: os.Getenv("TRONGRID_API_KEY"), HTTP: watchlistTronHTTPClient}
}

// watchlistTronTransfer is a normalized adapter transfer plus display fields
// and the operator bridge tag.
type watchlistTronTransfer struct {
	tron.Transfer
	ToBase58     string `json:"to_base58,omitempty"`
	Amount       string `json:"amount"`
	BridgeLabel  string `json:"bridge_label,omitempty"`
	BridgeSource string `json:"bridge_source,omitempty"`
}

type watchlistTronWalletSnapshot struct {
	Network              string                 `json:"network"`
	Address              string                 `json:"address_hex"`
	SeenOutboundEventIDs []string               `json:"seen_outbound_event_ids"`
	LatestOutboundMS     int64                  `json:"latest_outbound_block_timestamp_ms"`
	LastOutbound         *watchlistTronTransfer `json:"last_outbound,omitempty"`
	// USDTFreezeState is this check's observation ("unknown" when unreadable);
	// USDTFreezeLastKnown is the latest readable state, used for transitions.
	USDTFreezeState     string `json:"usdt_freeze_state"`
	USDTFreezeLastKnown string `json:"usdt_freeze_last_known,omitempty"`
	CheckedAt           string `json:"checked_at"`
}

func watchlistTronWalletNetwork(network string) bool {
	return strings.TrimSpace(network) == watchlistTronNetwork
}

func watchlistTronConfigured() bool {
	return strings.TrimSpace(os.Getenv("TRONGRID_API_KEY")) != ""
}

func watchlistTronCheckInterval() time.Duration {
	if parsed, err := time.ParseDuration(strings.TrimSpace(os.Getenv("WATCHLIST_TRON_INTERVAL"))); err == nil && parsed >= 15*time.Second {
		return parsed
	}
	return time.Minute
}

// decorateWatchlistTronTransfers adds display fields and the bridge tag from
// the operator-pinned registry shared with the Tron Flow Radar.
func decorateWatchlistTronTransfers(transfers []tron.Transfer, bridges map[string]services.TronBridge) []watchlistTronTransfer {
	out := make([]watchlistTronTransfer, 0, len(transfers))
	for _, transfer := range transfers {
		item := watchlistTronTransfer{Transfer: transfer, ToBase58: tron.Base58(transfer.To), Amount: formatWatchlistTronAmount(transfer.ValueRaw, transfer.Decimals)}
		if bridge, ok := bridges[transfer.To]; ok {
			item.BridgeLabel, item.BridgeSource = bridge.Label, bridge.Source
		}
		out = append(out, item)
	}
	return out
}

// formatWatchlistTronAmount renders an integer amount with its decimals,
// without floating-point rounding.
func formatWatchlistTronAmount(valueRaw string, decimals *int) string {
	value, ok := new(big.Int).SetString(valueRaw, 10)
	if !ok || decimals == nil {
		return valueRaw + " (raw units)"
	}
	return tron.FormatUnits(value, *decimals)
}

func (t watchlistTronTransfer) displayTo() string {
	if t.ToBase58 != "" {
		return t.ToBase58
	}
	return t.To
}

func (t watchlistTronTransfer) assetLabel() string {
	if t.TokenSymbol != "" {
		return t.TokenSymbol
	}
	return "token " + t.TokenContract
}

// newWatchlistTronOutbound returns transfer events not seen at the previous
// check, newest first. The first check is a baseline and reports no history.
func newWatchlistTronOutbound(previous *watchlistTronWalletSnapshot, fetched []watchlistTronTransfer) []watchlistTronTransfer {
	if previous == nil {
		return nil
	}
	seen := map[string]struct{}{}
	for _, id := range previous.SeenOutboundEventIDs {
		seen[id] = struct{}{}
	}
	out := []watchlistTronTransfer{}
	for _, transfer := range fetched {
		if _, ok := seen[transfer.EventID]; ok || transfer.BlockTimestampMS < previous.LatestOutboundMS-watchlistTronIndexLagMS {
			continue
		}
		out = append(out, transfer)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].BlockTimestampMS > out[j].BlockTimestampMS })
	return out
}

func watchlistTronResponseOptions(transfer watchlistTronTransfer, usdtContract string) []string {
	options := []string{}
	if transfer.TokenContract != "" && transfer.TokenContract == usdtContract {
		options = append(options, "issuer_freeze_request:tether_usdt")
	}
	if transfer.BridgeLabel != "" {
		options = append(options, "bridge_operator_notice:"+transfer.BridgeLabel)
	}
	return append(options, "receiving_party_notice")
}

// compareWatchlistTronWallet turns new outbound transfer events and USDT
// freeze changes into alerts. Alerts are keyed by event, not transaction, so
// a batch transaction's transfers stay individually observable.
func compareWatchlistTronWallet(previous *watchlistTronWalletSnapshot, current watchlistTronWalletSnapshot, fresh []watchlistTronTransfer) []watchlistAlertCandidate {
	usdtContract, _ := tron.Address(tron.USDTContract)
	alerts := []watchlistAlertCandidate{}
	baseEvidence := func() map[string]any {
		return map[string]any{
			"network": current.Network, "address_hex": current.Address, "source": "TronGrid v1 confirmed indexed transfers",
			"indexed_observation": true, "identity_or_intent_claim": false, "koschei_can_reverse_transfer": false,
			"bounded_window": fmt.Sprintf("at most %d most recent outbound transfers per asset class per check", watchlistTronPageLimit),
		}
	}
	for i, transfer := range fresh {
		if i >= watchlistTronMaxAlerts {
			break
		}
		evidence := baseEvidence()
		evidence["transfer"] = transfer
		evidence["response_options"] = watchlistTronResponseOptions(transfer, usdtContract)
		amount := transfer.Amount + " " + transfer.assetLabel()
		stop := "Koschei cannot reverse an on-chain transfer. "
		if transfer.TokenContract != "" && transfer.TokenContract == usdtContract {
			stop += "Only Tether can freeze USDT; report the transaction ID and destination to Tether and law enforcement now."
		} else {
			stop += "Only the receiving exchange, bridge or token issuer can hold these funds."
		}
		if transfer.BridgeLabel != "" {
			alerts = append(alerts, watchlistAlertCandidate{
				EventType: "tron_bridge_exit", Severity: "critical", Title: "Funds sent to a pinned bridge/swap address",
				Message:       amount + " left the watched wallet to " + transfer.BridgeLabel + " (" + transfer.displayTo() + "). Funds may leave Tron. " + stop,
				PreviousValue: nil, CurrentValue: transfer.EventID, Evidence: evidence,
			})
			continue
		}
		alerts = append(alerts, watchlistAlertCandidate{
			EventType: "tron_outbound_transfer", Severity: "high", Title: "Outbound transfer from a watched Tron wallet",
			Message:       amount + " was sent to " + transfer.displayTo() + ". If you did not authorize this, treat the wallet as compromised. " + stop,
			PreviousValue: nil, CurrentValue: transfer.EventID, Evidence: evidence,
		})
	}
	if len(fresh) >= watchlistTronBurstThreshold {
		evidence := baseEvidence()
		evidence["new_outbound_count"] = len(fresh)
		evidence["alerted_individually"] = min(len(fresh), watchlistTronMaxAlerts)
		alerts = append(alerts, watchlistAlertCandidate{
			EventType: "tron_outbound_burst", Severity: "critical", Title: "Burst of outbound transfers",
			Message:       fmt.Sprintf("%d new outbound transfers since the previous check. Scripted draining often looks like this.", len(fresh)),
			PreviousValue: nil, CurrentValue: fmt.Sprintf("%s:%d", current.CheckedAt, len(fresh)), Evidence: evidence,
		})
	}

	freezeEvidence := baseEvidence()
	freezeEvidence["source"] = "TRC-20 USDT isBlackListed(address) constant call"
	freezeEvidence["usdt_contract"] = usdtContract
	freezeEvidence["usdt_freeze_state"] = current.USDTFreezeState
	previousKnown := ""
	if previous != nil {
		previousKnown = previous.USDTFreezeLastKnown
		if previousKnown == "" && previous.USDTFreezeState != tron.FreezeUnknown {
			previousKnown = previous.USDTFreezeState
		}
	}
	switch {
	case previousKnown == "" && current.USDTFreezeState == tron.FreezeFrozen:
		alerts = append(alerts, watchlistAlertCandidate{
			EventType: "tron_usdt_frozen_present", Severity: "info", Title: "USDT on this wallet is frozen by the issuer",
			Message: "Tether's USDT contract lists this address as frozen. It does not freeze TRX or other tokens.", CurrentValue: current.USDTFreezeState, Evidence: freezeEvidence,
		})
	case previousKnown == tron.FreezeClear && current.USDTFreezeState == tron.FreezeFrozen:
		alerts = append(alerts, watchlistAlertCandidate{
			EventType: "tron_usdt_frozen", Severity: "info", Title: "Issuer froze USDT on a watched wallet",
			Message: "Tether's USDT contract now lists this address as frozen. TRX and other tokens can still move.", PreviousValue: previousKnown, CurrentValue: current.USDTFreezeState, Evidence: freezeEvidence,
		})
	case previousKnown == tron.FreezeFrozen && current.USDTFreezeState == tron.FreezeClear:
		alerts = append(alerts, watchlistAlertCandidate{
			EventType: "tron_usdt_unfrozen", Severity: "high", Title: "USDT freeze lifted on a watched wallet",
			Message: "Tether's USDT contract no longer lists this address as frozen; its USDT can move again.", PreviousValue: previousKnown, CurrentValue: current.USDTFreezeState, Evidence: freezeEvidence,
		})
	}
	return deduplicateWatchlistAlerts(alerts)
}

func (h *Handler) refreshWatchlistTronWallet(ctx context.Context, authSubject string, target watchlistTarget, oldSnapshotRaw []byte) (watchlistTarget, int, error) {
	interval := watchlistEVMNextCheck(watchlistTronCheckInterval())
	base58 := strings.TrimSpace(target.Target)
	hexAddress, err := tron.Address(base58)
	if err == nil && !watchlistTronConfigured() {
		err = errors.New("tron_provider_not_configured")
	}
	var fetched []watchlistTronTransfer
	freezeState := tron.FreezeUnknown
	if err == nil {
		probeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		grid := watchlistTronGrid()
		var trc20, trx []tron.Transfer
		trc20, err = grid.OutboundTRC20(probeCtx, base58, hexAddress)
		if err == nil {
			trx, err = grid.OutboundTRX(probeCtx, base58, hexAddress)
		}
		if err == nil {
			fetched = decorateWatchlistTronTransfers(append(trc20, trx...), services.TronPinnedBridges())
			freezeState = grid.USDTFreezeState(probeCtx, hexAddress)
		}
		cancel()
	}
	if err != nil {
		// Keep the previous observation but mark it stale, so an outage can
		// never read as a fresh "no outbound transfers" result.
		_, _ = h.DB.ExecContext(ctx, `
			UPDATE watchlist_targets SET last_risk_level='tron_probe_unavailable',
			       last_snapshot=last_snapshot||jsonb_build_object('tron_probe_status','unavailable','tron_probe_failed_at',to_char(now() AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')),
			       last_checked_at=now(),next_check_at=now()+$3::interval,updated_at=now()
			WHERE id=$1 AND auth_subject=$2`, target.ID, authSubject, interval.String())
		return target, 0, err
	}

	var previous *watchlistTronWalletSnapshot
	var stored watchlistTokenSnapshot
	if json.Unmarshal(oldSnapshotRaw, &stored) == nil && stored.TronWallet != nil {
		previous = stored.TronWallet
	}
	current := buildWatchlistTronSnapshot(previous, hexAddress, fetched, freezeState)
	fresh := newWatchlistTronOutbound(previous, fetched)
	alerts := compareWatchlistTronWallet(previous, current, fresh)
	snapshot := watchlistTokenSnapshot{SourceProvider: "trongrid_v1_indexed", CheckedAt: current.CheckedAt, TronWallet: &current}
	snapshotRaw, _ := json.Marshal(snapshot)
	riskLevel := "tron_watch_quiet"
	if len(fresh) > 0 {
		riskLevel = "tron_outbound_observed"
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return target, 0, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `
		UPDATE watchlist_targets SET last_score=NULL,last_risk_level=$1,last_snapshot=$2::jsonb,last_checked_at=now(),
		       next_check_at=now()+$5::interval,updated_at=now()
		WHERE id=$3 AND auth_subject=$4`, riskLevel, string(snapshotRaw), target.ID, authSubject, interval.String()); err != nil {
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
	target.LastScore, target.LastRiskLevel, target.LastSnapshot = nil, riskLevel, &snapshot
	target.LastCheckedAt, target.NextCheckAt, target.UpdatedAt = &now, &next, now
	return target, len(alerts), nil
}

// buildWatchlistTronSnapshot keeps the newest outbound timestamp ever observed
// so a transfer that scrolls out of the bounded page is not re-reported.
func buildWatchlistTronSnapshot(previous *watchlistTronWalletSnapshot, hexAddress string, fetched []watchlistTronTransfer, freezeState string) watchlistTronWalletSnapshot {
	current := watchlistTronWalletSnapshot{
		Network: watchlistTronNetwork, Address: hexAddress, SeenOutboundEventIDs: []string{},
		USDTFreezeState: freezeState, CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if freezeState != tron.FreezeUnknown {
		current.USDTFreezeLastKnown = freezeState
	}
	if previous != nil {
		current.LatestOutboundMS = previous.LatestOutboundMS
		current.LastOutbound = previous.LastOutbound
		if current.USDTFreezeLastKnown == "" {
			// An unreadable freeze state is recorded as unknown; the last
			// readable state is carried separately so no transition is invented.
			current.USDTFreezeLastKnown = previous.USDTFreezeLastKnown
			if current.USDTFreezeLastKnown == "" && previous.USDTFreezeState != tron.FreezeUnknown {
				current.USDTFreezeLastKnown = previous.USDTFreezeState
			}
		}
	}
	for i := range fetched {
		transfer := fetched[i]
		current.SeenOutboundEventIDs = append(current.SeenOutboundEventIDs, transfer.EventID)
		if transfer.BlockTimestampMS > current.LatestOutboundMS {
			current.LatestOutboundMS = transfer.BlockTimestampMS
			current.LastOutbound = &transfer
		}
	}
	return current
}
