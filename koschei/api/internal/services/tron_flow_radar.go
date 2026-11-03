package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
// Storage boundary: nothing per block, per transfer or per followed address
// is persisted. Followed addresses live in a bounded in-memory map and are
// lost on restart. Only findings an operator can act on leave the process,
// through the injected sink (security alerts upsert on a dedupe key, so a
// repeated finding updates one row instead of adding rows), and findings are
// capped per hour.
//
// Truth boundary: a followed address is "received funds from a seed within N
// hops", not "controlled by the attacker". Koschei cannot stop or reverse a
// transfer; findings name who can hold funds (Tether for USDT, the bridge
// operator, the receiving party). The radar is observe-only and off by default.

const (
	TronFlowRadarHealthID = "worker.tron-flow-radar"

	tronFlowUSDTContract   = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
	tronFlowTransferTopic  = "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	tronFlowMaxBlockBytes  = 16 << 20
	tronFlowMaxHeadBytes   = 1 << 20
	tronFlowSeedRefresh    = 5 * time.Minute
	tronFlowRecentFindings = 200
	tronFlowUSDTDecimals   = 6
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

// TronBase58 renders a 41-prefixed hex address in wallet Base58Check form, or
// "" when the input is not a valid Tron address.
func TronBase58(hexAddress string) string {
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

// ParseTronUSDTFreezeWord maps an isBlackListed(address) constant-call
// response to frozen / not_frozen / unknown. It never guesses not_frozen.
func ParseTronUSDTFreezeWord(raw []byte) string {
	var payload struct {
		Result struct {
			Result bool `json:"result"`
		} `json:"result"`
		ConstantResult []string `json:"constant_result"`
	}
	if json.Unmarshal(raw, &payload) != nil || !payload.Result.Result || len(payload.ConstantResult) != 1 {
		return "unknown"
	}
	switch strings.ToLower(payload.ConstantResult[0]) {
	case strings.Repeat("0", 64):
		return "not_frozen"
	case strings.Repeat("0", 63) + "1":
		return "frozen"
	default:
		return "unknown"
	}
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
	return new(big.Int).Mul(big.NewInt(int64(whole)), new(big.Int).Exp(big.NewInt(10), big.NewInt(tronFlowUSDTDecimals), nil))
}

// TronFlowRadarEnabled is the observe-only opt-in; the radar never starts by default.
func TronFlowRadarEnabled() bool {
	return strings.TrimSpace(os.Getenv("KOSCHEI_TRON_FLOW_RADAR_ENABLED")) == "1"
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

type TronFlowTransfer struct {
	TransactionID    string
	BlockNumber      uint64
	BlockTimestampMS int64
	From             string // 41-hex
	To               string // 41-hex
	ValueRaw         *big.Int
}

// ParseTronBlockUSDTTransfers extracts successful USDT Transfer events from a
// gettransactioninfobyblocknum response. Malformed entries are skipped.
func ParseTronBlockUSDTTransfers(raw []byte) ([]TronFlowTransfer, error) {
	var infos []struct {
		ID          string `json:"id"`
		BlockNumber uint64 `json:"blockNumber"`
		Timestamp   int64  `json:"blockTimeStamp"`
		Result      string `json:"result"`
		Receipt     struct {
			Result string `json:"result"`
		} `json:"receipt"`
		Log []struct {
			Address string   `json:"address"`
			Topics  []string `json:"topics"`
			Data    string   `json:"data"`
		} `json:"log"`
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "{}" {
		return []TronFlowTransfer{}, nil // empty block
	}
	if err := json.Unmarshal(trimmed, &infos); err != nil {
		return nil, errors.New("tron_block_info_invalid")
	}
	usdt, _ := tron.Address(tronFlowUSDTContract)
	out := []TronFlowTransfer{}
	for _, info := range infos {
		if strings.EqualFold(info.Result, "FAILED") || (info.Receipt.Result != "" && info.Receipt.Result != "SUCCESS") {
			continue
		}
		id, err := hex.DecodeString(info.ID)
		if err != nil || len(id) != 32 {
			continue
		}
		for _, entry := range info.Log {
			if len(entry.Topics) != 3 || strings.ToLower(entry.Topics[0]) != tronFlowTransferTopic || "41"+strings.ToLower(entry.Address) != usdt {
				continue
			}
			from, okFrom := tronFlowTopicAddress(entry.Topics[1])
			to, okTo := tronFlowTopicAddress(entry.Topics[2])
			data, err := hex.DecodeString(entry.Data)
			if !okFrom || !okTo || err != nil || len(data) != 32 {
				continue
			}
			out = append(out, TronFlowTransfer{
				TransactionID: strings.ToLower(info.ID), BlockNumber: info.BlockNumber, BlockTimestampMS: info.Timestamp,
				From: from, To: to, ValueRaw: new(big.Int).SetBytes(data),
			})
		}
	}
	return out, nil
}

func tronFlowTopicAddress(topic string) (string, bool) {
	topic = strings.ToLower(topic)
	if len(topic) != 64 || topic[:24] != strings.Repeat("0", 24) {
		return "", false
	}
	if _, err := hex.DecodeString(topic[24:]); err != nil {
		return "", false
	}
	return "41" + topic[24:], true
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

// TronFreezeChecker returns frozen / not_frozen / unknown for an address.
type TronFreezeChecker func(context.Context, string) string

type tronFlowNode struct {
	hop          int
	origin       string
	seed         bool
	taintedRaw   *big.Int
	lastSeenMS   int64
	outTimesMS   []int64
	burstUntilMS int64
	freezeDone   bool
	freezeState  string
}

// tronFlowState is the in-memory graph. It is not safe for concurrent use;
// the radar loop owns it and Snapshot copies under the radar mutex.
type tronFlowState struct {
	cfg        TronFlowRadarConfig
	bridges    map[string]TronBridge
	nodes      map[string]*tronFlowNode
	freeze     TronFreezeChecker
	findings   []TronFlowFinding
	hourStart  time.Time
	hourCount  int
	suppressed int
	transfers  uint64
	now        func() time.Time
}

func newTronFlowState(cfg TronFlowRadarConfig, bridges map[string]TronBridge, freeze TronFreezeChecker) *tronFlowState {
	s := &tronFlowState{cfg: cfg, bridges: bridges, nodes: map[string]*tronFlowNode{}, freeze: freeze, now: time.Now}
	s.setSeeds(cfg.Seeds)
	return s
}

// setSeeds marks seed wallets as hop 0. A former seed stays followed as an
// ordinary node and can be evicted.
func (s *tronFlowState) setSeeds(seeds []string) {
	current := map[string]struct{}{}
	for _, seed := range seeds {
		current[seed] = struct{}{}
		node := s.nodes[seed]
		if node == nil {
			node = &tronFlowNode{taintedRaw: new(big.Int)}
			s.nodes[seed] = node
		}
		node.hop, node.origin, node.seed = 0, seed, true
	}
	for address, node := range s.nodes {
		if _, ok := current[address]; !ok && node.seed {
			node.seed = false
		}
	}
}

func tronFlowAmount(value *big.Int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(tronFlowUSDTDecimals), nil)
	whole, fraction := new(big.Int).QuoRem(value, scale, new(big.Int))
	frac := strings.TrimRight(fmt.Sprintf("%0*s", tronFlowUSDTDecimals, fraction.String()), "0")
	if frac == "" {
		return whole.String() + " USDT"
	}
	return whole.String() + "." + frac + " USDT"
}

func tronFlowDisplay(hexAddress string) string {
	if encoded := TronBase58(hexAddress); encoded != "" {
		return encoded
	}
	return hexAddress
}

func (s *tronFlowState) emit(ctx context.Context, sink TronFlowSink, finding TronFlowFinding) {
	now := s.now()
	if now.Sub(s.hourStart) >= time.Hour {
		if s.suppressed > 0 {
			log.Printf("tron flow radar: %d finding(s) suppressed by the hourly cap", s.suppressed)
		}
		s.hourStart, s.hourCount, s.suppressed = now, 0, 0
	}
	if s.hourCount >= s.cfg.MaxFindings {
		s.suppressed++
		return
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
	log.Printf("tron flow radar finding kind=%s severity=%s address=%s", finding.Kind, finding.Severity, finding.Address)
	if sink != nil {
		sink(ctx, finding)
	}
}

func tronFlowTransferEvidence(t TronFlowTransfer) map[string]any {
	return map[string]any{
		"transaction_id": t.TransactionID, "block_number": t.BlockNumber, "block_timestamp_ms": t.BlockTimestampMS,
		"from": tronFlowDisplay(t.From), "to": tronFlowDisplay(t.To), "value_raw": t.ValueRaw.String(), "amount": tronFlowAmount(t.ValueRaw),
		"token_contract": tronFlowUSDTContract,
	}
}

func (s *tronFlowState) process(ctx context.Context, sink TronFlowSink, t TronFlowTransfer) {
	s.transfers++
	bridge, toBridge := s.bridges[t.To]
	source := s.nodes[t.From]

	if source == nil {
		// Network-wide signal: a very large USDT deposit into a pinned bridge,
		// even when it does not descend from a seed.
		if toBridge && t.ValueRaw.Cmp(s.cfg.BridgeAlertRaw) >= 0 {
			evidence := tronFlowTransferEvidence(t)
			evidence["bridge_label"], evidence["bridge_source"] = bridge.Label, bridge.Source
			evidence["response_options"] = []string{"bridge_operator_notice:" + bridge.Label}
			s.emit(ctx, sink, TronFlowFinding{
				Kind: "tron_large_bridge_inflow", Severity: "info", Address: tronFlowDisplay(t.From),
				Title:     "Large USDT deposit into a pinned bridge",
				Message:   fmt.Sprintf("%s moved from %s into %s. Not linked to a seed wallet.", tronFlowAmount(t.ValueRaw), tronFlowDisplay(t.From), bridge.Label),
				DedupeKey: "tron-flow:bridge-inflow:" + t.TransactionID, Evidence: evidence,
			})
		}
		return
	}
	if t.ValueRaw.Cmp(s.cfg.MinFollowRaw) < 0 {
		return
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
		s.emit(ctx, sink, TronFlowFinding{
			Kind: "tron_flow_seed_outflow", Severity: "high", Address: tronFlowDisplay(t.From),
			Title:     "Seed wallet started moving USDT",
			Message:   fmt.Sprintf("%s sent %s to %s. Every recipient is now followed up to %d hops across the network.", tronFlowDisplay(t.From), tronFlowAmount(t.ValueRaw), tronFlowDisplay(t.To), s.cfg.MaxHops),
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
		s.emit(ctx, sink, TronFlowFinding{
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
		s.emit(ctx, sink, TronFlowFinding{
			Kind: "tron_flow_bridge_exit", Severity: "critical", Address: tronFlowDisplay(t.From),
			Title:     "Followed funds entered a bridge",
			Message:   fmt.Sprintf("%s from %s (hop %d from seed %s) entered %s and may leave Tron. Koschei cannot reverse it; send the transaction ID to %s now.", tronFlowAmount(t.ValueRaw), tronFlowDisplay(t.From), source.hop, tronFlowDisplay(source.origin), bridge.Label, bridge.Label),
			DedupeKey: "tron-flow:bridge-exit:" + t.TransactionID, Evidence: evidence,
		})
		return
	}
	if source.hop >= s.cfg.MaxHops {
		return
	}
	target := s.nodes[t.To]
	if target == nil {
		s.evictIfFull()
		target = &tronFlowNode{hop: source.hop + 1, origin: source.origin, taintedRaw: new(big.Int)}
		s.nodes[t.To] = target
	} else if !target.seed && source.hop+1 < target.hop {
		target.hop, target.origin = source.hop+1, source.origin
	}
	target.lastSeenMS = t.BlockTimestampMS
	target.taintedRaw.Add(target.taintedRaw, t.ValueRaw)
	if !target.seed && !target.freezeDone && target.taintedRaw.Cmp(s.cfg.FreezeCheckRaw) >= 0 && s.freeze != nil {
		state := s.freeze(ctx, t.To)
		target.freezeState = state
		if state == "frozen" || state == "not_frozen" {
			target.freezeDone = true
		}
		if state == "not_frozen" {
			evidence := tronFlowTransferEvidence(t)
			evidence["hop"], evidence["origin_seed"] = target.hop, tronFlowDisplay(target.origin)
			evidence["followed_received_raw"], evidence["followed_received"] = target.taintedRaw.String(), tronFlowAmount(target.taintedRaw)
			evidence["usdt_freeze_state"] = state
			evidence["response_options"] = []string{"issuer_freeze_request:tether_usdt"}
			s.emit(ctx, sink, TronFlowFinding{
				Kind: "tron_flow_freeze_candidate", Severity: "high", Address: tronFlowDisplay(t.To),
				Title:     "Unfrozen address holds followed USDT",
				Message:   fmt.Sprintf("%s has received %s descending from seed %s (hop %d) and Tether has not frozen it. Only Tether can freeze it; report the address and transaction IDs to Tether and law enforcement now.", tronFlowDisplay(t.To), tronFlowAmount(target.taintedRaw), tronFlowDisplay(target.origin), target.hop),
				DedupeKey: "tron-flow:freeze-candidate:" + t.To, Evidence: evidence,
			})
		}
	}
}

// evictIfFull drops the least recently active non-seed node when the
// in-memory graph is at its bound.
func (s *tronFlowState) evictIfFull() {
	if len(s.nodes) < s.cfg.MaxTracked {
		return
	}
	oldest, oldestMS := "", int64(1<<62)
	for address, node := range s.nodes {
		if !node.seed && node.lastSeenMS < oldestMS {
			oldest, oldestMS = address, node.lastSeenMS
		}
	}
	if oldest != "" {
		delete(s.nodes, oldest)
	}
}

type TronFlowFollowedAddress struct {
	Address         string `json:"address"`
	Hop             int    `json:"hop"`
	OriginSeed      string `json:"origin_seed"`
	Seed            bool   `json:"seed"`
	ReceivedRaw     string `json:"followed_received_raw"`
	Received        string `json:"followed_received"`
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

const tronFlowTopFollowed = 25

// topFollowed lists seeds first, then followed addresses holding the most
// followed USDT.
func (s *tronFlowState) topFollowed(limit int) []TronFlowFollowedAddress {
	out := make([]TronFlowFollowedAddress, 0, len(s.nodes))
	for address, node := range s.nodes {
		out = append(out, TronFlowFollowedAddress{
			Address: tronFlowDisplay(address), Hop: node.hop, OriginSeed: tronFlowDisplay(node.origin), Seed: node.seed,
			ReceivedRaw: node.taintedRaw.String(), Received: tronFlowAmount(node.taintedRaw), LastActivityMS: node.lastSeenMS,
			USDTFreezeState: firstNonEmptyString(node.freezeState, "not_checked"),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Seed != out[j].Seed {
			return out[i].Seed
		}
		left, _ := new(big.Int).SetString(out[i].ReceivedRaw, 10)
		right, _ := new(big.Int).SetString(out[j].ReceivedRaw, 10)
		if cmp := left.Cmp(right); cmp != 0 {
			return cmp > 0
		}
		return out[i].Address < out[j].Address
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

type TronFlowRadarSnapshot struct {
	Running        bool                      `json:"running"`
	Cursor         uint64                    `json:"cursor,string"`
	Head           uint64                    `json:"head,string"`
	Seeds          int                       `json:"seeds"`
	Followed       int                       `json:"followed_addresses"`
	Transfers      uint64                    `json:"usdt_transfers_processed,string"`
	Suppressed     int                       `json:"findings_suppressed_this_hour"`
	RecentFindings []TronFlowFinding         `json:"recent_findings"`
	TopFollowed    []TronFlowFollowedAddress `json:"top_followed"`
	Settings       TronFlowRadarSettings     `json:"settings"`
	LastError      string                    `json:"last_error,omitempty"`
	LastCycleAt    *time.Time                `json:"last_cycle_at,omitempty"`
	Storage        string                    `json:"storage"`
}

type TronFlowRadar struct {
	cfg    TronFlowRadarConfig
	client *http.Client
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
		client: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	r.state = newTronFlowState(cfg, TronPinnedBridges(), r.usdtFreezeState)
	return r
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
		Transfers: r.state.transfers, Suppressed: r.state.suppressed, RecentFindings: findings,
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

// post never returns the URL, API key or upstream body in errors.
func (r *TronFlowRadar) post(ctx context.Context, path string, body any, limit int64) ([]byte, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, errors.New("tron_request_encoding_failed")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(r.cfg.Endpoint, "/")+path, bytes.NewReader(encoded))
	if err != nil {
		return nil, errors.New("tron_request_invalid")
	}
	req.Header.Set("Content-Type", "application/json")
	if r.cfg.APIKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", r.cfg.APIKey)
	}
	res, err := r.client.Do(req)
	if err != nil {
		return nil, errors.New("tron_provider_unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tron_provider_http_%d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, errors.New("tron_provider_response_too_large")
	}
	return raw, nil
}

func (r *TronFlowRadar) solidHead(ctx context.Context) (uint64, error) {
	raw, err := r.post(ctx, "/walletsolidity/getnowblock", map[string]any{}, tronFlowMaxHeadBytes)
	if err != nil {
		return 0, err
	}
	var block struct {
		Header struct {
			Raw struct {
				Number uint64 `json:"number"`
			} `json:"raw_data"`
		} `json:"block_header"`
	}
	if json.Unmarshal(raw, &block) != nil || block.Header.Raw.Number == 0 {
		return 0, errors.New("tron_head_invalid")
	}
	return block.Header.Raw.Number, nil
}

func (r *TronFlowRadar) usdtFreezeState(ctx context.Context, hexAddress string) string {
	contract, err := tron.Address(tronFlowUSDTContract)
	if err != nil || len(hexAddress) != 42 {
		return "unknown"
	}
	callCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	raw, err := r.post(callCtx, "/wallet/triggerconstantcontract", map[string]any{
		"owner_address": hexAddress, "contract_address": contract, "function_selector": "isBlackListed(address)",
		"parameter": strings.Repeat("0", 24) + hexAddress[2:], "visible": false,
	}, tronFlowMaxHeadBytes)
	if err != nil {
		return "unknown"
	}
	return ParseTronUSDTFreezeWord(raw)
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
// per cycle. It returns the number of blocks processed.
func (r *TronFlowRadar) cycle(ctx context.Context) (int, error) {
	head, err := r.solidHead(ctx)
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
		raw, err := r.post(ctx, "/walletsolidity/gettransactioninfobyblocknum", map[string]any{"num": next}, tronFlowMaxBlockBytes)
		if err != nil {
			return processed, err
		}
		transfers, err := ParseTronBlockUSDTTransfers(raw)
		if err != nil {
			return processed, err
		}
		r.mu.Lock()
		for _, transfer := range transfers {
			r.state.process(ctx, r.sink, transfer)
		}
		r.cursor = next
		r.mu.Unlock()
		cursor = next
		processed++
	}
	return processed, nil
}

// Start runs the radar until ctx ends. It is a no-op unless enabled and keyed.
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
				backoff = min(max(2*backoff, 5*time.Second), 5*time.Minute)
				wait = backoff
			} else {
				backoff = 0
				if processed >= r.cfg.MaxBlocks {
					wait = 0 // still catching up
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
