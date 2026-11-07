package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"koschei/api/internal/radaradapter/evm"
	"koschei/api/internal/runtimehealth"
)

// EVM Flow Radar is the EVM counterpart of Tron Flow Radar. For each enabled
// network it reads ERC-20 Transfer logs of the issuer stablecoins (USDT,
// USDC) for every block, a few confirmations behind the head, and follows
// funds leaving operator seed wallets hop by hop, in memory.
//
// Adapter boundary: JSON-RPC requests and ABI decoding live in
// radaradapter/evm (flow.go). This consumer only sees normalized
// FlowTransfer, balance and freeze-state observations.
//
// Storage boundary: nothing per block, per transfer or per followed address
// is persisted. Followed addresses live in a bounded in-memory map per
// network and are lost on restart. Only findings leave the process, through
// the injected sink (security alerts upsert on a dedupe key), capped per hour.
// No ClickHouse or other analytics store is needed.
//
// Truth boundary: a followed address "received stablecoins descending from a
// seed within N hops"; it is not an attribution of control or intent. A holder
// finding is made only after the current balance and issuer freeze state are
// read. Koschei cannot stop or reverse a transfer; findings name who can hold
// funds (Tether, Circle, a bridge operator, the receiving party). The radar is
// observe-only and off by default.

const (
	evmFlowSeedRefresh     = 5 * time.Minute
	evmFlowRecentFindings  = 200
	evmFlowTopFollowed     = 25
	evmFlowMaxHolderChecks = 5
	evmFlowMaxCheckTries   = 6
	evmFlowCheckBaseDelay  = time.Minute
	evmFlowRecheckInterval = 10 * time.Minute
	// After this many successful ranges, a shrunken block range doubles again
	// toward the configured maximum.
	evmFlowRangeGrowAfter = 50
	evmFlowCatchUpWait    = 500 * time.Millisecond
)

var evmFlowZeroAddress = "0x" + strings.Repeat("0", 40)

func EVMFlowRadarHealthID(network string) string { return "worker.evm-flow-radar." + network }

// evmFlowProfile holds per-network defaults. Confirmations keep the radar
// behind short reorgs; Poll is chosen so one poll covers roughly a free-tier
// eth_getLogs range (10 blocks) on fast chains.
type evmFlowProfile struct {
	rpcEnv        string
	confirmations uint64
	blockTime     time.Duration
	poll          time.Duration
	explorer      string
}

var evmFlowProfiles = map[string]evmFlowProfile{
	"ethereum-mainnet": {rpcEnv: "ETHEREUM_RPC_URL", confirmations: 3, blockTime: 12 * time.Second, poll: 30 * time.Second, explorer: "https://etherscan.io"},
	"base-mainnet":     {rpcEnv: "BASE_RPC_URL", confirmations: 10, blockTime: 2 * time.Second, poll: 20 * time.Second, explorer: "https://basescan.org"},
	"optimism-mainnet": {rpcEnv: "OPTIMISM_RPC_URL", confirmations: 10, blockTime: 2 * time.Second, poll: 20 * time.Second, explorer: "https://optimistic.etherscan.io"},
	"arbitrum-mainnet": {rpcEnv: "ARBITRUM_RPC_URL", confirmations: 40, blockTime: 250 * time.Millisecond, poll: 3 * time.Second, explorer: "https://arbiscan.io"},
	"polygon-mainnet":  {rpcEnv: "POLYGON_RPC_URL", confirmations: 16, blockTime: 2 * time.Second, poll: 20 * time.Second, explorer: "https://polygonscan.com"},
	"bnb-mainnet":      {rpcEnv: "BNB_RPC_URL", confirmations: 15, blockTime: 750 * time.Millisecond, poll: 8 * time.Second, explorer: "https://bscscan.com"},
}

// EVMFlowRadarExplorer returns the public explorer base URL for a network.
func EVMFlowRadarExplorer(network string) string { return evmFlowProfiles[network].explorer }

// EVMFlowRadarEnabled is the observe-only opt-in. It accepts 1, true, yes or on.
func EVMFlowRadarEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("KOSCHEI_EVM_FLOW_RADAR_ENABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// EVMFlowRadarNetworks reads KOSCHEI_EVM_FLOW_RADAR_NETWORKS (comma separated,
// default ethereum-mainnet). Unsupported names are returned separately.
func EVMFlowRadarNetworks() (supported, unsupported []string) {
	raw := strings.TrimSpace(os.Getenv("KOSCHEI_EVM_FLOW_RADAR_NETWORKS"))
	if raw == "" {
		raw = "ethereum-mainnet"
	}
	seen := map[string]struct{}{}
	for _, item := range strings.Split(raw, ",") {
		network := strings.ToLower(strings.TrimSpace(item))
		if network == "" {
			continue
		}
		if _, dup := seen[network]; dup {
			continue
		}
		seen[network] = struct{}{}
		if _, ok := evmFlowProfiles[network]; ok && len(evm.FlowTokens[network]) > 0 {
			supported = append(supported, network)
		} else {
			unsupported = append(unsupported, network)
		}
	}
	return supported, unsupported
}

type EVMBridge struct {
	Label    string   `json:"label"`
	Source   string   `json:"source"`
	Networks []string `json:"networks,omitempty"`
}

// EVMPinnedBridges reads KOSCHEI_EVM_FLOW_RADAR_BRIDGES_JSON for one network:
// {"0x...": {"label": "...", "source": "https://...", "networks": ["ethereum-mainnet"]}}.
// An entry without networks applies to every network. Entries need a valid
// address, a label and an HTTPS attribution source; malformed input tags
// nothing.
func EVMPinnedBridges(network string) map[string]EVMBridge {
	out := map[string]EVMBridge{}
	raw := strings.TrimSpace(os.Getenv("KOSCHEI_EVM_FLOW_RADAR_BRIDGES_JSON"))
	if raw == "" {
		return out
	}
	var entries map[string]EVMBridge
	if json.Unmarshal([]byte(raw), &entries) != nil {
		return out
	}
	for address, entry := range entries {
		canonical, err := evm.Address(strings.TrimSpace(address))
		label := strings.TrimSpace(entry.Label)
		source, perr := url.Parse(strings.TrimSpace(entry.Source))
		if err != nil || label == "" || len(label) > 80 || perr != nil || source.Scheme != "https" || source.Host == "" {
			continue
		}
		applies := len(entry.Networks) == 0
		for _, item := range entry.Networks {
			if strings.EqualFold(strings.TrimSpace(item), network) {
				applies = true
			}
		}
		if applies {
			out[canonical] = EVMBridge{Label: label, Source: source.String()}
		}
	}
	return out
}

type EVMFlowRadarConfig struct {
	Network           string
	Endpoint          string
	EndpointEnv       string
	Explorer          string
	Tokens            []evm.FlowToken
	Seeds             []string // 0x-lower
	MinFollowUSD6     *big.Int
	BridgeAlertUSD6   *big.Int
	FreezeCheckUSD6   *big.Int
	MaxHops           int
	BurstCount        int
	BurstWindow       time.Duration
	BlockTime         time.Duration
	MaxTracked        int
	MaxFindings       int // per hour
	Poll              time.Duration
	Confirmations     uint64
	MaxBlockRange     uint64
	MaxRangesPerCycle int
	Backfill          uint64
}

func evmFlowUSD6(whole int) *big.Int {
	return new(big.Int).Mul(big.NewInt(int64(whole)), big.NewInt(1_000_000))
}

// EVMFlowRadarSeedsFromEnv parses KOSCHEI_EVM_FLOW_RADAR_SEEDS (comma
// separated 0x addresses, applied to every enabled network).
func EVMFlowRadarSeedsFromEnv() []string {
	seeds := []string{}
	for _, raw := range strings.Split(os.Getenv("KOSCHEI_EVM_FLOW_RADAR_SEEDS"), ",") {
		if canonical, err := evm.Address(strings.TrimSpace(raw)); err == nil {
			seeds = append(seeds, canonical)
		}
	}
	return seeds
}

// LoadEVMFlowRadarConfigFromEnv builds one network's configuration. The RPC
// endpoint is the network's existing *_RPC_URL variable.
func LoadEVMFlowRadarConfigFromEnv(network string) (EVMFlowRadarConfig, bool) {
	profile, ok := evmFlowProfiles[network]
	tokens := evm.FlowTokens[network]
	if !ok || len(tokens) == 0 {
		return EVMFlowRadarConfig{}, false
	}
	poll := profile.poll
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_EVM_FLOW_RADAR_POLL_SECONDS")); raw != "" {
		poll = time.Duration(tronFlowEnvInt("KOSCHEI_EVM_FLOW_RADAR_POLL_SECONDS", int(profile.poll.Seconds()), 1, 300)) * time.Second
	}
	confirmations := profile.confirmations
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_EVM_FLOW_RADAR_CONFIRMATIONS")); raw != "" {
		confirmations = uint64(tronFlowEnvInt("KOSCHEI_EVM_FLOW_RADAR_CONFIRMATIONS", int(profile.confirmations), 1, 512))
	}
	return EVMFlowRadarConfig{
		Network: network, EndpointEnv: profile.rpcEnv, Endpoint: strings.TrimSpace(os.Getenv(profile.rpcEnv)), Explorer: profile.explorer,
		Tokens: append([]evm.FlowToken(nil), tokens...), Seeds: EVMFlowRadarSeedsFromEnv(),
		MinFollowUSD6:     evmFlowUSD6(tronFlowEnvInt("KOSCHEI_EVM_FLOW_RADAR_MIN_FOLLOW_USD", 10_000, 1, 1_000_000_000)),
		BridgeAlertUSD6:   evmFlowUSD6(tronFlowEnvInt("KOSCHEI_EVM_FLOW_RADAR_BRIDGE_ALERT_USD", 1_000_000, 1, 1_000_000_000)),
		FreezeCheckUSD6:   evmFlowUSD6(tronFlowEnvInt("KOSCHEI_EVM_FLOW_RADAR_FREEZE_CHECK_USD", 1_000_000, 1, 1_000_000_000)),
		MaxHops:           tronFlowEnvInt("KOSCHEI_EVM_FLOW_RADAR_MAX_HOPS", 4, 1, 8),
		BurstCount:        tronFlowEnvInt("KOSCHEI_EVM_FLOW_RADAR_BURST_COUNT", 10, 2, 1000),
		BurstWindow:       time.Duration(tronFlowEnvInt("KOSCHEI_EVM_FLOW_RADAR_BURST_WINDOW_SECONDS", 600, 60, 86400)) * time.Second,
		BlockTime:         profile.blockTime,
		MaxTracked:        tronFlowEnvInt("KOSCHEI_EVM_FLOW_RADAR_MAX_TRACKED", 20_000, 100, 200_000),
		MaxFindings:       tronFlowEnvInt("KOSCHEI_EVM_FLOW_RADAR_MAX_FINDINGS_PER_HOUR", 30, 1, 1000),
		Poll:              poll,
		Confirmations:     confirmations,
		MaxBlockRange:     uint64(tronFlowEnvInt("KOSCHEI_EVM_FLOW_RADAR_MAX_BLOCK_RANGE", 10, 1, 5000)),
		MaxRangesPerCycle: tronFlowEnvInt("KOSCHEI_EVM_FLOW_RADAR_MAX_RANGES_PER_CYCLE", 5, 1, 100),
		Backfill:          uint64(tronFlowEnvInt("KOSCHEI_EVM_FLOW_RADAR_BACKFILL_BLOCKS", 0, 0, 50_000)),
	}, true
}

type EVMFlowFinding struct {
	Network    string         `json:"network"`
	Kind       string         `json:"kind"`
	Severity   string         `json:"severity"`
	Address    string         `json:"address"`
	Title      string         `json:"title"`
	Message    string         `json:"message"`
	DedupeKey  string         `json:"dedupe_key"`
	ObservedAt time.Time      `json:"observed_at"`
	Evidence   map[string]any `json:"evidence"`
}

type EVMFlowSink func(context.Context, EVMFlowFinding)

// EVMHolderObservation is the current issuer freeze state and, when not
// frozen, the current token balance (nil when unreadable).
type EVMHolderObservation struct {
	Freeze  string
	Balance *big.Int
}

// EVMHolderChecker reads one holder/token observation. It performs I/O and is
// always called outside the radar state lock.
type EVMHolderChecker func(ctx context.Context, address string, token evm.FlowToken) EVMHolderObservation

type evmFlowHolding struct {
	receivedRaw *big.Int
	last        evm.FlowTransfer
	done        bool
	freeze      string
	balanceRaw  *big.Int
	lastCheck   time.Time
}

type evmFlowNode struct {
	hop          int
	origin       string
	seed         bool
	receivedUSD6 *big.Int
	lastBlock    uint64
	outBlocks    []uint64
	burstUntil   uint64
	holdings     map[string]*evmFlowHolding // token address -> holding
}

type evmFlowPendingCheck struct {
	attempts int
	next     time.Time
}

type evmFlowEffects struct {
	findings []EVMFlowFinding
	checks   []string // "address|token" keys needing a holder check
}

// evmFlowState is the in-memory graph for one network. It performs no I/O
// and is guarded by the radar mutex.
type evmFlowState struct {
	cfg        EVMFlowRadarConfig
	tokens     map[string]evm.FlowToken
	bridges    map[string]EVMBridge
	nodes      map[string]*evmFlowNode
	pending    map[string]evmFlowPendingCheck
	findings   []EVMFlowFinding
	hourStart  time.Time
	hourCount  int
	suppressed int
	transfers  uint64
	now        func() time.Time
}

func newEVMFlowState(cfg EVMFlowRadarConfig, bridges map[string]EVMBridge) *evmFlowState {
	tokens := map[string]evm.FlowToken{}
	for _, token := range cfg.Tokens {
		tokens[strings.ToLower(token.Address)] = token
	}
	s := &evmFlowState{cfg: cfg, tokens: tokens, bridges: bridges, nodes: map[string]*evmFlowNode{}, pending: map[string]evmFlowPendingCheck{}, now: time.Now}
	s.setSeeds(cfg.Seeds)
	return s
}

func newEVMFlowNode() *evmFlowNode {
	return &evmFlowNode{receivedUSD6: new(big.Int), holdings: map[string]*evmFlowHolding{}}
}

// windowBlocks converts the burst window to blocks using the network's block
// time, so bursts are measured on chain time rather than processing time.
func (s *evmFlowState) windowBlocks() uint64 {
	if s.cfg.BlockTime <= 0 {
		return 1
	}
	return max(uint64(1), uint64(s.cfg.BurstWindow/s.cfg.BlockTime))
}

// setSeeds marks seed wallets as hop 0 and keeps the graph within MaxTracked.
func (s *evmFlowState) setSeeds(seeds []string) {
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
		log.Printf("evm flow radar %s: %d seed(s) exceed max tracked %d; extra seeds ignored", s.cfg.Network, len(unique)-s.cfg.MaxTracked, s.cfg.MaxTracked)
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
			node = newEVMFlowNode()
			s.nodes[seed] = node
		}
		node.hop, node.origin, node.seed = 0, seed, true
	}
	s.enforceBound(0)
}

// enforceBound evicts least recently active non-seed nodes until there is
// room for `room` more nodes within MaxTracked.
func (s *evmFlowState) enforceBound(room int) {
	for len(s.nodes)+room > s.cfg.MaxTracked {
		oldest, oldestBlock := "", uint64(1<<63)
		for address, node := range s.nodes {
			if !node.seed && node.lastBlock < oldestBlock {
				oldest, oldestBlock = address, node.lastBlock
			}
		}
		if oldest == "" {
			return
		}
		delete(s.nodes, oldest)
		for key := range s.pending {
			if strings.HasPrefix(key, oldest+"|") {
				delete(s.pending, key)
			}
		}
	}
}

func evmFlowAmount(value *big.Int, token evm.FlowToken) string {
	return evm.FormatUnits(value, token.Decimals) + " " + token.Symbol
}

func evmFlowUSD(value *big.Int) string {
	return evm.FormatUnits(value, 6) + " USD (stablecoins)"
}

func (s *evmFlowState) key(prefix string) string { return "evm-flow:" + s.cfg.Network + ":" + prefix }

// admit applies the hourly cap and records an admitted finding. No I/O.
func (s *evmFlowState) admit(finding EVMFlowFinding) (EVMFlowFinding, bool) {
	now := s.now()
	if now.Sub(s.hourStart) >= time.Hour {
		if s.suppressed > 0 {
			log.Printf("evm flow radar %s: %d finding(s) suppressed by the hourly cap", s.cfg.Network, s.suppressed)
		}
		s.hourStart, s.hourCount, s.suppressed = now, 0, 0
	}
	if s.hourCount >= s.cfg.MaxFindings {
		s.suppressed++
		return finding, false
	}
	s.hourCount++
	finding.Network = s.cfg.Network
	finding.ObservedAt = now.UTC()
	if finding.Evidence == nil {
		finding.Evidence = map[string]any{}
	}
	finding.Evidence["network"] = s.cfg.Network
	finding.Evidence["source"] = fmt.Sprintf("eth_getLogs ERC-20 Transfer logs read %d block(s) behind the head", s.cfg.Confirmations)
	finding.Evidence["identity_or_intent_claim"] = false
	finding.Evidence["koschei_can_reverse_transfer"] = false
	finding.Evidence["follow_state"] = "in_memory_only"
	s.findings = append(s.findings, finding)
	if len(s.findings) > evmFlowRecentFindings {
		s.findings = s.findings[len(s.findings)-evmFlowRecentFindings:]
	}
	return finding, true
}

func evmFlowTransferEvidence(t evm.FlowTransfer, token evm.FlowToken) map[string]any {
	return map[string]any{
		"transaction_id": t.TransactionHash, "log_index": t.LogIndex, "block_number": t.BlockNumber, "block_hash": t.BlockHash,
		"from": t.From, "to": t.To, "value_raw": t.Value.String(), "amount": evmFlowAmount(t.Value, token),
		"token_contract": token.Address, "token_symbol": token.Symbol, "token_decimals": token.Decimals,
	}
}

func evmFlowResponseOptions(token evm.FlowToken) []string {
	options := []string{}
	if token.IssuerOption != "" {
		options = append(options, token.IssuerOption)
	}
	return append(options, "receiving_party_notice")
}

// process updates the in-memory graph for one transfer. It performs no I/O;
// findings and holder checks are returned for the caller to carry out.
func (s *evmFlowState) process(t evm.FlowTransfer) evmFlowEffects {
	effects := evmFlowEffects{}
	token, known := s.tokens[t.Token]
	if !known || t.Value == nil {
		return effects
	}
	s.transfers++
	if t.From == t.To {
		return effects
	}
	usd := evm.NormalizeUSD6(t.Value, token.Decimals)
	bridge, toBridge := s.bridges[t.To]
	source := s.nodes[t.From]

	if source == nil {
		// Network-wide signal: a very large stablecoin deposit into a pinned
		// bridge, even when it does not descend from a seed.
		if toBridge && usd.Cmp(s.cfg.BridgeAlertUSD6) >= 0 {
			evidence := evmFlowTransferEvidence(t, token)
			evidence["bridge_label"], evidence["bridge_source"] = bridge.Label, bridge.Source
			evidence["response_options"] = []string{"bridge_operator_notice:" + bridge.Label}
			effects.findings = append(effects.findings, EVMFlowFinding{
				Kind: "evm_large_bridge_inflow", Severity: "info", Address: t.From,
				Title:     "Large stablecoin deposit into a pinned bridge",
				Message:   fmt.Sprintf("%s moved from %s into %s on %s. Not linked to a seed wallet.", evmFlowAmount(t.Value, token), t.From, bridge.Label, s.cfg.Network),
				DedupeKey: s.key("bridge-inflow:" + t.EventID()), Evidence: evidence,
			})
		}
		return effects
	}
	if usd.Cmp(s.cfg.MinFollowUSD6) < 0 {
		return effects
	}
	source.lastBlock = max(source.lastBlock, t.BlockNumber)
	responseOptions := evmFlowResponseOptions(token)

	window := s.windowBlocks()
	windowStart := uint64(0)
	if t.BlockNumber > window {
		windowStart = t.BlockNumber - window
	}
	kept := source.outBlocks[:0]
	for _, block := range source.outBlocks {
		if block >= windowStart {
			kept = append(kept, block)
		}
	}
	if source.seed && len(kept) == 0 {
		evidence := evmFlowTransferEvidence(t, token)
		evidence["response_options"] = responseOptions
		effects.findings = append(effects.findings, EVMFlowFinding{
			Kind: "evm_flow_seed_outflow", Severity: "high", Address: t.From,
			Title:     "Seed wallet started moving stablecoins",
			Message:   fmt.Sprintf("%s sent %s to %s on %s. Every recipient is now followed up to %d hops across the network.", t.From, evmFlowAmount(t.Value, token), t.To, s.cfg.Network, s.cfg.MaxHops),
			DedupeKey: s.key("seed-outflow:" + t.From), Evidence: evidence,
		})
	}
	source.outBlocks = append(kept, t.BlockNumber)
	if len(source.outBlocks) >= s.cfg.BurstCount && t.BlockNumber >= source.burstUntil {
		source.burstUntil = t.BlockNumber + window
		evidence := evmFlowTransferEvidence(t, token)
		evidence["hop"], evidence["origin_seed"] = source.hop, source.origin
		evidence["transfers_in_window"], evidence["window_blocks"], evidence["window_seconds"] = len(source.outBlocks), window, int(s.cfg.BurstWindow.Seconds())
		evidence["response_options"] = responseOptions
		effects.findings = append(effects.findings, EVMFlowFinding{
			Kind: "evm_flow_fan_out", Severity: "high", Address: t.From,
			Title:     "Scripted fan-out from a followed wallet",
			Message:   fmt.Sprintf("%s (hop %d from %s) sent %d stablecoin transfers of at least the follow threshold within %d blocks (about %s) on %s.", t.From, source.hop, source.origin, len(source.outBlocks), window, s.cfg.BurstWindow, s.cfg.Network),
			DedupeKey: s.key(fmt.Sprintf("fan-out:%s:%d", t.From, source.burstUntil)), Evidence: evidence,
		})
	}

	if toBridge {
		evidence := evmFlowTransferEvidence(t, token)
		evidence["hop"], evidence["origin_seed"] = source.hop, source.origin
		evidence["bridge_label"], evidence["bridge_source"] = bridge.Label, bridge.Source
		options := []string{"bridge_operator_notice:" + bridge.Label}
		if token.IssuerOption != "" {
			options = append(options, token.IssuerOption)
		}
		evidence["response_options"] = options
		effects.findings = append(effects.findings, EVMFlowFinding{
			Kind: "evm_flow_bridge_exit", Severity: "critical", Address: t.From,
			Title:     "Followed funds entered a bridge",
			Message:   fmt.Sprintf("%s from %s (hop %d from seed %s) entered %s on %s and may leave the network. Koschei cannot reverse it; send the transaction hash to %s now.", evmFlowAmount(t.Value, token), t.From, source.hop, source.origin, bridge.Label, s.cfg.Network, bridge.Label),
			DedupeKey: s.key("bridge-exit:" + t.EventID()), Evidence: evidence,
		})
		return effects
	}
	if source.hop >= s.cfg.MaxHops || t.To == evmFlowZeroAddress {
		return effects
	}
	target := s.nodes[t.To]
	if target == nil {
		s.enforceBound(1)
		target = newEVMFlowNode()
		target.hop, target.origin = source.hop+1, source.origin
		s.nodes[t.To] = target
	} else if !target.seed && source.hop+1 < target.hop {
		target.hop, target.origin = source.hop+1, source.origin
	}
	target.lastBlock = max(target.lastBlock, t.BlockNumber)
	target.receivedUSD6.Add(target.receivedUSD6, usd)
	holding := target.holdings[t.Token]
	if holding == nil {
		holding = &evmFlowHolding{receivedRaw: new(big.Int)}
		target.holdings[t.Token] = holding
	}
	holding.receivedRaw.Add(holding.receivedRaw, t.Value)
	holding.last = t
	if !target.seed && token.FreezeMethod != "" && !holding.done &&
		evm.NormalizeUSD6(holding.receivedRaw, token.Decimals).Cmp(s.cfg.FreezeCheckUSD6) >= 0 &&
		(holding.lastCheck.IsZero() || s.now().Sub(holding.lastCheck) >= evmFlowRecheckInterval) {
		key := t.To + "|" + t.Token
		if _, queued := s.pending[key]; !queued {
			effects.checks = append(effects.checks, key)
		}
	}
	return effects
}

func splitEVMFlowKey(key string) (string, string) {
	address, token, _ := strings.Cut(key, "|")
	return address, token
}

// applyHolderCheck records a holder observation. A finding is made only when
// the issuer has not frozen the address and its current balance of the token
// is at least the freeze-check threshold. Unreadable states are retried with
// backoff, up to a bounded number of attempts.
func (s *evmFlowState) applyHolderCheck(key string, observed EVMHolderObservation) evmFlowEffects {
	effects := evmFlowEffects{}
	address, tokenAddress := splitEVMFlowKey(key)
	node := s.nodes[address]
	token, known := s.tokens[tokenAddress]
	if node == nil || node.seed || !known || node.holdings[tokenAddress] == nil {
		delete(s.pending, key)
		return effects
	}
	holding := node.holdings[tokenAddress]
	holding.freeze = observed.Freeze
	readable := observed.Freeze == evm.FreezeFrozen || (observed.Freeze == evm.FreezeClear && observed.Balance != nil)
	if !readable {
		retry := s.pending[key]
		retry.attempts++
		if retry.attempts >= evmFlowMaxCheckTries {
			delete(s.pending, key)
			return effects
		}
		retry.next = s.now().Add(evmFlowCheckBaseDelay << (retry.attempts - 1))
		s.pending[key] = retry
		return effects
	}
	delete(s.pending, key)
	holding.lastCheck = s.now()
	holding.balanceRaw = observed.Balance
	if observed.Freeze == evm.FreezeFrozen {
		holding.done = true
		return effects
	}
	if evm.NormalizeUSD6(observed.Balance, token.Decimals).Cmp(s.cfg.FreezeCheckUSD6) < 0 {
		return effects
	}
	holding.done = true
	evidence := evmFlowTransferEvidence(holding.last, token)
	evidence["hop"], evidence["origin_seed"] = node.hop, node.origin
	evidence["followed_received_raw"], evidence["followed_received"] = holding.receivedRaw.String(), evmFlowAmount(holding.receivedRaw, token)
	evidence["current_balance_raw"], evidence["current_balance"] = observed.Balance.String(), evmFlowAmount(observed.Balance, token)
	evidence["balance_source"] = "ERC-20 balanceOf(address) eth_call at the latest block"
	evidence["freeze_source"] = token.FreezeMethod + " eth_call at the latest block"
	evidence["issuer_freeze_state"] = observed.Freeze
	evidence["response_options"] = []string{token.IssuerOption}
	effects.findings = append(effects.findings, EVMFlowFinding{
		Kind: "evm_flow_freeze_candidate", Severity: "high", Address: address,
		Title: fmt.Sprintf("Unfrozen address holds %s after receiving followed funds", token.Symbol),
		Message: fmt.Sprintf("%s received %s descending from seed %s (hop %d) and currently holds %s on %s. %s has not frozen it. Only %s can freeze it; report the address and transaction hashes to %s and law enforcement now.",
			address, evmFlowAmount(holding.receivedRaw, token), node.origin, node.hop, evmFlowAmount(observed.Balance, token), s.cfg.Network, token.Issuer, token.Issuer, token.Issuer),
		DedupeKey: s.key("freeze-candidate:" + address + ":" + token.Symbol), Evidence: evidence,
	})
	return effects
}

func (s *evmFlowState) dueChecks(limit int) []string {
	now := s.now()
	due := []string{}
	for key, retry := range s.pending {
		if !retry.next.After(now) {
			due = append(due, key)
		}
	}
	sort.Strings(due)
	if len(due) > limit {
		due = due[:limit]
	}
	return due
}

type EVMFlowHoldingView struct {
	Token          string `json:"token_contract"`
	Symbol         string `json:"symbol"`
	ReceivedRaw    string `json:"followed_received_raw"`
	Received       string `json:"followed_received"`
	CurrentBalance string `json:"current_balance,omitempty"`
	FreezeState    string `json:"issuer_freeze_state"`
}

type EVMFlowFollowedAddress struct {
	Address           string               `json:"address"`
	Hop               int                  `json:"hop"`
	OriginSeed        string               `json:"origin_seed"`
	Seed              bool                 `json:"seed"`
	ReceivedUSD6      string               `json:"followed_received_usd6"`
	Received          string               `json:"followed_received"`
	Holdings          []EVMFlowHoldingView `json:"holdings"`
	LastActivityBlock uint64               `json:"last_activity_block,string"`
}

type EVMFlowRadarSettings struct {
	MinFollow          string `json:"min_follow"`
	MaxHops            int    `json:"max_hops"`
	BurstCount         int    `json:"burst_count"`
	BurstWindowSeconds int    `json:"burst_window_seconds"`
	FreezeCheck        string `json:"freeze_check"`
	BridgeAlert        string `json:"bridge_alert"`
	PinnedBridges      int    `json:"pinned_bridges"`
	MaxTracked         int    `json:"max_tracked"`
	MaxFindingsPerHour int    `json:"max_findings_per_hour"`
	Confirmations      uint64 `json:"confirmations"`
	MaxBlockRange      uint64 `json:"max_block_range"`
	PollSeconds        int    `json:"poll_seconds"`
	RPCVariable        string `json:"rpc_variable"`
}

func (s *evmFlowState) topFollowed(limit int) []EVMFlowFollowedAddress {
	type ranked struct {
		row      EVMFlowFollowedAddress
		received *big.Int
	}
	all := make([]ranked, 0, len(s.nodes))
	for address, node := range s.nodes {
		row := EVMFlowFollowedAddress{
			Address: address, Hop: node.hop, OriginSeed: node.origin, Seed: node.seed,
			ReceivedUSD6: node.receivedUSD6.String(), Received: evmFlowUSD(node.receivedUSD6), LastActivityBlock: node.lastBlock,
			Holdings: []EVMFlowHoldingView{},
		}
		tokens := make([]string, 0, len(node.holdings))
		for token := range node.holdings {
			tokens = append(tokens, token)
		}
		sort.Strings(tokens)
		for _, tokenAddress := range tokens {
			holding, token := node.holdings[tokenAddress], s.tokens[tokenAddress]
			view := EVMFlowHoldingView{Token: tokenAddress, Symbol: token.Symbol, ReceivedRaw: holding.receivedRaw.String(),
				Received: evmFlowAmount(holding.receivedRaw, token), FreezeState: firstNonEmptyString(holding.freeze, "not_checked")}
			if token.FreezeMethod == "" {
				view.FreezeState = evm.FreezeUnsupported
			}
			if holding.balanceRaw != nil {
				view.CurrentBalance = evmFlowAmount(holding.balanceRaw, token)
			}
			row.Holdings = append(row.Holdings, view)
		}
		all = append(all, ranked{row: row, received: node.receivedUSD6})
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
	out := make([]EVMFlowFollowedAddress, 0, len(all))
	for _, item := range all {
		out = append(out, item.row)
	}
	return out
}

type EVMFlowRadarSnapshot struct {
	Network        string                   `json:"network"`
	Running        bool                     `json:"running"`
	Cursor         uint64                   `json:"cursor,string"`
	Head           uint64                   `json:"head,string"`
	LatestBlock    uint64                   `json:"latest_block,string"`
	BlockRange     uint64                   `json:"block_range"`
	Seeds          int                      `json:"seeds"`
	Followed       int                      `json:"followed_addresses"`
	PendingChecks  int                      `json:"pending_holder_checks"`
	Transfers      uint64                   `json:"transfers_processed,string"`
	Suppressed     int                      `json:"findings_suppressed_this_hour"`
	Tokens         []evm.FlowToken          `json:"tokens"`
	Explorer       string                   `json:"explorer"`
	RecentFindings []EVMFlowFinding         `json:"recent_findings"`
	TopFollowed    []EVMFlowFollowedAddress `json:"top_followed"`
	Settings       EVMFlowRadarSettings     `json:"settings"`
	LastError      string                   `json:"last_error,omitempty"`
	LastCycleAt    *time.Time               `json:"last_cycle_at,omitempty"`
	Storage        string                   `json:"storage"`
}

// EVMFlowRadar owns one network's loop. mu guards state and progress only;
// provider requests and the alert sink always run without holding it.
type EVMFlowRadar struct {
	cfg    EVMFlowRadarConfig
	client evm.FlowClient
	holder EVMHolderChecker
	sink   EVMFlowSink
	health *runtimehealth.Registry
	db     *sql.DB

	mu         sync.Mutex
	state      *evmFlowState
	cursor     uint64
	head       uint64
	latest     uint64
	blockRange uint64
	rangeOK    int
	running    bool
	lastError  string
	lastCycle  time.Time
}

var activeEVMFlowRadars struct {
	sync.RWMutex
	list []*EVMFlowRadar
}

// ActiveEVMFlowRadars returns the radars started in this process, ordered by
// network; empty when disabled or this process runs no background workers.
func ActiveEVMFlowRadars() []*EVMFlowRadar {
	activeEVMFlowRadars.RLock()
	defer activeEVMFlowRadars.RUnlock()
	return append([]*EVMFlowRadar(nil), activeEVMFlowRadars.list...)
}

func registerEVMFlowRadar(r *EVMFlowRadar) {
	activeEVMFlowRadars.Lock()
	defer activeEVMFlowRadars.Unlock()
	for i, existing := range activeEVMFlowRadars.list {
		if existing.cfg.Network == r.cfg.Network {
			activeEVMFlowRadars.list[i] = r
			return
		}
	}
	activeEVMFlowRadars.list = append(activeEVMFlowRadars.list, r)
	sort.Slice(activeEVMFlowRadars.list, func(i, j int) bool {
		return activeEVMFlowRadars.list[i].cfg.Network < activeEVMFlowRadars.list[j].cfg.Network
	})
}

// FindEVMFlowFinding returns the most recent in-memory finding with that
// dedupe key from any running EVM Flow Radar.
func FindEVMFlowFinding(key string) (EVMFlowFinding, bool) {
	for _, radar := range ActiveEVMFlowRadars() {
		if finding, ok := radar.FindingByDedupeKey(key); ok {
			return finding, true
		}
	}
	return EVMFlowFinding{}, false
}

func NewEVMFlowRadar(cfg EVMFlowRadarConfig, db *sql.DB, sink EVMFlowSink, health *runtimehealth.Registry) *EVMFlowRadar {
	r := &EVMFlowRadar{cfg: cfg, sink: sink, health: health, db: db, blockRange: max(uint64(1), cfg.MaxBlockRange),
		client: evm.FlowClient{URL: cfg.Endpoint,
			HTTP: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}}
	r.holder = r.rpcHolderObservation
	r.state = newEVMFlowState(cfg, EVMPinnedBridges(cfg.Network))
	return r
}

func (r *EVMFlowRadar) Network() string { return r.cfg.Network }

func (r *EVMFlowRadar) rpcHolderObservation(ctx context.Context, address string, token evm.FlowToken) EVMHolderObservation {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	observed := EVMHolderObservation{Freeze: r.client.FreezeState(callCtx, token, address)}
	if observed.Freeze == evm.FreezeClear {
		if balance, err := r.client.TokenBalance(callCtx, token.Address, address); err == nil {
			observed.Balance = balance
		}
	}
	return observed
}

func (r *EVMFlowRadar) FindingByDedupeKey(key string) (EVMFlowFinding, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.state.findings) - 1; i >= 0; i-- {
		if r.state.findings[i].DedupeKey == key {
			return r.state.findings[i], true
		}
	}
	return EVMFlowFinding{}, false
}

func (r *EVMFlowRadar) Snapshot() EVMFlowRadarSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	seeds := 0
	for _, node := range r.state.nodes {
		if node.seed {
			seeds++
		}
	}
	findings := append([]EVMFlowFinding(nil), r.state.findings...)
	sort.SliceStable(findings, func(i, j int) bool { return findings[i].ObservedAt.After(findings[j].ObservedAt) })
	snapshot := EVMFlowRadarSnapshot{Network: r.cfg.Network, Running: r.running, Cursor: r.cursor, Head: r.head, LatestBlock: r.latest,
		BlockRange: r.blockRange, Seeds: seeds, Followed: len(r.state.nodes), PendingChecks: len(r.state.pending), Transfers: r.state.transfers,
		Suppressed: r.state.suppressed, Tokens: append([]evm.FlowToken(nil), r.cfg.Tokens...), Explorer: r.cfg.Explorer,
		RecentFindings: findings, TopFollowed: r.state.topFollowed(evmFlowTopFollowed), LastError: r.lastError, Storage: "in_memory_only",
		Settings: EVMFlowRadarSettings{
			MinFollow: evmFlowUSD(r.cfg.MinFollowUSD6), MaxHops: r.cfg.MaxHops, BurstCount: r.cfg.BurstCount,
			BurstWindowSeconds: int(r.cfg.BurstWindow.Seconds()), FreezeCheck: evmFlowUSD(r.cfg.FreezeCheckUSD6),
			BridgeAlert: evmFlowUSD(r.cfg.BridgeAlertUSD6), PinnedBridges: len(r.state.bridges), MaxTracked: r.cfg.MaxTracked,
			MaxFindingsPerHour: r.cfg.MaxFindings, Confirmations: r.cfg.Confirmations, MaxBlockRange: r.cfg.MaxBlockRange,
			PollSeconds: int(r.cfg.Poll.Seconds()), RPCVariable: r.cfg.EndpointEnv,
		},
	}
	if !r.lastCycle.IsZero() {
		at := r.lastCycle.UTC()
		snapshot.LastCycleAt = &at
	}
	return snapshot
}

func (r *EVMFlowRadar) deliver(ctx context.Context, findings []EVMFlowFinding) {
	if len(findings) == 0 {
		return
	}
	admitted := []EVMFlowFinding{}
	r.mu.Lock()
	for _, finding := range findings {
		if accepted, ok := r.state.admit(finding); ok {
			admitted = append(admitted, accepted)
		}
	}
	r.mu.Unlock()
	for _, finding := range admitted {
		log.Printf("evm flow radar finding network=%s kind=%s severity=%s address=%s", finding.Network, finding.Kind, finding.Severity, finding.Address)
		if r.sink != nil {
			r.sink(ctx, finding)
		}
	}
}

func (r *EVMFlowRadar) runHolderChecks(ctx context.Context, keys []string) {
	seen := map[string]struct{}{}
	for _, key := range keys {
		if _, ok := seen[key]; ok || ctx.Err() != nil {
			continue
		}
		seen[key] = struct{}{}
		address, tokenAddress := splitEVMFlowKey(key)
		r.mu.Lock()
		token, known := r.state.tokens[tokenAddress]
		r.mu.Unlock()
		if !known {
			continue
		}
		observed := r.holder(ctx, address, token)
		r.mu.Lock()
		effects := r.state.applyHolderCheck(key, observed)
		r.mu.Unlock()
		r.deliver(ctx, effects.findings)
	}
}

func (r *EVMFlowRadar) ingest(ctx context.Context, transfers []evm.FlowTransfer) {
	all := evmFlowEffects{}
	r.mu.Lock()
	for _, transfer := range transfers {
		effects := r.state.process(transfer)
		all.findings = append(all.findings, effects.findings...)
		all.checks = append(all.checks, effects.checks...)
	}
	r.mu.Unlock()
	r.deliver(ctx, all.findings)
	if len(all.checks) > evmFlowMaxHolderChecks {
		r.mu.Lock()
		for _, key := range all.checks[evmFlowMaxHolderChecks:] {
			if _, queued := r.state.pending[key]; !queued {
				r.state.pending[key] = evmFlowPendingCheck{next: r.state.now()}
			}
		}
		r.mu.Unlock()
		all.checks = all.checks[:evmFlowMaxHolderChecks]
	}
	r.runHolderChecks(ctx, all.checks)
}

func (r *EVMFlowRadar) retryPendingChecks(ctx context.Context) {
	r.mu.Lock()
	due := r.state.dueChecks(evmFlowMaxHolderChecks)
	r.mu.Unlock()
	r.runHolderChecks(ctx, due)
}

func (r *EVMFlowRadar) refreshSeeds(ctx context.Context) {
	seeds := append([]string(nil), r.cfg.Seeds...)
	if r.db != nil {
		rows, err := r.db.QueryContext(ctx, `
			SELECT target FROM watchlist_targets
			WHERE network=$1 AND target_type='wallet' AND status='active'
			ORDER BY created_at DESC LIMIT 500`, r.cfg.Network)
		if err == nil {
			for rows.Next() {
				var target string
				if rows.Scan(&target) == nil {
					if canonical, err := evm.Address(strings.TrimSpace(target)); err == nil {
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

func (r *EVMFlowRadar) tokenAddresses() []string {
	out := make([]string, 0, len(r.cfg.Tokens))
	for _, token := range r.cfg.Tokens {
		out = append(out, strings.ToLower(token.Address))
	}
	return out
}

// cycle reads Transfer logs from the cursor toward the confirmed head in
// bounded block ranges. A range the provider rejects as too large is halved
// and retried; after a run of successes the range grows back. It returns the
// number of blocks processed and whether the cursor reached the head.
func (r *EVMFlowRadar) cycle(ctx context.Context) (int, bool, error) {
	// Validate every cycle, including after an RPC provider failover, before
	// reading the head or advancing the cursor under a misleading chain label.
	expectedChainIDs := map[string]uint64{
		"ethereum-mainnet": 1,
		"base-mainnet":     8453,
		"optimism-mainnet": 10,
		"arbitrum-mainnet": 42161,
		"polygon-mainnet":  137,
		"bnb-mainnet":      56,
	}
	expected, ok := expectedChainIDs[r.cfg.Network]
	if !ok {
		return 0, false, errors.New("evm_flow_unknown_network")
	}
	actual, err := r.client.ChainID(ctx)
	if err != nil {
		return 0, false, err
	}
	if actual != expected {
		return 0, false, errors.New("evm_flow_provider_chain_id_mismatch")
	}
	latest, err := r.client.BlockNumber(ctx)
	if err != nil {
		return 0, false, err
	}
	if latest <= r.cfg.Confirmations {
		return 0, true, errors.New("evm_flow_head_below_confirmations")
	}
	safe := latest - r.cfg.Confirmations
	r.mu.Lock()
	r.latest = latest
	if safe > r.head {
		r.head = safe
	}
	if r.cursor == 0 {
		start := safe
		if r.cfg.Backfill > 0 && safe > r.cfg.Backfill {
			start = safe - r.cfg.Backfill
		}
		r.cursor = start - 1
	}
	// Process only up to this call's confirmed head: a lagging provider
	// backend can answer eth_getLogs with an empty result for blocks it has
	// not seen, so a higher head remembered from an earlier call is not used.
	cursor, head, blockRange := r.cursor, safe, r.blockRange
	r.mu.Unlock()

	tokens := r.tokenAddresses()
	processed, attempts := 0, 0
	for cursor < head && attempts < r.cfg.MaxRangesPerCycle && ctx.Err() == nil {
		attempts++
		from := cursor + 1
		to := min(head, cursor+blockRange)
		transfers, err := r.client.TransferLogs(ctx, from, to, tokens)
		if errors.Is(err, evm.ErrLogRangeTooLarge) && blockRange > 1 {
			blockRange = max(uint64(1), blockRange/2)
			r.mu.Lock()
			r.blockRange, r.rangeOK = blockRange, 0
			r.mu.Unlock()
			continue
		}
		if err != nil {
			return processed, false, err
		}
		r.ingest(ctx, transfers)
		r.mu.Lock()
		r.cursor = to
		r.rangeOK++
		if r.rangeOK >= evmFlowRangeGrowAfter && r.blockRange < r.cfg.MaxBlockRange {
			r.blockRange, r.rangeOK = min(r.cfg.MaxBlockRange, r.blockRange*2), 0
		}
		blockRange = r.blockRange
		r.mu.Unlock()
		processed += int(to - from + 1)
		cursor = to
	}
	r.retryPendingChecks(ctx)
	return processed, cursor >= head, nil
}

// Start runs the radar until ctx ends.
func (r *EVMFlowRadar) Start(ctx context.Context) func() {
	id := EVMFlowRadarHealthID(r.cfg.Network)
	if r.health != nil {
		r.health.Register(id, "worker", r.cfg.Network, true)
	}
	runCtx, cancel := context.WithCancel(ctx)
	registerEVMFlowRadar(r)
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
			if time.Since(lastSeedRefresh) >= evmFlowSeedRefresh {
				r.refreshSeeds(runCtx)
				lastSeedRefresh = time.Now()
			}
			processed, caughtUp, err := r.cycle(runCtx)
			r.mu.Lock()
			r.lastCycle, r.lastError = time.Now(), ""
			if err != nil {
				r.lastError = err.Error() // error categories only; never URLs or keys
			}
			head, cursor := r.head, r.cursor
			r.mu.Unlock()
			if r.health != nil {
				r.health.RecordIngestProgress(id, &head, &cursor, err != nil, false)
				if err != nil {
					r.health.Failure(id, err)
				} else {
					r.health.Success(id, processed)
				}
			}
			wait := r.cfg.Poll
			if err != nil {
				log.Printf("evm flow radar %s cycle failed: %v", r.cfg.Network, err)
				backoff = min(max(2*backoff, 5*time.Second), 5*time.Minute)
				wait = backoff
			} else {
				backoff = 0
				if !caughtUp {
					wait = evmFlowCatchUpWait
				}
			}
			select {
			case <-runCtx.Done():
			case <-time.After(wait):
			}
		}
	}()
	symbols := []string{}
	for _, token := range r.cfg.Tokens {
		symbols = append(symbols, token.Symbol)
	}
	log.Printf("evm flow radar started: network=%s mode=observe_only storage=in_memory tokens=%s seeds=%d max_hops=%d confirmations=%d block_range=%d poll=%s",
		r.cfg.Network, strings.Join(symbols, "+"), len(r.cfg.Seeds), r.cfg.MaxHops, r.cfg.Confirmations, r.cfg.MaxBlockRange, r.cfg.Poll)
	return cancel
}
