package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"koschei/api/internal/radaradapter/tron"
)

// A Tron wallet on the watchlist is re-read every minute for new outbound
// transfers and for the USDT issuer freeze state. In the October/November 2026
// Binance incident about $242M of Tron funds left through bridges within two
// days while Tether's freeze covered the rest; an hourly refresh cannot report
// inside that window. Alerts report indexed observations only: they never
// identify who controls the wallet, never claim a transfer was malicious and
// never claim Koschei can stop or reverse an on-chain transfer.

const (
	watchlistTronNetwork = "tron-mainnet"
	// Tether USD (TRC-20) contract, validated by Base58Check at runtime.
	watchlistTronUSDTContract   = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
	watchlistTronPageLimit      = 50
	watchlistTronMaxAlerts      = 10
	watchlistTronBurstThreshold = 5
	watchlistTronMaxResponse    = 2 << 20
	watchlistTronFreezeFrozen   = "frozen"
	watchlistTronFreezeClear    = "not_frozen"
	watchlistTronFreezeUnknown  = "unknown"
	// TronGrid can index a confirmed transfer after newer ones; an unseen
	// transfer up to this much older than the newest seen one still alerts.
	watchlistTronIndexLagMS = int64(10 * 60 * 1000)
)

// watchlistTronBaseURL and watchlistTronHTTPClient are overridden only by tests.
var (
	watchlistTronBaseURL    = "https://api.trongrid.io"
	watchlistTronHTTPClient *http.Client
)

var watchlistTronUnsignedDecimal = regexp.MustCompile(`^[0-9]{1,78}$`)

type watchlistTronTransfer struct {
	TransactionID    string `json:"transaction_id"`
	Asset            string `json:"asset"`
	TokenContract    string `json:"token_contract,omitempty"`
	TokenSymbol      string `json:"token_symbol,omitempty"`
	To               string `json:"to"`
	ToBase58         string `json:"to_base58,omitempty"`
	ValueRaw         string `json:"value_raw"`
	Decimals         *int   `json:"decimals,omitempty"`
	Amount           string `json:"amount"`
	BlockTimestampMS int64  `json:"block_timestamp_ms"`
	BridgeLabel      string `json:"bridge_label,omitempty"`
	BridgeSource     string `json:"bridge_source,omitempty"`
}

type watchlistTronWalletSnapshot struct {
	Network           string                 `json:"network"`
	Address           string                 `json:"address_hex"`
	SeenOutboundTxIDs []string               `json:"seen_outbound_transaction_ids"`
	LatestOutboundMS  int64                  `json:"latest_outbound_block_timestamp_ms"`
	LastOutbound      *watchlistTronTransfer `json:"last_outbound,omitempty"`
	// USDTFreezeState is this check's observation ("unknown" when unreadable);
	// USDTFreezeLastKnown is the latest readable state, used for transitions.
	USDTFreezeState     string `json:"usdt_freeze_state"`
	USDTFreezeLastKnown string `json:"usdt_freeze_last_known,omitempty"`
	CheckedAt           string `json:"checked_at"`
}

type watchlistTronBridge struct {
	Label  string `json:"label"`
	Source string `json:"source"`
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

// watchlistTronBridges reads the operator-pinned bridge/swap deposit registry.
// Entries need a valid Tron address, a label and an HTTPS attribution source;
// anything else is ignored, so an empty or malformed registry tags nothing.
func watchlistTronBridges() map[string]watchlistTronBridge {
	out := map[string]watchlistTronBridge{}
	raw := strings.TrimSpace(os.Getenv("WATCHLIST_TRON_BRIDGE_ADDRESSES_JSON"))
	if raw == "" {
		return out
	}
	var entries map[string]watchlistTronBridge
	if json.Unmarshal([]byte(raw), &entries) != nil {
		return out
	}
	for address, entry := range entries {
		canonical, err := tron.Address(strings.TrimSpace(address))
		label := strings.TrimSpace(entry.Label)
		source, perr := url.Parse(strings.TrimSpace(entry.Source))
		if err != nil || label == "" || len(label) > 80 || perr != nil || source.Scheme != "https" || source.Host == "" {
			continue
		}
		out[canonical] = watchlistTronBridge{Label: label, Source: source.String()}
	}
	return out
}

func watchlistTronClient() *http.Client {
	if watchlistTronHTTPClient != nil {
		return watchlistTronHTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// watchlistTronDo never returns the request URL or upstream body in errors.
func watchlistTronDo(ctx context.Context, method, path string, body any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, errors.New("tron_request_encoding_failed")
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(watchlistTronBaseURL, "/")+path, reader)
	if err != nil {
		return nil, errors.New("tron_request_invalid")
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key := strings.TrimSpace(os.Getenv("TRONGRID_API_KEY")); key != "" {
		req.Header.Set("TRON-PRO-API-KEY", key)
	}
	res, err := watchlistTronClient().Do(req)
	if err != nil {
		return nil, errors.New("tron_provider_unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tron_provider_http_%d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, watchlistTronMaxResponse+1))
	if err != nil || len(raw) > watchlistTronMaxResponse {
		return nil, errors.New("tron_provider_response_too_large")
	}
	return raw, nil
}

func watchlistTronFetchTRC20Outbound(ctx context.Context, base58, hexAddress string, bridges map[string]watchlistTronBridge) ([]watchlistTronTransfer, error) {
	raw, err := watchlistTronDo(ctx, http.MethodGet, fmt.Sprintf("/v1/accounts/%s/transactions/trc20?only_confirmed=true&only_from=true&limit=%d", url.PathEscape(base58), watchlistTronPageLimit), nil)
	if err != nil {
		return nil, err
	}
	return parseWatchlistTronTRC20(raw, hexAddress, bridges)
}

func parseWatchlistTronTRC20(raw []byte, hexAddress string, bridges map[string]watchlistTronBridge) ([]watchlistTronTransfer, error) {
	var payload struct {
		Success bool `json:"success"`
		Data    []struct {
			ID        string `json:"transaction_id"`
			From      string `json:"from"`
			To        string `json:"to"`
			Type      string `json:"type"`
			Value     string `json:"value"`
			Timestamp int64  `json:"block_timestamp"`
			Token     struct {
				Address  string `json:"address"`
				Symbol   string `json:"symbol"`
				Decimals *int   `json:"decimals"`
			} `json:"token_info"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &payload) != nil || !payload.Success {
		return nil, errors.New("tron_trc20_response_invalid")
	}
	out := []watchlistTronTransfer{}
	for i, item := range payload.Data {
		if i >= watchlistTronPageLimit {
			break
		}
		from, fe := tron.Address(item.From)
		to, te := tron.Address(item.To)
		token, ce := tron.Address(item.Token.Address)
		id, ie := hex.DecodeString(item.ID)
		if fe != nil || te != nil || ce != nil || ie != nil || len(id) != 32 || from != hexAddress || item.Type != "Transfer" ||
			item.Timestamp <= 0 || !watchlistTronUnsignedDecimal.MatchString(item.Value) {
			continue
		}
		transfer := watchlistTronTransfer{
			TransactionID: strings.ToLower(item.ID), Asset: "trc20", TokenContract: token, To: to,
			ValueRaw: item.Value, BlockTimestampMS: item.Timestamp,
		}
		if symbol := strings.TrimSpace(item.Token.Symbol); symbol != "" && len(symbol) <= 16 {
			transfer.TokenSymbol = symbol
		}
		if item.Token.Decimals != nil && *item.Token.Decimals >= 0 && *item.Token.Decimals <= 36 {
			decimals := *item.Token.Decimals
			transfer.Decimals = &decimals
		}
		tagWatchlistTronBridge(&transfer, bridges)
		out = append(out, transfer)
	}
	return out, nil
}

func watchlistTronFetchTRXOutbound(ctx context.Context, base58, hexAddress string, bridges map[string]watchlistTronBridge) ([]watchlistTronTransfer, error) {
	raw, err := watchlistTronDo(ctx, http.MethodGet, fmt.Sprintf("/v1/accounts/%s/transactions?only_confirmed=true&only_from=true&limit=%d", url.PathEscape(base58), watchlistTronPageLimit), nil)
	if err != nil {
		return nil, err
	}
	return parseWatchlistTronTRX(raw, hexAddress, bridges)
}

func parseWatchlistTronTRX(raw []byte, hexAddress string, bridges map[string]watchlistTronBridge) ([]watchlistTronTransfer, error) {
	var payload struct {
		Success bool `json:"success"`
		Data    []struct {
			ID        string `json:"txID"`
			Timestamp int64  `json:"block_timestamp"`
			Ret       []struct {
				ContractRet string `json:"contractRet"`
			} `json:"ret"`
			RawData struct {
				Contract []struct {
					Type      string `json:"type"`
					Parameter struct {
						Value struct {
							Amount       json.Number `json:"amount"`
							OwnerAddress string      `json:"owner_address"`
							ToAddress    string      `json:"to_address"`
						} `json:"value"`
					} `json:"parameter"`
				} `json:"contract"`
			} `json:"raw_data"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&payload) != nil || !payload.Success {
		return nil, errors.New("tron_trx_response_invalid")
	}
	sixDecimals := 6
	out := []watchlistTronTransfer{}
	for i, item := range payload.Data {
		if i >= watchlistTronPageLimit {
			break
		}
		if len(item.RawData.Contract) != 1 || item.RawData.Contract[0].Type != "TransferContract" || len(item.Ret) == 0 || item.Ret[0].ContractRet != "SUCCESS" {
			continue
		}
		value := item.RawData.Contract[0].Parameter.Value
		from, fe := tron.Address(value.OwnerAddress)
		to, te := tron.Address(value.ToAddress)
		id, ie := hex.DecodeString(item.ID)
		amount := value.Amount.String()
		if fe != nil || te != nil || ie != nil || len(id) != 32 || from != hexAddress || item.Timestamp <= 0 || !watchlistTronUnsignedDecimal.MatchString(amount) {
			continue
		}
		transfer := watchlistTronTransfer{
			TransactionID: strings.ToLower(item.ID), Asset: "trx", TokenSymbol: "TRX", To: to,
			ValueRaw: amount, Decimals: &sixDecimals, BlockTimestampMS: item.Timestamp,
		}
		tagWatchlistTronBridge(&transfer, bridges)
		out = append(out, transfer)
	}
	return out, nil
}

// tagWatchlistTronBridge also fills the display amount; TRC-20 decimals come
// from the indexer and are unverified, TRX uses the protocol's 6 decimals.
func tagWatchlistTronBridge(transfer *watchlistTronTransfer, bridges map[string]watchlistTronBridge) {
	transfer.Amount = formatWatchlistTronAmount(transfer.ValueRaw, transfer.Decimals)
	transfer.ToBase58 = watchlistTronBase58(transfer.To)
	if bridge, ok := bridges[transfer.To]; ok {
		transfer.BridgeLabel, transfer.BridgeSource = bridge.Label, bridge.Source
	}
}

// watchlistTronUSDTFreezeState reads Tether's TRC-20 isBlackListed(address)
// through a constant (read-only) call. Any failure yields "unknown", never
// "not_frozen".
func watchlistTronUSDTFreezeState(ctx context.Context, hexAddress string) string {
	contract, err := tron.Address(watchlistTronUSDTContract)
	if err != nil || len(hexAddress) != 42 {
		return watchlistTronFreezeUnknown
	}
	raw, err := watchlistTronDo(ctx, http.MethodPost, "/wallet/triggerconstantcontract", map[string]any{
		"owner_address":     hexAddress,
		"contract_address":  contract,
		"function_selector": "isBlackListed(address)",
		"parameter":         strings.Repeat("0", 24) + hexAddress[2:],
		"visible":           false,
	})
	if err != nil {
		return watchlistTronFreezeUnknown
	}
	return parseWatchlistTronFreezeState(raw)
}

func parseWatchlistTronFreezeState(raw []byte) string {
	var payload struct {
		Result struct {
			Result bool `json:"result"`
		} `json:"result"`
		ConstantResult []string `json:"constant_result"`
	}
	if json.Unmarshal(raw, &payload) != nil || !payload.Result.Result || len(payload.ConstantResult) != 1 {
		return watchlistTronFreezeUnknown
	}
	word := strings.ToLower(payload.ConstantResult[0])
	switch word {
	case strings.Repeat("0", 64):
		return watchlistTronFreezeClear
	case strings.Repeat("0", 63) + "1":
		return watchlistTronFreezeFrozen
	default:
		return watchlistTronFreezeUnknown
	}
}

// formatWatchlistTronAmount renders an integer amount with its decimals,
// without floating-point rounding.
func formatWatchlistTronAmount(valueRaw string, decimals *int) string {
	value, ok := new(big.Int).SetString(valueRaw, 10)
	if !ok || decimals == nil {
		return valueRaw + " (raw units)"
	}
	if *decimals == 0 {
		return value.String()
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(*decimals)), nil)
	whole, fraction := new(big.Int).QuoRem(value, scale, new(big.Int))
	frac := fmt.Sprintf("%0*s", *decimals, fraction.String())
	frac = strings.TrimRight(frac, "0")
	if frac == "" {
		return whole.String()
	}
	return whole.String() + "." + frac
}

// watchlistTronBase58 renders a validated 41-prefixed hex address in the
// Base58Check form wallets display; it returns "" if the input is not one.
func watchlistTronBase58(hexAddress string) string {
	payload, err := hex.DecodeString(hexAddress)
	if err != nil || len(payload) != 21 || payload[0] != 0x41 {
		return ""
	}
	first := sha256.Sum256(payload)
	second := sha256.Sum256(first[:])
	full := append(append([]byte{}, payload...), second[:4]...)
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	n := new(big.Int).SetBytes(full)
	mod := new(big.Int)
	out := []byte{}
	for n.Sign() > 0 {
		n.DivMod(n, big.NewInt(58), mod)
		out = append([]byte{alphabet[mod.Int64()]}, out...)
	}
	encoded := string(out)
	if roundTrip, err := tron.Address(encoded); err != nil || roundTrip != strings.ToLower(hexAddress) {
		return ""
	}
	return encoded
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

// newWatchlistTronOutbound returns transfers not seen at the previous check,
// newest first. The first check is a baseline and reports no history.
func newWatchlistTronOutbound(previous *watchlistTronWalletSnapshot, fetched []watchlistTronTransfer) []watchlistTronTransfer {
	if previous == nil {
		return nil
	}
	seen := map[string]struct{}{}
	for _, id := range previous.SeenOutboundTxIDs {
		seen[id] = struct{}{}
	}
	out := []watchlistTronTransfer{}
	for _, transfer := range fetched {
		if _, ok := seen[transfer.TransactionID]; ok || transfer.BlockTimestampMS < previous.LatestOutboundMS-watchlistTronIndexLagMS {
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

// compareWatchlistTronWallet turns new outbound transfers and USDT freeze
// changes into alerts.
func compareWatchlistTronWallet(previous *watchlistTronWalletSnapshot, current watchlistTronWalletSnapshot, fresh []watchlistTronTransfer) []watchlistAlertCandidate {
	usdtContract, _ := tron.Address(watchlistTronUSDTContract)
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
		amount := formatWatchlistTronAmount(transfer.ValueRaw, transfer.Decimals) + " " + transfer.assetLabel()
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
				PreviousValue: nil, CurrentValue: transfer.TransactionID, Evidence: evidence,
			})
			continue
		}
		alerts = append(alerts, watchlistAlertCandidate{
			EventType: "tron_outbound_transfer", Severity: "high", Title: "Outbound transfer from a watched Tron wallet",
			Message:       amount + " was sent to " + transfer.displayTo() + ". If you did not authorize this, treat the wallet as compromised. " + stop,
			PreviousValue: nil, CurrentValue: transfer.TransactionID, Evidence: evidence,
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
		if previousKnown == "" && previous.USDTFreezeState != watchlistTronFreezeUnknown {
			previousKnown = previous.USDTFreezeState
		}
	}
	switch {
	case previousKnown == "" && current.USDTFreezeState == watchlistTronFreezeFrozen:
		alerts = append(alerts, watchlistAlertCandidate{
			EventType: "tron_usdt_frozen_present", Severity: "info", Title: "USDT on this wallet is frozen by the issuer",
			Message: "Tether's USDT contract lists this address as frozen. It does not freeze TRX or other tokens.", CurrentValue: current.USDTFreezeState, Evidence: freezeEvidence,
		})
	case previousKnown == watchlistTronFreezeClear && current.USDTFreezeState == watchlistTronFreezeFrozen:
		alerts = append(alerts, watchlistAlertCandidate{
			EventType: "tron_usdt_frozen", Severity: "info", Title: "Issuer froze USDT on a watched wallet",
			Message: "Tether's USDT contract now lists this address as frozen. TRX and other tokens can still move.", PreviousValue: previousKnown, CurrentValue: current.USDTFreezeState, Evidence: freezeEvidence,
		})
	case previousKnown == watchlistTronFreezeFrozen && current.USDTFreezeState == watchlistTronFreezeClear:
		alerts = append(alerts, watchlistAlertCandidate{
			EventType: "tron_usdt_unfrozen", Severity: "high", Title: "USDT freeze lifted on a watched wallet",
			Message: "Tether's USDT contract no longer lists this address as frozen; its USDT can move again.", PreviousValue: previousKnown, CurrentValue: current.USDTFreezeState, Evidence: freezeEvidence,
		})
	}
	return deduplicateWatchlistAlerts(alerts)
}

func (h *Handler) refreshWatchlistTronWallet(ctx context.Context, authSubject string, target watchlistTarget, oldSnapshotRaw []byte) (watchlistTarget, int, error) {
	interval := watchlistEVMNextCheck(watchlistTronCheckInterval())
	hexAddress, err := tron.Address(strings.TrimSpace(target.Target))
	if err == nil && !watchlistTronConfigured() {
		err = errors.New("tron_provider_not_configured")
	}
	var fetched []watchlistTronTransfer
	freezeState := watchlistTronFreezeUnknown
	if err == nil {
		probeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		bridges := watchlistTronBridges()
		var trc20, trx []watchlistTronTransfer
		trc20, err = watchlistTronFetchTRC20Outbound(probeCtx, strings.TrimSpace(target.Target), hexAddress, bridges)
		if err == nil {
			trx, err = watchlistTronFetchTRXOutbound(probeCtx, strings.TrimSpace(target.Target), hexAddress, bridges)
		}
		if err == nil {
			fetched = append(trc20, trx...)
			freezeState = watchlistTronUSDTFreezeState(probeCtx, hexAddress)
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
		Network: watchlistTronNetwork, Address: hexAddress, SeenOutboundTxIDs: []string{},
		USDTFreezeState: freezeState, CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if freezeState != watchlistTronFreezeUnknown {
		current.USDTFreezeLastKnown = freezeState
	}
	if previous != nil {
		current.LatestOutboundMS = previous.LatestOutboundMS
		current.LastOutbound = previous.LastOutbound
		if current.USDTFreezeLastKnown == "" {
			// An unreadable freeze state is recorded as unknown; the last
			// readable state is carried separately so no transition is invented.
			current.USDTFreezeLastKnown = previous.USDTFreezeLastKnown
			if current.USDTFreezeLastKnown == "" && previous.USDTFreezeState != watchlistTronFreezeUnknown {
				current.USDTFreezeLastKnown = previous.USDTFreezeState
			}
		}
	}
	for i := range fetched {
		transfer := fetched[i]
		current.SeenOutboundTxIDs = append(current.SeenOutboundTxIDs, transfer.TransactionID)
		if transfer.BlockTimestampMS > current.LatestOutboundMS {
			current.LatestOutboundMS = transfer.BlockTimestampMS
			current.LastOutbound = &transfer
		}
	}
	return current
}
