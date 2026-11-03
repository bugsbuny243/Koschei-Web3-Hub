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
	"sync"
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

type flowCollector struct {
	mu       sync.Mutex
	findings []TronFlowFinding
}

func (c *flowCollector) sink(_ context.Context, f TronFlowFinding) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.findings = append(c.findings, f)
}

func (c *flowCollector) kinds() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := []string{}
	for _, f := range c.findings {
		out = append(out, f.Kind+":"+f.Severity)
	}
	return out
}

func (c *flowCollector) ofKind(kind string) []TronFlowFinding {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := []TronFlowFinding{}
	for _, f := range c.findings {
		if f.Kind == kind {
			out = append(out, f)
		}
	}
	return out
}

func flowTransfer(tx byte, from, to string, usdt int64, ts int64) tron.BlockTransfer {
	return tron.BlockTransfer{TransactionID: flowTx(tx), BlockNumber: 1, BlockTimestampMS: ts, From: from, To: to, Value: flowUSDT(usdt)}
}

func flowRadar(cfg TronFlowRadarConfig, bridges map[string]TronBridge, holder TronHolderChecker) (*TronFlowRadar, *flowCollector) {
	collector := &flowCollector{}
	radar := NewTronFlowRadar(cfg, nil, collector.sink, nil)
	radar.state.bridges = bridges
	if bridges == nil {
		radar.state.bridges = map[string]TronBridge{}
	}
	radar.holder = holder
	if holder == nil {
		radar.holder = func(context.Context, string) TronHolderObservation {
			return TronHolderObservation{Freeze: tron.FreezeUnknown}
		}
	}
	return radar, collector
}

// Mirrors the incident shape: seed -> hop 1 -> hop 2 -> bridge.
func TestTronFlowFollowsSeedFundsHopByHopToABridge(t *testing.T) {
	seed, hop1, hop2, hop3, bridge := flowAddr(0x11), flowAddr(0x22), flowAddr(0x33), flowAddr(0x44), flowAddr(0x55)
	cfg := flowConfig(seed)
	cfg.MaxHops = 2
	radar, collector := flowRadar(cfg, map[string]TronBridge{bridge: {Label: "Fixture Bridge", Source: "https://example.invalid"}}, nil)
	ctx := context.Background()

	radar.ingest(ctx, []tron.BlockTransfer{flowTransfer(1, seed, hop1, 9_999, 1000)})
	if len(radar.state.nodes) != 1 || len(collector.kinds()) != 0 {
		t.Fatal("a transfer below the follow threshold must not be followed or reported")
	}
	radar.ingest(ctx, []tron.BlockTransfer{
		flowTransfer(2, seed, hop1, 500_000, 1001),
		flowTransfer(3, hop1, hop2, 400_000, 1002),
		flowTransfer(4, hop2, hop3, 100_000, 1003), // hop2 == MaxHops: not followed further
		flowTransfer(5, hop2, bridge, 300_000, 1004),
	})
	if radar.state.nodes[hop1].hop != 1 || radar.state.nodes[hop2].hop != 2 || radar.state.nodes[hop3] != nil {
		t.Fatal("recipients must be followed with their hop distance and never beyond the hop limit")
	}
	if got := strings.Join(collector.kinds(), ","); got != "tron_flow_seed_outflow:high,tron_flow_bridge_exit:critical" {
		t.Fatalf("findings = %s", got)
	}
	exit := collector.ofKind("tron_flow_bridge_exit")[0]
	if exit.Evidence["hop"] != 2 || exit.Evidence["origin_seed"] != tron.Base58(seed) || exit.Evidence["koschei_can_reverse_transfer"] != false ||
		exit.Evidence["identity_or_intent_claim"] != false || exit.Evidence["follow_state"] != "in_memory_only" {
		t.Fatalf("bridge exit evidence = %#v", exit.Evidence)
	}
	if !strings.Contains(exit.Message, "300000 USDT") || exit.DedupeKey != "tron-flow:bridge-exit:"+flowTx(5)+":0" {
		t.Fatalf("bridge exit = %q / %q", exit.Message, exit.DedupeKey)
	}
}

func TestTronFlowReportsOneFanOutPerWindow(t *testing.T) {
	seed := flowAddr(0x11)
	radar, collector := flowRadar(flowConfig(seed), nil, nil)
	for i := 0; i < 6; i++ {
		radar.ingest(context.Background(), []tron.BlockTransfer{flowTransfer(byte(10+i), seed, flowAddr(byte(0x60+i)), 20_000, int64(1000+i))})
	}
	if got := strings.Join(collector.kinds(), ","); got != "tron_flow_seed_outflow:high,tron_flow_fan_out:high" {
		t.Fatalf("findings = %s", got)
	}
	later := int64(1000) + (11 * time.Minute).Milliseconds()
	for i := 0; i < 3; i++ {
		radar.ingest(context.Background(), []tron.BlockTransfer{flowTransfer(byte(30+i), seed, flowAddr(byte(0x70+i)), 20_000, later+int64(i))})
	}
	if got := collector.kinds(); len(got) != 4 || got[2] != "tron_flow_seed_outflow:high" || got[3] != "tron_flow_fan_out:high" {
		t.Fatalf("a later burst must be reported again, got %v", got)
	}
}

// Codex P1: a holder finding requires the current balance, not just receipts.
func TestTronFlowHolderFindingRequiresCurrentBalance(t *testing.T) {
	seed, holder, emptied, frozen := flowAddr(0x11), flowAddr(0x22), flowAddr(0x33), flowAddr(0x44)
	calls := map[string]int{}
	checker := func(_ context.Context, address string) TronHolderObservation {
		calls[address]++
		switch address {
		case holder:
			return TronHolderObservation{Freeze: tron.FreezeClear, Balance: flowUSDT(1_100_000)}
		case emptied:
			return TronHolderObservation{Freeze: tron.FreezeClear, Balance: flowUSDT(3)} // funds already moved on
		default:
			return TronHolderObservation{Freeze: tron.FreezeFrozen}
		}
	}
	radar, collector := flowRadar(flowConfig(seed), nil, checker)
	ctx := context.Background()
	radar.ingest(ctx, []tron.BlockTransfer{flowTransfer(1, seed, holder, 600_000, 1000)})
	if calls[holder] != 0 {
		t.Fatal("no holder check below the freeze-check threshold")
	}
	radar.ingest(ctx, []tron.BlockTransfer{
		flowTransfer(2, seed, holder, 600_000, 1001),
		flowTransfer(3, seed, emptied, 2_000_000, 1002),
		flowTransfer(4, seed, frozen, 2_000_000, 1003),
	})
	candidates := collector.ofKind("tron_flow_freeze_candidate")
	if len(candidates) != 1 || candidates[0].Address != tron.Base58(holder) {
		t.Fatalf("only the unfrozen address that still holds USDT may be reported, got %+v", candidates)
	}
	evidence := candidates[0].Evidence
	if evidence["current_usdt_balance"] != "1100000 USDT" || evidence["followed_received"] != "1200000 USDT" || !strings.Contains(candidates[0].Message, "currently holds 1100000 USDT") {
		t.Fatalf("holder evidence must separate historical receipts from the current balance: %#v / %q", evidence, candidates[0].Message)
	}
	// Further receipts inside the recheck interval do not trigger new provider calls.
	radar.ingest(ctx, []tron.BlockTransfer{flowTransfer(5, seed, emptied, 20_000, 1004)})
	if calls[emptied] != 1 || calls[holder] != 1 || calls[frozen] != 1 {
		t.Fatalf("unexpected holder checks %v", calls)
	}
	rows := radar.Snapshot().TopFollowed
	if len(rows) < 4 || rows[1].Address != tron.Base58(emptied) || rows[1].CurrentBalance != "3 USDT" {
		t.Fatalf("followed table must show the current balance once read: %+v", rows)
	}
}

// Codex P2: unknown checks are retried without needing another receipt.
func TestTronFlowRetriesUnknownHolderChecksIndependently(t *testing.T) {
	seed, holder := flowAddr(0x11), flowAddr(0x22)
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	answers := []TronHolderObservation{{Freeze: tron.FreezeUnknown}, {Freeze: tron.FreezeClear}, {Freeze: tron.FreezeClear, Balance: flowUSDT(1_500_000)}}
	calls := 0
	checker := func(context.Context, string) TronHolderObservation {
		answer := answers[min(calls, len(answers)-1)]
		calls++
		return answer
	}
	radar, collector := flowRadar(flowConfig(seed), nil, checker)
	radar.state.now = func() time.Time { return now }
	ctx := context.Background()
	radar.ingest(ctx, []tron.BlockTransfer{flowTransfer(1, seed, holder, 1_500_000, 1000)})
	if calls != 1 || radar.Snapshot().PendingChecks != 1 {
		t.Fatalf("an unknown check must be queued, calls=%d", calls)
	}
	radar.retryPendingChecks(ctx)
	if calls != 1 {
		t.Fatal("a retry must wait for its backoff")
	}
	now = now.Add(time.Minute)
	radar.retryPendingChecks(ctx) // freeze readable but balance unreadable: still unknown
	now = now.Add(2 * time.Minute)
	radar.retryPendingChecks(ctx)
	if calls != 3 || len(collector.ofKind("tron_flow_freeze_candidate")) != 1 || radar.Snapshot().PendingChecks != 0 {
		t.Fatalf("retries must resolve without new receipts: calls=%d findings=%v", calls, collector.kinds())
	}
}

// Codex P2: provider I/O and delivery never run under the snapshot lock.
func TestTronFlowHolderChecksRunOutsideTheSnapshotLock(t *testing.T) {
	seed, holder := flowAddr(0x11), flowAddr(0x22)
	var radar *TronFlowRadar
	snapshotDuringCheck := make(chan bool, 1)
	checker := func(context.Context, string) TronHolderObservation {
		done := make(chan struct{})
		go func() { radar.Snapshot(); close(done) }()
		select {
		case <-done:
			snapshotDuringCheck <- true
		case <-time.After(2 * time.Second):
			snapshotDuringCheck <- false
		}
		return TronHolderObservation{Freeze: tron.FreezeClear, Balance: flowUSDT(2_000_000)}
	}
	radar, collector := flowRadar(flowConfig(seed), nil, checker)
	sinkSawUnlocked := false
	radar.sink = func(ctx context.Context, f TronFlowFinding) {
		if radar.mu.TryLock() {
			sinkSawUnlocked = true
			radar.mu.Unlock()
		}
		collector.sink(ctx, f)
	}
	radar.ingest(context.Background(), []tron.BlockTransfer{flowTransfer(1, seed, holder, 2_000_000, 1000)})
	if !<-snapshotDuringCheck {
		t.Fatal("Snapshot blocked while a holder check was running")
	}
	if !sinkSawUnlocked {
		t.Fatal("the alert sink must run without the radar lock")
	}
}

// Codex P2: seed refreshes keep the graph within MaxTracked.
func TestTronFlowSeedRefreshEnforcesTheTrackedBound(t *testing.T) {
	cfg := flowConfig(flowAddr(0x01))
	cfg.MaxTracked, cfg.BurstCount = 5, 1000
	radar, _ := flowRadar(cfg, nil, nil)
	for i := 0; i < 20; i++ {
		radar.ingest(context.Background(), []tron.BlockTransfer{flowTransfer(byte(i), flowAddr(0x01), flowAddr(byte(0x80+i)), 20_000, int64(1000+i))})
	}
	for round := 0; round < 10; round++ {
		radar.state.setSeeds([]string{flowAddr(byte(0x10 + round)), flowAddr(byte(0x20 + round))})
		if len(radar.state.nodes) > cfg.MaxTracked {
			t.Fatalf("round %d: graph grew to %d nodes, bound %d", round, len(radar.state.nodes), cfg.MaxTracked)
		}
	}
	many := []string{}
	for i := 0; i < 9; i++ {
		many = append(many, flowAddr(byte(0xa0+i)))
	}
	radar.state.setSeeds(many)
	if len(radar.state.nodes) != cfg.MaxTracked {
		t.Fatalf("more seeds than the bound must be truncated to it, got %d", len(radar.state.nodes))
	}
	for _, node := range radar.state.nodes {
		if !node.seed {
			t.Fatal("seeds take priority over followed nodes at the bound")
		}
	}
}

func TestTronFlowFindingsAreCappedPerHour(t *testing.T) {
	seedA, seedB, seedC := flowAddr(0x11), flowAddr(0x12), flowAddr(0x13)
	cfg := flowConfig(seedA, seedB, seedC)
	cfg.MaxFindings = 2
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	radar, collector := flowRadar(cfg, nil, nil)
	radar.state.now = func() time.Time { return now }
	for i, seed := range []string{seedA, seedB, seedC} {
		radar.ingest(context.Background(), []tron.BlockTransfer{flowTransfer(byte(i+1), seed, flowAddr(byte(0x90+i)), 20_000, 1000)})
	}
	if len(collector.kinds()) != 2 || radar.Snapshot().Suppressed != 1 {
		t.Fatalf("findings=%d suppressed=%d", len(collector.kinds()), radar.Snapshot().Suppressed)
	}
	now = now.Add(time.Hour)
	radar.ingest(context.Background(), []tron.BlockTransfer{flowTransfer(9, seedA, flowAddr(0xa0), 20_000, 1000+(11*time.Minute).Milliseconds())})
	if len(collector.kinds()) != 3 || radar.Snapshot().Suppressed != 0 {
		t.Fatalf("the cap must reset after an hour: findings=%d", len(collector.kinds()))
	}
}

func TestTronFlowNetworkWideLargeBridgeInflow(t *testing.T) {
	stranger, bridge := flowAddr(0x21), flowAddr(0x55)
	radar, collector := flowRadar(flowConfig(), map[string]TronBridge{bridge: {Label: "Fixture Bridge", Source: "https://example.invalid"}}, nil)
	radar.ingest(context.Background(), []tron.BlockTransfer{flowTransfer(1, stranger, bridge, 999_999, 1000), flowTransfer(2, stranger, bridge, 1_000_000, 1001)})
	if got := strings.Join(collector.kinds(), ","); got != "tron_large_bridge_inflow:info" {
		t.Fatalf("findings = %s", got)
	}
	if len(radar.state.nodes) != 0 {
		t.Fatal("an unlinked sender must not be followed")
	}
}

func TestTronFlowSnapshotListsSeedsThenLargestFollowedHolders(t *testing.T) {
	seed, small, large := flowAddr(0x11), flowAddr(0x22), flowAddr(0x33)
	radar, _ := flowRadar(flowConfig(seed), nil, nil)
	radar.ingest(context.Background(), []tron.BlockTransfer{flowTransfer(1, seed, small, 20_000, 1000), flowTransfer(2, seed, large, 300_000, 1001)})
	snapshot := radar.Snapshot()
	got := []string{}
	for _, item := range snapshot.TopFollowed {
		got = append(got, item.Address)
	}
	if strings.Join(got, ",") != strings.Join([]string{tron.Base58(seed), tron.Base58(large), tron.Base58(small)}, ",") {
		t.Fatalf("top followed = %v", got)
	}
	if snapshot.TopFollowed[1].Received != "300000 USDT" || snapshot.TopFollowed[1].USDTFreezeState != "not_checked" || snapshot.Settings.MinFollow != "10000 USDT" || snapshot.Storage != "in_memory_only" {
		t.Fatalf("unexpected snapshot %+v", snapshot)
	}
}

func TestLoadTronFlowRadarConfigBoundsSeedsAndEnabledValues(t *testing.T) {
	t.Setenv("KOSCHEI_TRON_FLOW_RADAR_RPC_URL", "")
	t.Setenv("KOSCHEI_TRON_FLOW_RADAR_SEEDS", tron.USDTContract+", not-an-address ,"+flowAddr(0x11))
	t.Setenv("KOSCHEI_TRON_FLOW_RADAR_MAX_HOPS", "99")
	t.Setenv("KOSCHEI_TRON_FLOW_RADAR_MAX_TRACKED", "1")
	t.Setenv("KOSCHEI_TRON_FLOW_RADAR_MIN_FOLLOW_USDT", "abc")
	cfg := LoadTronFlowRadarConfigFromEnv()
	usdt, _ := tron.Address(tron.USDTContract)
	if cfg.Endpoint != "https://api.trongrid.io" || len(cfg.Seeds) != 2 || cfg.Seeds[0] != usdt || cfg.Seeds[1] != flowAddr(0x11) {
		t.Fatalf("unexpected endpoint/seeds %q %v", cfg.Endpoint, cfg.Seeds)
	}
	if cfg.MaxHops != 8 || cfg.MaxTracked != 100 || cfg.MinFollowRaw.Cmp(flowUSDT(10_000)) != 0 {
		t.Fatalf("bounds not applied: hops=%d tracked=%d min=%s", cfg.MaxHops, cfg.MaxTracked, cfg.MinFollowRaw)
	}
	for value, want := range map[string]bool{"": false, "0": false, "false": false, "1": true, "true": true, "TRUE": true, "yes": true, "on": true} {
		t.Setenv("KOSCHEI_TRON_FLOW_RADAR_ENABLED", value)
		if TronFlowRadarEnabled() != want {
			t.Fatalf("enabled(%q) = %v, want %v", value, !want, want)
		}
	}
}

func TestTronFlowRadarCycleReadsSolidBlocksThroughTheAdapter(t *testing.T) {
	const key = "fixture-trongrid-key-not-real"
	seed, recipient := flowAddr(0x11), flowAddr(0x22)
	usdt, _ := tron.Address(tron.USDTContract)
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
				fmt.Fprintf(w, `[{"id":"%s","blockNumber":100,"blockTimeStamp":5000,"receipt":{"result":"SUCCESS"},"log":[{"address":"%s","topics":["ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","%s%s","%s%s"],"data":"%064x"}]}]`,
					flowTx(1), usdt[2:], strings.Repeat("0", 24), seed[2:], strings.Repeat("0", 24), recipient[2:], flowUSDT(50_000))
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
	radar, collector := flowRadar(cfg, nil, nil)
	radar.grid = tron.GridClient{BaseURL: server.URL, APIKey: key, HTTP: server.Client()}

	processed, err := radar.cycle(context.Background())
	if err != nil || processed != 2 || fmt.Sprint(requested) != "[99 100]" {
		t.Fatalf("processed=%d requested=%v err=%v", processed, requested, err)
	}
	snapshot := radar.Snapshot()
	if snapshot.Cursor != 100 || snapshot.Head != 100 || snapshot.Followed != 2 || snapshot.Storage != "in_memory_only" || len(collector.kinds()) != 1 {
		t.Fatalf("snapshot = %+v findings=%v", snapshot, collector.kinds())
	}
	if processed, err := radar.cycle(context.Background()); err != nil || processed != 0 {
		t.Fatalf("an unchanged head must process nothing, got %d err=%v", processed, err)
	}
	radar.grid.BaseURL = server.URL + "/missing"
	_, err = radar.cycle(context.Background())
	if err == nil || err.Error() != "tron_provider_http_500" || strings.Contains(err.Error(), server.URL) || strings.Contains(err.Error(), key) {
		t.Fatalf("provider failure must surface only a category, got %v", err)
	}
}
