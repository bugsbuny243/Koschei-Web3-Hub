package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"koschei/api/internal/radaradapter/tron"
	"koschei/api/internal/runtimehealth"
)

// Tron Flow Radar reads every solidified Tron block and follows USDT leaving
// operator seed wallets across the whole network, hop by hop, in memory.
//
// Adapter boundary: TronGrid requests and Tron protocol decoding live in
// radaradapter/tron. This consumer only sees normalized BlockTransfer,
// freeze-state and balance observations.
//
// Storage boundary: nothing per block, per transfer or per followed address
// is persisted. Followed addresses live in a bounded in-memory map and are
// lost on restart. Only findings an operator can act on leave the process,
// through the injected sink (security alerts upsert on a dedupe key, so a
// repeated finding updates one row instead of adding rows), and findings are
// capped per hour.
//
// Truth boundary: a followed address is "received funds from a seed within N
// hops", not "controlled by the attacker". A holder finding is made only after
// the current USDT balance is read. Koschei cannot stop or reverse a transfer;
// findings name who can hold funds (Tether for USDT, the bridge operator, the
// receiving party). The radar is observe-only and off by default.

const (
	TronFlowRadarHealthID = "worker.tron-flow-radar"

	tronFlowSeedRefresh     = 5 * time.Minute
	tronFlowRecentFindings  = 200
	tronFlowTopFollowed     = 25
	tronFlowMaxHolderChecks = 5 // per cycle, sequential, outside the state lock
	tronFlowMaxCheckTries   = 6
	tronFlowCheckBaseDelay  = time.Minute
	// A busy recipient is re-checked at most this often after a readable
	// check found its balance below the threshold.
	tronFlowRecheckInterval = 10 * time.Minute
)

type TronBridge struct {
	Label  string `json:"label"`
	Source string `json:"source"`
}

// TronPinnedBridges reads WATCHLIST_TRON_BRIDGE_ADDRESSES_JSON. Entries need a
// valid Tron address, a label and an HTTPS attribution source; anything else
// is ignored, so an empty or malformed registry tags nothing. Keys are 41-hex.
func TronPinnedBridges() map[string]TronBridge {
	out := map[string]TronBridge{}
	raw := strings.TrimSpace(os.Getenv("WATCHLIST_TRON_BRIDGE_ADDRESSES_JSON"))
	if raw == "" {
		return out
	}
	var entries map[string]TronBridge
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
		out[canonical] = TronBridge{Label: label, Source: source.String()}
	}
	return out
}

type TronFlowRadarConfig struct {
	Endpoint       string
	APIKey         string
	Seeds          []string // 41-hex
	MinFollowRaw   *big.Int
	BridgeAlertRaw *big.Int
	FreezeCheckRaw *big.Int
	MaxHops        int
	BurstCount     int
	BurstWindow    time.Duration
	MaxTracked     int
	MaxFindings    int // per hour
	Poll           time.Duration
	MaxBlocks      int // per cycle
	Backfill       uint64
}

func tronFlowEnvInt(name string, fallback, minimum, maximum int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil {
		return fallback
	}
	return max(minimum, min(maximum, value))
}

func tronFlowUSDTRaw(whole int) *big.Int {
	return new(big.Int).Mul(big.NewInt(int64(whole)), new(big.Int).Exp(big.NewInt(10), big.NewInt(tron.USDTDecimals), nil))
}

// TronFlowRadarEnabled is the observe-only opt-in; the radar never starts by
// default. It accepts 1, true, yes or on.
func TronFlowRadarEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("KOSCHEI_TRON_FLOW_RADAR_ENABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func LoadTronFlowRadarConfigFromEnv() TronFlowRadarConfig {
	endpoint := strings.TrimSpace(os.Getenv("KOSCHEI_TRON_FLOW_RADAR_RPC_URL"))
	if endpoint == "" {
		endpoint = "https://api.trongrid.io"
	}
	seeds := []string{}
	for _, raw := range strings.Split(os.Getenv("KOSCHEI_TRON_FLOW_RADAR_SEEDS"), ",") {
		if canonical, err := tron.Address(strings.TrimSpace(raw)); err == nil {
			seeds = append(seeds, canonical)
		}
	}
	return TronFlowRadarConfig{
		Endpoint:       endpoint,
		APIKey:         strings.TrimSpace(os.Getenv("TRONGRID_API_KEY")),
		Seeds:          seeds,
		MinFollowRaw:   tronFlowUSDTRaw(tronFlowEnvInt("KOSCHEI_TRON_FLOW_RADAR_MIN_FOLLOW_USDT", 10_000, 1, 1_000_000_000)),
		BridgeAlertRaw: tronFlowUSDTRaw(tronFlowEnvInt("KOSCHEI_TRON_FLOW_RADAR_BRIDGE_ALERT_USDT", 1_000_000, 1, 1_000_000_000)),
		FreezeCheckRaw: tronFlowUSDTRaw(tronFlowEnvInt("KOSCHEI_TRON_FLOW_RADAR_FREEZE_CHECK_USDT", 1_000_000, 1, 1_000_000_000)),
		MaxHops:        tronFlowEnvInt("KOSCHEI_TRON_FLOW_RADAR_MAX_HOPS", 4, 1, 8),
		BurstCount:     tronFlowEnvInt("KOSCHEI_TRON_FLOW_RADAR_BURST_COUNT", 10, 2, 1000),
		BurstWindow:    time.Duration(tronFlowEnvInt("KOSCHEI_TRON_FLOW_RADAR_BURST_WINDOW_SECONDS", 600, 60, 86400)) * time.Second,
		MaxTracked:     tronFlowEnvInt("KOSCHEI_TRON_FLOW_RADAR_MAX_TRACKED", 20_000, 100, 200_000),
		MaxFindings:    tronFlowEnvInt("KOSCHEI_TRON_FLOW_RADAR_MAX_FINDINGS_PER_HOUR", 30, 1, 1000),
		Poll:           time.Duration(tronFlowEnvInt("KOSCHEI_TRON_FLOW_RADAR_POLL_SECONDS", 3, 1, 60)) * time.Second,
		MaxBlocks:      tronFlowEnvInt("KOSCHEI_TRON_FLOW_RADAR_MAX_BLOCKS_PER_CYCLE", 20, 1, 200),
		Backfill:       uint64(tronFlowEnvInt("KOSCHEI_TRON_FLOW_RADAR_BACKFILL_BLOCKS", 0, 0, 28_800)),
	}
}

type TronFlowFinding struct {
	Kind       string         `json:"kind"`
	Severity   string         `json:"severity"`
	Address    string         `json:"address"`
	Title      string         `json:"title"`
	Message    string         `json:"message"`
	DedupeKey  string         `json:"dedupe_key"`
	ObservedAt time.Time      `json:"observed_at"`
	Evidence   map[string]any `json:"evidence"`
}

type TronFlowSink func(context.Context, TronFlowFinding)

// TronHolderObservation is the current issuer freeze state and, when the
// address is not frozen, its current USDT balance (nil when unreadable).
type TronHolderObservation struct {
	Freeze  string
	Balance *big.Int
}

// TronHolderChecker reads a holder observation. It performs I/O and is always
// called outside the radar state lock.
type TronHolderChecker func(context.Context, string) TronHolderObservation

type tronFlowNode struct {
	hop          int
	origin       string
	seed         bool
	taintedRaw   *big.Int
	lastSeenMS   int64
	outTimesMS   []int64
	burstUntilMS int64
	holderDone   bool
	freezeState  string
	balanceRaw   *big.Int
	lastCheck    time.Time
	lastTransfer tron.BlockTransfer
}

type tronFlowPendingCheck struct {
	attempts int
	next     time.Time
}

// tronFlowEffects are produced under the state lock without I/O and carried
// out after it is released.
type tronFlowEffects struct {
	findings []TronFlowFinding
	checks   []string // addresses needing a holder check
}

// tronFlowState is the in-memory graph. It performs no I/O and is guarded by
// the radar mutex.
type tronFlowState struct {
	cfg        TronFlowRadarConfig
	bridges    map[string]TronBridge
	nodes      map[string]*tronFlowNode
	pending    map[string]tronFlowPendingCheck
	findings   []TronFlowFinding
	hourStart  time.Time
	hourCount  int
	suppressed int
	transfers  uint64
	now        func() time.Time
}

func newTronFlowState(cfg TronFlowRadarConfig, bridges map[string]TronBridge) *tronFlowState {
	s := &tronFlowState{cfg: cfg, bridges: bridges, nodes: map[string]*tronFlowNode{}, pending: map[string]tronFlowPendingCheck{}, now: time.Now}
	s.setSeeds(cfg.Seeds)
	return s
}

// setSeeds marks seed wallets as hop 0 and keeps the graph within MaxTracked.
// Seeds take priority: when there are more seeds than the bound, only the
// first MaxTracked are kept. A former seed stays followed as an ordinary node
// and is evictable.
func (s *tronFlowState) setSeeds(seeds []string) {
	unique := []string{}
	seen := map[string]struct{}{}
	for _, seed := range seeds {
		if _, ok := seen[seed]; ok {
			continue
		}
		seen[seed] = struct{}{}
		unique = append(unique, seed)
	}
	if len(unique) > s.cfg.MaxTracked {
		log.Printf("tron flow radar: %d seed(s) exceed max tracked %d; extra seeds ignored", len(unique)-s.cfg.MaxTracked, s.cfg.MaxTracked)
		unique = unique[:s.cfg.MaxTracked]
	}
	current := map[string]struct{}{}
	for _, seed := range unique {
		current[seed] = struct{}{}
	}
	for address, node := range s.nodes {
		if _, ok := current[address]; !ok && node.seed {
			node.seed = false
		}
	}
	for _, seed := range unique {
		node := s.nodes[seed]
		if node == nil {
			node = &tronFlowNode{taintedRaw: new(big.Int)}
			s.nodes[seed] = node
		}
		node.hop, node.origin, node.seed = 0, seed, true
	}
	s.enforceBound(0)
}

// enforceBound evicts least recently active non-seed nodes until there is
// room for `room` more nodes within MaxTracked.
func (s *tronFlowState) enforceBound(room int) {
	for len(s.nodes)+room > s.cfg.MaxTracked {
		oldest, oldestMS := "", int64(1<<62)
		for address, node := range s.nodes {
			if !node.seed && node.lastSeenMS < oldestMS {
				oldest, oldestMS = address, node.lastSeenMS
			}
		}
		if oldest == "" {
			return // only seeds remain
		}
		delete(s.nodes, oldest)
		delete(s.pending, oldest)
	}
}

func tronFlowAmount(value *big.Int) string {
	return tron.FormatUnits(value, tron.USDTDecimals) + " USDT"
}

func tronFlowDisplay(hexAddress string) string {
	if encoded := tron.Base58(hexAddress); encoded != "" {
		return encoded
	}
	return hexAddress
}

// admit applies the hourly cap and records an admitted finding. It does no I/O.
func (s *tronFlowState) admit(finding TronFlowFinding) (TronFlowFinding, bool) {
	now := s.now()
	if now.Sub(s.hourStart) >= time.Hour {
		if s.suppressed > 0 {
			log.Printf("tron flow radar: %d finding(s) suppressed by the hourly cap", s.suppressed)
		}
		s.hourStart, s.hourCount, s.suppressed = now, 0, 0
	}
	if s.hourCount >= s.cfg.MaxFindings {
		s.suppressed++
		return finding, false
	}
	s.hourCount++
	finding.ObservedAt = now.UTC()
	if finding.Evidence == nil {
		finding.Evidence = map[string]any{}
	}
	finding.Evidence["source"] = "TronGrid walletsolidity gettransactioninfobyblocknum USDT Transfer logs"
	finding.Evidence["identity_or_intent_claim"] = false
	finding.Evidence["koschei_can_reverse_transfer"] = false
	finding.Evidence["follow_state"] = "in_memory_only"
	s.findings = append(s.findings, finding)
	if len(s.findings) > tronFlowRecentFindings {
		s.findings = s.findings[len(s.findings)-tronFlowRecentFindings:]
	}
	return finding, true
}

func tronFlowTransferEvidence(t tron.BlockTransfer) map[string]any {
	return map[string]any{
		"transaction_id": t.TransactionID, "event_index": t.EventIndex, "block_number": t.BlockNumber, "block_timestamp_ms": t.BlockTimestampMS,
		"from": tronFlowDisplay(t.From), "to": tronFlowDisplay(t.To), "value_raw": t.Value.String(), "amount": tronFlowAmount(t.Value),
		"token_contract": tron.USDTContract,
	}
}

// process updates the in-memory graph for one transfer. It performs no I/O;
// findings and holder checks are returned for the caller to carry out.
func (s *tronFlowState) process(t tron.BlockTransfer) tronFlowEffects {
	s.transfers++
	effects := tronFlowEffects{}
	bridge, toBridge := s.bridges[t.To]
	source := s.nodes[t.From]

	if source == nil {
		// Network-wide signal: a very large USDT deposit into a pinned bridge,
		// even when it does not descend from a seed.
		if toBridge && t.Value.Cmp(s.cfg.BridgeAlertRaw) >= 0 {
			evidence := tronFlowTransferEvidence(t)
			evidence["bridge_label"], evidence["bridge_source"] = bridge.Label, bridge.Source
			evidence["response_options"] = []string{"bridge_operator_notice:" + bridge.Label}
			effects.findings = append(effects.findings, TronFlowFinding{
				Kind: "tron_large_bridge_inflow", Severity: "info", Address: tronFlowDisplay(t.From),
				Title:     "Large USDT deposit into a pinned bridge",
				Message:   fmt.Sprintf("%s moved from %s into %s. Not linked to a seed wallet.", tronFlowAmount(t.Value), tronFlowDisplay(t.From), bridge.Label),
				DedupeKey: "tron-flow:bridge-inflow:" + t.EventID(), Evidence: evidence,
			})
		}
		return effects
	}
	if t.Value.Cmp(s.cfg.MinFollowRaw) < 0 {
		return effects
	}
	source.lastSeenMS = t.BlockTimestampMS
	responseOptions := []string{"issuer_freeze_request:tether_usdt", "receiving_party_notice"}

	windowStart := t.BlockTimestampMS - s.cfg.BurstWindow.Milliseconds()
	kept := source.outTimesMS[:0]
	for _, ts := range source.outTimesMS {
		if ts >= windowStart {
			kept = append(kept, ts)
		}
	}
	if source.seed && len(kept) == 0 {
		evidence := tronFlowTransferEvidence(t)
		evidence["response_options"] = responseOptions
		effects.findings = append(effects.findings, TronFlowFinding{
			Kind: "tron_flow_seed_outflow", Severity: "high", Address: tronFlowDisplay(t.From),
			Title:     "Seed wallet started moving USDT",
			Message:   fmt.Sprintf("%s sent %s to %s. Every recipient is now followed up to %d hops across the network.", tronFlowDisplay(t.From), tronFlowAmount(t.Value), tronFlowDisplay(t.To), s.cfg.MaxHops),
			DedupeKey: "tron-flow:seed-outflow:" + t.From, Evidence: evidence,
		})
	}
	source.outTimesMS = append(kept, t.BlockTimestampMS)
	if len(source.outTimesMS) >= s.cfg.BurstCount && t.BlockTimestampMS >= source.burstUntilMS {
		source.burstUntilMS = t.BlockTimestampMS + s.cfg.BurstWindow.Milliseconds()
		evidence := tronFlowTransferEvidence(t)
		evidence["hop"], evidence["origin_seed"] = source.hop, tronFlowDisplay(source.origin)
		evidence["transfers_in_window"], evidence["window_seconds"] = len(source.outTimesMS), int(s.cfg.BurstWindow.Seconds())
		evidence["response_options"] = responseOptions
		effects.findings = append(effects.findings, TronFlowFinding{
			Kind: "tron_flow_fan_out", Severity: "high", Address: tronFlowDisplay(t.From),
			Title:     "Scripted fan-out from a followed wallet",
			Message:   fmt.Sprintf("%s (hop %d from %s) sent %d USDT transfers of at least the follow threshold within %s.", tronFlowDisplay(t.From), source.hop, tronFlowDisplay(source.origin), len(source.outTimesMS), s.cfg.BurstWindow),
			DedupeKey: fmt.Sprintf("tron-flow:fan-out:%s:%d", t.From, source.burstUntilMS), Evidence: evidence,
		})
	}

	if toBridge {
		evidence := tronFlowTransferEvidence(t)
		evidence["hop"], evidence["origin_seed"] = source.hop, tronFlowDisplay(source.origin)
		evidence["bridge_label"], evidence["bridge_source"] = bridge.Label, bridge.Source
		evidence["response_options"] = []string{"bridge_operator_notice:" + bridge.Label, "issuer_freeze_request:tether_usdt"}
		effects.findings = append(effects.findings, TronFlowFinding{
			Kind: "tron_flow_bridge_exit", Severity: "critical", Address: tronFlowDisplay(t.From),
			Title:     "Followed funds entered a bridge",
			Message:   fmt.Sprintf("%s from %s (hop %d from seed %s) entered %s and may leave Tron. Koschei cannot reverse it; send the transaction ID to %s now.", tronFlowAmount(t.Value), tronFlowDisplay(t.From), source.hop, tronFlowDisplay(source.origin), bridge.Label, bridge.Label),
			DedupeKey: "tron-flow:bridge-exit:" + t.EventID(), Evidence: evidence,
		})
		return effects
	}
	if source.hop >= s.cfg.MaxHops {
		return effects
	}
	target := s.nodes[t.To]
	if target == nil {
		s.enforceBound(1)
		target = &tronFlowNode{hop: source.hop + 1, origin: source.origin, taintedRaw: new(big.Int)}
		s.nodes[t.To] = target
	} else if !target.seed && source.hop+1 < target.hop {
		target.hop, target.origin = source.hop+1, source.origin
	}
	target.lastSeenMS = t.BlockTimestampMS
	target.lastTransfer = t
	target.taintedRaw.Add(target.taintedRaw, t.Value)
	if !target.seed && !target.holderDone && target.taintedRaw.Cmp(s.cfg.FreezeCheckRaw) >= 0 &&
		(target.lastCheck.IsZero() || s.now().Sub(target.lastCheck) >= tronFlowRecheckInterval) {
		if _, queued := s.pending[t.To]; !queued {
			effects.checks = append(effects.checks, t.To)
		}
	}
	return effects
}

// applyHolderCheck records a holder observation. A finding is produced only
// when the issuer has not frozen the address and its current USDT balance is
// at least the freeze-check threshold. Unreadable states are retried with
// backoff, independent of new receipts, up to a bounded number of attempts.
func (s *tronFlowState) applyHolderCheck(address string, observed TronHolderObservation) tronFlowEffects {
	effects := tronFlowEffects{}
	node := s.nodes[address]
	if node == nil || node.seed {
		delete(s.pending, address)
		return effects
	}
	node.freezeState = observed.Freeze
	readable := observed.Freeze == tron.FreezeFrozen || (observed.Freeze == tron.FreezeClear && observed.Balance != nil)
	if !readable {
		retry := s.pending[address]
		retry.attempts++
		if retry.attempts >= tronFlowMaxCheckTries {
			delete(s.pending, address)
			return effects
		}
		retry.next = s.now().Add(tronFlowCheckBaseDelay << (retry.attempts - 1))
		s.pending[address] = retry
		return effects
	}
	delete(s.pending, address)
	node.lastCheck = s.now()
	node.balanceRaw = observed.Balance
	if observed.Freeze == tron.FreezeFrozen {
		node.holderDone = true
		return effects
	}
	if observed.Balance.Cmp(s.cfg.FreezeCheckRaw) < 0 {
		// Funds already moved on; a later qualifying receipt re-checks.
		return effects
	}
	node.holderDone = true
	evidence := tronFlowTransferEvidence(node.lastTransfer)
	evidence["hop"], evidence["origin_seed"] = node.hop, tronFlowDisplay(node.origin)
	evidence["followed_received_raw"], evidence["followed_received"] = node.taintedRaw.String(), tronFlowAmount(node.taintedRaw)
	evidence["current_usdt_balance_raw"], evidence["current_usdt_balance"] = observed.Balance.String(), tronFlowAmount(observed.Balance)
	evidence["balance_source"] = "TRC-20 USDT balanceOf(address) constant call at the node's latest state"
	evidence["usdt_freeze_state"] = observed.Freeze
	evidence["response_options"] = []string{"issuer_freeze_request:tether_usdt"}
	effects.findings = append(effects.findings, TronFlowFinding{
		Kind: "tron_flow_freeze_candidate", Severity: "high", Address: tronFlowDisplay(address),
		Title: "Unfrozen address holds USDT after receiving followed funds",
		Message: fmt.Sprintf("%s received %s descending from seed %s (hop %d) and currently holds %s USDT. Tether has not frozen it. Only Tether can freeze it; report the address and transaction IDs to Tether and law enforcement now.",
			tronFlowDisplay(address), tronFlowAmount(node.taintedRaw), tronFlowDisplay(node.origin), node.hop, tron.FormatUnits(observed.Balance, tron.USDTDecimals)),
		DedupeKey: "tron-flow:freeze-candidate:" + address, Evidence: evidence,
	})
	return effects
}

// dueChecks returns up to limit queued holder checks whose retry time passed.
func (s *tronFlowState) dueChecks(limit int) []string {
	now := s.now()
	due := []string{}
	for address, retry := range s.pending {
		if !retry.next.After(now) {
			due = append(due, address)
		}
	}
	sort.Strings(due)
	if len(due) > limit {
		due = due[:limit]
	}
	return due
}

type TronFlowFollowedAddress struct {
	Address         string `json:"address"`
	Hop             int    `json:"hop"`
	OriginSeed      string `json:"origin_seed"`
	Seed            bool   `json:"seed"`
	ReceivedRaw     string `json:"followed_received_raw"`
	Received        string `json:"followed_received"`
	CurrentBalance  string `json:"current_usdt_balance,omitempty"`
	LastActivityMS  int64  `json:"last_activity_block_timestamp_ms"`
	USDTFreezeState string `json:"usdt_freeze_state"`
}

type TronFlowRadarSettings struct {
	MinFollow          string `json:"min_follow"`
	MaxHops            int    `json:"max_hops"`
	BurstCount         int    `json:"burst_count"`
	BurstWindowSeconds int    `json:"burst_window_seconds"`
	FreezeCheck        string `json:"freeze_check"`
	BridgeAlert        string `json:"bridge_alert"`
	PinnedBridges      int    `json:"pinned_bridges"`
	MaxTracked         int    `json:"max_tracked"`
	MaxFindingsPerHour int    `json:"max_findings_per_hour"`
}

// topFollowed lists seeds first, then followed addresses holding the most
// followed USDT.
func (s *tronFlowState) topFollowed(limit int) []TronFlowFollowedAddress {
	type ranked struct {
		row      TronFlowFollowedAddress
		received *big.Int
	}
	all := make([]ranked, 0, len(s.nodes))
	for address, node := range s.nodes {
		row := TronFlowFollowedAddress{
			Address: tronFlowDisplay(address), Hop: node.hop, OriginSeed: tronFlowDisplay(node.origin), Seed: node.seed,
			ReceivedRaw: node.taintedRaw.String(), Received: tronFlowAmount(node.taintedRaw), LastActivityMS: node.lastSeenMS,
			USDTFreezeState: firstNonEmptyString(node.freezeState, "not_checked"),
		}
		if node.balanceRaw != nil {
			row.CurrentBalance = tronFlowAmount(node.balanceRaw)
		}
		all = append(all, ranked{row: row, received: node.taintedRaw})
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].row.Seed != all[j].row.Seed {
			return all[i].row.Seed
		}
		if cmp := all[i].received.Cmp(all[j].received); cmp != 0 {
			return cmp > 0
		}
		return all[i].row.Address < all[j].row.Address
	})
	if len(all) > limit {
		all = all[:limit]
	}
	out := make([]TronFlowFollowedAddress, 0, len(all))
	for _, item := range all {
		out = append(out, item.row)
	}
	return out
}

type TronFlowRadarSnapshot struct {
	Running        bool                      `json:"running"`
	Cursor         uint64                    `json:"cursor,string"`
	Head           uint64                    `json:"head,string"`
	Seeds          int                       `json:"seeds"`
	Followed       int                       `json:"followed_addresses"`
	PendingChecks  int                       `json:"pending_holder_checks"`
	Transfers      uint64                    `json:"usdt_transfers_processed,string"`
	Suppressed     int                       `json:"findings_suppressed_this_hour"`
	RecentFindings []TronFlowFinding         `json:"recent_findings"`
	TopFollowed    []TronFlowFollowedAddress `json:"top_followed"`
	Settings       TronFlowRadarSettings     `json:"settings"`
	LastError      string                    `json:"last_error,omitempty"`
	LastCycleAt    *time.Time                `json:"last_cycle_at,omitempty"`
	Storage        string                    `json:"storage"`
}

// TronFlowRadar owns the loop. mu guards state and progress only; provider
// requests and the alert sink always run without holding it, so Snapshot
// stays responsive during provider degradation.
type TronFlowRadar struct {
	cfg    TronFlowRadarConfig
	grid   tron.GridClient
	holder TronHolderChecker
	sink   TronFlowSink
	health *runtimehealth.Registry
	db     *sql.DB

	mu        sync.Mutex
	state     *tronFlowState
	cursor    uint64
	head      uint64
	running   bool
	lastError string
	lastCycle time.Time
}

var activeTronFlowRadar atomic.Pointer[TronFlowRadar]

// ActiveTronFlowRadar returns the radar started in this process, or nil when
// it is disabled or this process role runs no background workers.
func ActiveTronFlowRadar() *TronFlowRadar { return activeTronFlowRadar.Load() }

func NewTronFlowRadar(cfg TronFlowRadarConfig, db *sql.DB, sink TronFlowSink, health *runtimehealth.Registry) *TronFlowRadar {
	r := &TronFlowRadar{cfg: cfg, sink: sink, health: health, db: db,
		grid: tron.GridClient{BaseURL: cfg.Endpoint, APIKey: cfg.APIKey,
			HTTP: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}}
	r.holder = r.gridHolderObservation
	r.state = newTronFlowState(cfg, TronPinnedBridges())
	return r
}

func (r *TronFlowRadar) gridHolderObservation(ctx context.Context, address string) TronHolderObservation {
	callCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	observed := TronHolderObservation{Freeze: r.grid.USDTFreezeState(callCtx, address)}
	if observed.Freeze == tron.FreezeClear {
		if balance, err := r.grid.USDTBalance(callCtx, address); err == nil {
			observed.Balance = balance
		}
	}
	return observed
}

func (r *TronFlowRadar) Snapshot() TronFlowRadarSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	seeds := 0
	for _, node := range r.state.nodes {
		if node.seed {
			seeds++
		}
	}
	findings := append([]TronFlowFinding(nil), r.state.findings...)
	sort.SliceStable(findings, func(i, j int) bool { return findings[i].ObservedAt.After(findings[j].ObservedAt) })
	snapshot := TronFlowRadarSnapshot{Running: r.running, Cursor: r.cursor, Head: r.head, Seeds: seeds, Followed: len(r.state.nodes),
		PendingChecks: len(r.state.pending), Transfers: r.state.transfers, Suppressed: r.state.suppressed, RecentFindings: findings,
		TopFollowed: r.state.topFollowed(tronFlowTopFollowed), LastError: r.lastError, Storage: "in_memory_only",
		Settings: TronFlowRadarSettings{
			MinFollow: tronFlowAmount(r.cfg.MinFollowRaw), MaxHops: r.cfg.MaxHops, BurstCount: r.cfg.BurstCount,
			BurstWindowSeconds: int(r.cfg.BurstWindow.Seconds()), FreezeCheck: tronFlowAmount(r.cfg.FreezeCheckRaw),
			BridgeAlert: tronFlowAmount(r.cfg.BridgeAlertRaw), PinnedBridges: len(r.state.bridges), MaxTracked: r.cfg.MaxTracked,
			MaxFindingsPerHour: r.cfg.MaxFindings,
		},
	}
	if !r.lastCycle.IsZero() {
		at := r.lastCycle.UTC()
		snapshot.LastCycleAt = &at
	}
	return snapshot
}

// deliver admits findings under the lock, then sends them without it.
func (r *TronFlowRadar) deliver(ctx context.Context, findings []TronFlowFinding) {
	if len(findings) == 0 {
		return
	}
	admitted := []TronFlowFinding{}
	r.mu.Lock()
	for _, finding := range findings {
		if accepted, ok := r.state.admit(finding); ok {
			admitted = append(admitted, accepted)
		}
	}
	r.mu.Unlock()
	for _, finding := range admitted {
		log.Printf("tron flow radar finding kind=%s severity=%s address=%s", finding.Kind, finding.Severity, finding.Address)
		if r.sink != nil {
			r.sink(ctx, finding)
		}
	}
}

// runHolderChecks performs holder reads without the lock and applies each
// result under a short lock section.
func (r *TronFlowRadar) runHolderChecks(ctx context.Context, addresses []string) {
	seen := map[string]struct{}{}
	for _, address := range addresses {
		if _, ok := seen[address]; ok || ctx.Err() != nil {
			continue
		}
		seen[address] = struct{}{}
		observed := r.holder(ctx, address)
		r.mu.Lock()
		effects := r.state.applyHolderCheck(address, observed)
		r.mu.Unlock()
		r.deliver(ctx, effects.findings)
	}
}

// ingest applies a block's transfers to the graph, then carries out the
// resulting deliveries and holder checks outside the lock.
func (r *TronFlowRadar) ingest(ctx context.Context, transfers []tron.BlockTransfer) {
	all := tronFlowEffects{}
	r.mu.Lock()
	for _, transfer := range transfers {
		effects := r.state.process(transfer)
		all.findings = append(all.findings, effects.findings...)
		all.checks = append(all.checks, effects.checks...)
	}
	r.mu.Unlock()
	r.deliver(ctx, all.findings)
	if len(all.checks) > tronFlowMaxHolderChecks {
		// Defer the overflow to the retry queue instead of stalling the block loop.
		r.mu.Lock()
		for _, address := range all.checks[tronFlowMaxHolderChecks:] {
			if _, queued := r.state.pending[address]; !queued {
				r.state.pending[address] = tronFlowPendingCheck{next: r.state.now()}
			}
		}
		r.mu.Unlock()
		all.checks = all.checks[:tronFlowMaxHolderChecks]
	}
	r.runHolderChecks(ctx, all.checks)
}

// retryPendingChecks runs due queued holder checks, bounded per cycle.
func (r *TronFlowRadar) retryPendingChecks(ctx context.Context) {
	r.mu.Lock()
	due := r.state.dueChecks(tronFlowMaxHolderChecks)
	r.mu.Unlock()
	r.runHolderChecks(ctx, due)
}

func (r *TronFlowRadar) refreshSeeds(ctx context.Context) {
	seeds := append([]string(nil), r.cfg.Seeds...)
	if r.db != nil {
		rows, err := r.db.QueryContext(ctx, `
			SELECT target FROM watchlist_targets
			WHERE network='tron-mainnet' AND target_type='wallet' AND status='active'
			ORDER BY created_at DESC LIMIT 500`)
		if err == nil {
			for rows.Next() {
				var target string
				if rows.Scan(&target) == nil {
					if canonical, err := tron.Address(strings.TrimSpace(target)); err == nil {
						seeds = append(seeds, canonical)
					}
				}
			}
			rows.Close()
		}
	}
	r.mu.Lock()
	r.state.setSeeds(seeds)
	r.mu.Unlock()
}

// cycle processes solidified blocks from the cursor toward the head, bounded
// per cycle, then retries due holder checks. It returns blocks processed.
func (r *TronFlowRadar) cycle(ctx context.Context) (int, error) {
	head, err := r.grid.SolidHead(ctx)
	if err != nil {
		return 0, err
	}
	r.mu.Lock()
	r.head = head
	if r.cursor == 0 {
		start := head
		if r.cfg.Backfill > 0 && head > r.cfg.Backfill {
			start = head - r.cfg.Backfill
		}
		r.cursor = start - 1
	}
	cursor := r.cursor
	r.mu.Unlock()

	processed := 0
	for cursor < head && processed < r.cfg.MaxBlocks && ctx.Err() == nil {
		next := cursor + 1
		transfers, err := r.grid.SolidBlockUSDTTransfers(ctx, next)
		if err != nil {
			return processed, err
		}
		r.ingest(ctx, transfers)
		r.mu.Lock()
		r.cursor = next
		r.mu.Unlock()
		cursor = next
		processed++
	}
	r.retryPendingChecks(ctx)
	return processed, nil
}

// tronFlowCatchupDelay prevents an unbounded hot loop while behind the head.
// Keep the normal polling delay, with a minimum pause between full batches.
func tronFlowCatchupDelay(poll time.Duration) time.Duration {
	return max(poll, 2*time.Second)
}

// tronFlowFailureBackoff avoids retrying a quota-exhausted provider every five
// seconds. Keep the existing exponential ceiling for all failure categories.
func tronFlowFailureBackoff(previous time.Duration, err error) time.Duration {
	wait := min(max(2*previous, 5*time.Second), 5*time.Minute)
	if err != nil && strings.Contains(err.Error(), "tron_provider_http_429") {
		wait = max(wait, time.Minute)
	}
	return wait
}

// Start runs the radar until ctx ends.
func (r *TronFlowRadar) Start(ctx context.Context) func() {
	if r.health != nil {
		r.health.Register(TronFlowRadarHealthID, "worker", "tron-mainnet", true)
	}
	runCtx, cancel := context.WithCancel(ctx)
	activeTronFlowRadar.Store(r)
	go func() {
		r.mu.Lock()
		r.running = true
		r.mu.Unlock()
		defer func() {
			r.mu.Lock()
			r.running = false
			r.mu.Unlock()
		}()
		r.refreshSeeds(runCtx)
		lastSeedRefresh := time.Now()
		backoff := time.Duration(0)
		for runCtx.Err() == nil {
			if time.Since(lastSeedRefresh) >= tronFlowSeedRefresh {
				r.refreshSeeds(runCtx)
				lastSeedRefresh = time.Now()
			}
			processed, err := r.cycle(runCtx)
			r.mu.Lock()
			r.lastCycle, r.lastError = time.Now(), ""
			if err != nil {
				r.lastError = err.Error() // error categories only; never URLs or keys
			}
			r.mu.Unlock()
			snapshot := r.Snapshot()
			head, cursor := snapshot.Head, snapshot.Cursor
			if r.health != nil {
				r.health.RecordIngestProgress(TronFlowRadarHealthID, &head, &cursor, err != nil, false)
				if err != nil {
					r.health.Failure(TronFlowRadarHealthID, err)
				} else {
					r.health.Success(TronFlowRadarHealthID, processed)
				}
			}
			wait := r.cfg.Poll
			if err != nil {
				log.Printf("tron flow radar cycle failed: %v", err)
				backoff = tronFlowFailureBackoff(backoff, err)
				wait = backoff
			} else {
				backoff = 0
				// Catch-up still makes provider requests. Never spin at zero
				// delay after a full batch: the next cycle reads the head and
				// more block receipts, which can exhaust TronGrid quotas.
				if processed >= r.cfg.MaxBlocks {
					wait = tronFlowCatchupDelay(r.cfg.Poll)
				}
			}
			select {
			case <-runCtx.Done():
			case <-time.After(wait):
			}
		}
	}()
	log.Printf("tron flow radar started: mode=observe_only storage=in_memory seeds=%d max_hops=%d max_tracked=%d poll=%s",
		len(r.cfg.Seeds), r.cfg.MaxHops, r.cfg.MaxTracked, r.cfg.Poll)
	return cancel
}
