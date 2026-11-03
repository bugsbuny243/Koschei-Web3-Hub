package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"koschei/api/internal/radaradapter/tron"
)

// Explicit synthetic fixtures: addresses and transaction IDs are repeated
// bytes and do not refer to real incident wallets.
func flowAddr(fill byte) string { return "41" + strings.Repeat(fmt.Sprintf("%02x", fill), 20) }
func flowTx(fill byte) string   { return strings.Repeat(fmt.Sprintf("%02x", fill), 32) }
func flowUSDT(whole int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(whole), big.NewInt(1_000_000))
}

func flowConfig(seeds ...string) TronFlowRadarConfig {
	return TronFlowRadarConfig{
		Seeds: seeds, MinFollowRaw: flowUSDT(10_000), BridgeAlertRaw: flowUSDT(1_000_000), FreezeCheckRaw: flowUSDT(1_000_000),
		MaxHops: 3, BurstCount: 3, BurstWindow: 10 * time.Minute, MaxTracked: 1000, MaxFindings: 100, Poll: time.Second, MaxBlocks: 20,
	}
}

type flowCollector struct{ findings []TronFlowFinding }

func (c *flowCollector) sink(_ context.Context, f TronFlowFinding) {
	c.findings = append(c.findings, f)
}
func (c *flowCollector) kinds() []string {
	out := []string{}
	for _, f := range c.findings {
		out = append(out, f.Kind+":"+f.Severity)
	}
	return out
}

func flowTransfer(tx byte, from, to string, usdt int64, ts int64) TronFlowTransfer {
	return TronFlowTransfer{TransactionID: flowTx(tx), BlockNumber: 1, BlockTimestampMS: ts, From: from, To: to, ValueRaw: flowUSDT(usdt)}
}

func usdtLog(from, to string, raw *big.Int) string {
	usdt, _ := tron.Address(tronFlowUSDTContract)
	return fmt.Sprintf(`{"address":"%s","topics":["%s","%s%s","%s%s"],"data":"%064x"}`,
		usdt[2:], tronFlowTransferTopic, strings.Repeat("0", 24), from[2:], strings.Repeat("0", 24), to[2:], raw)
}

func TestParseTronBlockUSDTTransfersKeepsOnlySuccessfulUSDTEvents(t *testing.T) {
	a, b := flowAddr(0x11), flowAddr(0x22)
	raw := fmt.Sprintf(`[
		{"id":"%s","blockNumber":7,"blockTimeStamp":1000,"receipt":{"result":"SUCCESS"},"log":[%s,
			{"address":"%s","topics":["%s","%s%s","%s%s"],"data":"%064x"}]},
		{"id":"%s","blockNumber":7,"blockTimeStamp":1000,"result":"FAILED","receipt":{"result":"REVERT"},"log":[%s]},
		{"id":"not-hex","blockNumber":7,"blockTimeStamp":1000,"log":[%s]},
		{"id":"%s","blockNumber":7,"blockTimeStamp":1000,"log":[{"address":"a614f803b6fd780986a42c78ec9c7f77e6ded13c","topics":["%s","bad","bad"],"data":"00"}]}
	]`,
		flowTx(1), usdtLog(a, b, flowUSDT(612_700_000)),
		strings.Repeat("ab", 20), tronFlowTransferTopic, strings.Repeat("0", 24), a[2:], strings.Repeat("0", 24), b[2:], flowUSDT(5), // other token
		flowTx(2), usdtLog(a, b, flowUSDT(1)),
		usdtLog(a, b, flowUSDT(1)),
		flowTx(4), tronFlowTransferTopic,
	)
	transfers, err := ParseTronBlockUSDTTransfers([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 || transfers[0].From != a || transfers[0].To != b || transfers[0].ValueRaw.Cmp(flowUSDT(612_700_000)) != 0 || transfers[0].BlockNumber != 7 {
		t.Fatalf("unexpected transfers %+v", transfers)
	}
	if empty, err := ParseTronBlockUSDTTransfers([]byte(`{}`)); err != nil || len(empty) != 0 {
		t.Fatalf("an empty block must parse as no transfers, got %d err=%v", len(empty), err)
	}
	if _, err := ParseTronBlockUSDTTransfers([]byte(`{"Error":"x"}`)); err == nil {
		t.Fatal("a provider error object must not read as an empty block")
	}
}

// Mirrors the incident shape: seed -> hop 1 -> hop 2 -> bridge.
func TestTronFlowFollowsSeedFundsHopByHopToABridge(t *testing.T) {
	seed, hop1, hop2, hop3, bridge := flowAddr(0x11), flowAddr(0x22), flowAddr(0x33), flowAddr(0x44), flowAddr(0x55)
	collector := &flowCollector{}
	cfg := flowConfig(seed)
	cfg.MaxHops = 2
	state := newTronFlowState(cfg, map[string]TronBridge{bridge: {Label: "Fixture Bridge", Source: "https://example.invalid"}}, nil)
	ctx := context.Background()

	state.process(ctx, collector.sink, flowTransfer(1, seed, hop1, 9_999, 1000))
	if len(state.nodes) != 1 || len(collector.findings) != 0 {
		t.Fatal("a transfer below the follow threshold must not be followed or reported")
	}
	state.process(ctx, collector.sink, flowTransfer(2, seed, hop1, 500_000, 1001))
	state.process(ctx, collector.sink, flowTransfer(3, hop1, hop2, 400_000, 1002))
	state.process(ctx, collector.sink, flowTransfer(4, hop2, hop3, 100_000, 1003)) // hop2 == MaxHops: not followed further
	state.process(ctx, collector.sink, flowTransfer(5, hop2, bridge, 300_000, 1004))

	if state.nodes[hop1] == nil || state.nodes[hop1].hop != 1 || state.nodes[hop2] == nil || state.nodes[hop2].hop != 2 {
		t.Fatal("recipients must be followed with their hop distance")
	}
	if state.nodes[hop3] != nil {
		t.Fatal("funds must not be followed beyond the hop limit")
	}
	if got := strings.Join(collector.kinds(), ","); got != "tron_flow_seed_outflow:high,tron_flow_bridge_exit:critical" {
		t.Fatalf("findings = %s", got)
	}
	exit := collector.findings[1]
	if exit.Evidence["hop"] != 2 || exit.Evidence["origin_seed"] != TronBase58(seed) || exit.Evidence["koschei_can_reverse_transfer"] != false ||
		exit.Evidence["identity_or_intent_claim"] != false || exit.Evidence["follow_state"] != "in_memory_only" {
		t.Fatalf("bridge exit evidence = %#v", exit.Evidence)
	}
	if !strings.Contains(exit.Message, "300000 USDT") || !strings.Contains(exit.Message, "Fixture Bridge") || exit.DedupeKey != "tron-flow:bridge-exit:"+flowTx(5) {
		t.Fatalf("bridge exit = %q / %q", exit.Message, exit.DedupeKey)
	}
}

func TestTronFlowReportsOneFanOutPerWindow(t *testing.T) {
	seed := flowAddr(0x11)
	collector := &flowCollector{}
	state := newTronFlowState(flowConfig(seed), nil, nil)
	for i := 0; i < 6; i++ {
		state.process(context.Background(), collector.sink, flowTransfer(byte(10+i), seed, flowAddr(byte(0x60+i)), 20_000, int64(1000+i)))
	}
	if got := strings.Join(collector.kinds(), ","); got != "tron_flow_seed_outflow:high,tron_flow_fan_out:high" {
		t.Fatalf("findings = %s", got)
	}
	// After the window, a new burst is reported again.
	later := int64(1000) + (11 * time.Minute).Milliseconds()
	for i := 0; i < 3; i++ {
		state.process(context.Background(), collector.sink, flowTransfer(byte(30+i), seed, flowAddr(byte(0x70+i)), 20_000, later+int64(i)))
	}
	if got := collector.kinds(); len(got) != 4 || got[3] != "tron_flow_fan_out:high" {
		t.Fatalf("a later burst must be reported, got %v", got)
	}
}

func TestTronFlowFreezeCandidateOnlyForReadableUnfrozenState(t *testing.T) {
	seed, holder, frozenHolder := flowAddr(0x11), flowAddr(0x22), flowAddr(0x33)
	calls := map[string]int{}
	states := map[string][]string{holder: {"unknown", "not_frozen"}, frozenHolder: {"frozen"}}
	checker := func(_ context.Context, address string) string {
		calls[address]++
		queue := states[address]
		state := queue[0]
		if len(queue) > 1 {
			states[address] = queue[1:]
		}
		return state
	}
	collector := &flowCollector{}
	state := newTronFlowState(flowConfig(seed), nil, checker)
	ctx := context.Background()
	state.process(ctx, collector.sink, flowTransfer(1, seed, holder, 600_000, 1000))
	if calls[holder] != 0 {
		t.Fatal("freeze state must not be read below the freeze-check threshold")
	}
	state.process(ctx, collector.sink, flowTransfer(2, seed, holder, 600_000, 1001)) // cumulative 1.2M, unknown
	state.process(ctx, collector.sink, flowTransfer(3, seed, holder, 10_000, 1002))  // retried, not_frozen
	state.process(ctx, collector.sink, flowTransfer(4, seed, holder, 10_000, 1003))  // already decided
	state.process(ctx, collector.sink, flowTransfer(5, seed, frozenHolder, 2_000_000, 1004))
	if calls[holder] != 2 || calls[frozenHolder] != 1 {
		t.Fatalf("freeze calls = %v", calls)
	}
	candidates := []TronFlowFinding{}
	for _, f := range collector.findings {
		if f.Kind == "tron_flow_freeze_candidate" {
			candidates = append(candidates, f)
		}
	}
	if len(candidates) != 1 || candidates[0].Address != TronBase58(holder) || candidates[0].Evidence["followed_received"] != "1210000 USDT" {
		t.Fatalf("freeze candidates = %+v", candidates)
	}
}

func TestTronFlowMemoryIsBoundedAndSeedsAreNeverEvicted(t *testing.T) {
	seed := flowAddr(0x11)
	cfg := flowConfig(seed)
	cfg.MaxTracked, cfg.BurstCount = 5, 1000
	state := newTronFlowState(cfg, nil, nil)
	for i := 0; i < 50; i++ {
		state.process(context.Background(), nil, flowTransfer(byte(i), seed, flowAddr(byte(0x80+i)), 20_000, int64(1000+i)))
	}
	if len(state.nodes) > cfg.MaxTracked {
		t.Fatalf("followed graph grew to %d nodes, bound %d", len(state.nodes), cfg.MaxTracked)
	}
	if node := state.nodes[seed]; node == nil || !node.seed {
		t.Fatal("a seed must never be evicted")
	}
}

func TestTronFlowFindingsAreCappedPerHour(t *testing.T) {
	seedA, seedB, seedC := flowAddr(0x11), flowAddr(0x12), flowAddr(0x13)
	cfg := flowConfig(seedA, seedB, seedC)
	cfg.MaxFindings = 2
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	collector := &flowCollector{}
	state := newTronFlowState(cfg, nil, nil)
	state.now = func() time.Time { return now }
	for i, seed := range []string{seedA, seedB, seedC} {
		state.process(context.Background(), collector.sink, flowTransfer(byte(i+1), seed, flowAddr(byte(0x90+i)), 20_000, 1000))
	}
	if len(collector.findings) != 2 || state.suppressed != 1 {
		t.Fatalf("findings=%d suppressed=%d", len(collector.findings), state.suppressed)
	}
	now = now.Add(time.Hour)
	state.process(context.Background(), collector.sink, flowTransfer(9, flowAddr(0x90), flowAddr(0xa0), 20_000, 2000))
	state.nodes[flowAddr(0x13)] = &tronFlowNode{seed: true, origin: flowAddr(0x13), taintedRaw: new(big.Int)}
	state.process(context.Background(), collector.sink, flowTransfer(10, flowAddr(0x13), flowAddr(0xa1), 20_000, 2001))
	if len(collector.findings) != 3 || state.suppressed != 0 {
		t.Fatalf("the cap must reset after an hour: findings=%d suppressed=%d", len(collector.findings), state.suppressed)
	}
}

func TestTronFlowNetworkWideLargeBridgeInflow(t *testing.T) {
	stranger, bridge := flowAddr(0x21), flowAddr(0x55)
	collector := &flowCollector{}
	state := newTronFlowState(flowConfig(), map[string]TronBridge{bridge: {Label: "Fixture Bridge", Source: "https://example.invalid"}}, nil)
	state.process(context.Background(), collector.sink, flowTransfer(1, stranger, bridge, 999_999, 1000))
	state.process(context.Background(), collector.sink, flowTransfer(2, stranger, bridge, 1_000_000, 1001))
	if got := strings.Join(collector.kinds(), ","); got != "tron_large_bridge_inflow:info" {
		t.Fatalf("findings = %s", got)
	}
	if len(state.nodes) != 0 {
		t.Fatal("an unlinked sender must not be followed")
	}
}

func TestLoadTronFlowRadarConfigBoundsAndSeeds(t *testing.T) {
	t.Setenv("KOSCHEI_TRON_FLOW_RADAR_RPC_URL", "")
	t.Setenv("KOSCHEI_TRON_FLOW_RADAR_SEEDS", tronFlowUSDTContract+", not-an-address ,"+flowAddr(0x11))
	t.Setenv("KOSCHEI_TRON_FLOW_RADAR_MAX_HOPS", "99")
	t.Setenv("KOSCHEI_TRON_FLOW_RADAR_MAX_TRACKED", "1")
	t.Setenv("KOSCHEI_TRON_FLOW_RADAR_MIN_FOLLOW_USDT", "abc")
	cfg := LoadTronFlowRadarConfigFromEnv()
	usdt, _ := tron.Address(tronFlowUSDTContract)
	if cfg.Endpoint != "https://api.trongrid.io" || len(cfg.Seeds) != 2 || cfg.Seeds[0] != usdt || cfg.Seeds[1] != flowAddr(0x11) {
		t.Fatalf("unexpected endpoint/seeds %q %v", cfg.Endpoint, cfg.Seeds)
	}
	if cfg.MaxHops != 8 || cfg.MaxTracked != 100 || cfg.MinFollowRaw.Cmp(flowUSDT(10_000)) != 0 {
		t.Fatalf("bounds not applied: hops=%d tracked=%d min=%s", cfg.MaxHops, cfg.MaxTracked, cfg.MinFollowRaw)
	}
	t.Setenv("KOSCHEI_TRON_FLOW_RADAR_ENABLED", "")
	if TronFlowRadarEnabled() {
		t.Fatal("the radar must be off by default")
	}
}

func TestTronFlowRadarCycleReadsSolidBlocksWithKeyInHeaderOnly(t *testing.T) {
	const key = "fixture-trongrid-key-not-real"
	seed, recipient := flowAddr(0x11), flowAddr(0x22)
	requested := []uint64{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("TRON-PRO-API-KEY") != key || strings.Contains(r.URL.String(), key) {
			t.Errorf("API key must travel only in the header")
		}
		switch r.URL.Path {
		case "/walletsolidity/getnowblock":
			fmt.Fprint(w, `{"block_header":{"raw_data":{"number":100}}}`)
		case "/walletsolidity/gettransactioninfobyblocknum":
			body, _ := io.ReadAll(r.Body)
			var call struct {
				Num uint64 `json:"num"`
			}
			_ = json.Unmarshal(body, &call)
			requested = append(requested, call.Num)
			if call.Num == 100 {
				fmt.Fprintf(w, `[{"id":"%s","blockNumber":100,"blockTimeStamp":5000,"receipt":{"result":"SUCCESS"},"log":[%s]}]`, flowTx(1), usdtLog(seed, recipient, flowUSDT(50_000)))
				return
			}
			fmt.Fprint(w, `{}`)
		default:
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	cfg := flowConfig(seed)
	cfg.Endpoint, cfg.APIKey, cfg.Backfill = server.URL, key, 1
	collector := &flowCollector{}
	radar := NewTronFlowRadar(cfg, nil, collector.sink, nil)
	radar.client = server.Client()

	processed, err := radar.cycle(context.Background())
	if err != nil || processed != 2 || fmt.Sprint(requested) != "[99 100]" {
		t.Fatalf("processed=%d requested=%v err=%v", processed, requested, err)
	}
	snapshot := radar.Snapshot()
	if snapshot.Cursor != 100 || snapshot.Head != 100 || snapshot.Followed != 2 || snapshot.Storage != "in_memory_only" || len(snapshot.RecentFindings) != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if processed, err := radar.cycle(context.Background()); err != nil || processed != 0 {
		t.Fatalf("an unchanged head must process nothing, got %d err=%v", processed, err)
	}

	radar.cfg.Endpoint = server.URL + "/missing"
	_, err = radar.cycle(context.Background())
	if err == nil || err.Error() != "tron_provider_http_500" || strings.Contains(err.Error(), server.URL) || strings.Contains(err.Error(), key) {
		t.Fatalf("provider failure must surface only a category, got %v", err)
	}
}
