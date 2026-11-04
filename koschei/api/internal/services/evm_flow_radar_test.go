package services

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
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

	"koschei/api/internal/radaradapter/evm"
)

// Explicit synthetic fixtures: addresses and transaction hashes are repeated
// bytes and do not refer to real incident wallets. The fixture tokens are not
// real contracts.
func evmFlowAddr(fill byte) string { return "0x" + strings.Repeat(fmt.Sprintf("%02x", fill), 20) }
func evmFlowTx(fill byte) string   { return "0x" + strings.Repeat(fmt.Sprintf("%02x", fill), 32) }

var (
	evmFixtureTether = evm.FlowToken{Address: evmFlowAddr(0xa1), Symbol: "USDT", Decimals: 6, Issuer: "Tether", IssuerOption: "issuer_freeze_request:tether_usdt", FreezeMethod: "isBlackListed(address)"}
	evmFixturePeg    = evm.FlowToken{Address: evmFlowAddr(0xa2), Symbol: "PEG", Decimals: 18}
)

func evmUnits(whole int64, decimals int) *big.Int {
	return new(big.Int).Mul(big.NewInt(whole), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
}

func evmFlowConfig(seeds ...string) EVMFlowRadarConfig {
	return EVMFlowRadarConfig{
		Network: "ethereum-mainnet", Explorer: "https://etherscan.io", Tokens: []evm.FlowToken{evmFixtureTether, evmFixturePeg}, Seeds: seeds,
		MinFollowUSD6: evmFlowUSD6(10_000), BridgeAlertUSD6: evmFlowUSD6(1_000_000), FreezeCheckUSD6: evmFlowUSD6(1_000_000),
		MaxHops: 3, BurstCount: 3, BurstWindow: 10 * time.Minute, BlockTime: 12 * time.Second, MaxTracked: 1000, MaxFindings: 100,
		Poll: time.Second, Confirmations: 3, MaxBlockRange: 10, MaxRangesPerCycle: 5,
	}
}

type evmFlowCollector struct {
	mu       sync.Mutex
	findings []EVMFlowFinding
}

func (c *evmFlowCollector) sink(_ context.Context, f EVMFlowFinding) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.findings = append(c.findings, f)
}

func (c *evmFlowCollector) ofKind(kind string) []EVMFlowFinding {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := []EVMFlowFinding{}
	for _, f := range c.findings {
		if f.Kind == kind {
			out = append(out, f)
		}
	}
	return out
}

func (c *evmFlowCollector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.findings)
}

func evmTransfer(tx byte, token evm.FlowToken, from, to string, whole int64, block uint64) evm.FlowTransfer {
	return evm.FlowTransfer{TransactionHash: evmFlowTx(tx), LogIndex: uint64(tx), BlockNumber: block, BlockHash: evmFlowTx(0xbb),
		Token: token.Address, From: from, To: to, Value: evmUnits(whole, token.Decimals)}
}

func evmFlowRadarFixture(cfg EVMFlowRadarConfig, bridges map[string]EVMBridge, holder EVMHolderChecker) (*EVMFlowRadar, *evmFlowCollector) {
	collector := &evmFlowCollector{}
	radar := NewEVMFlowRadar(cfg, nil, collector.sink, nil)
	radar.state.bridges = bridges
	if bridges == nil {
		radar.state.bridges = map[string]EVMBridge{}
	}
	radar.holder = holder
	if holder == nil {
		radar.holder = func(context.Context, string, evm.FlowToken) EVMHolderObservation {
			return EVMHolderObservation{Freeze: evm.FreezeUnknown}
		}
	}
	return radar, collector
}

func TestEVMFlowRadarFollowsSeedOutflowHopByHopWithinLimits(t *testing.T) {
	seed, hop1, hop2, hop3, hop4 := evmFlowAddr(0x11), evmFlowAddr(0x12), evmFlowAddr(0x13), evmFlowAddr(0x14), evmFlowAddr(0x15)
	radar, collector := evmFlowRadarFixture(evmFlowConfig(seed), nil, nil)
	radar.ingest(context.Background(), []evm.FlowTransfer{
		evmTransfer(1, evmFixtureTether, seed, hop1, 200_000, 100),
		evmTransfer(2, evmFixtureTether, hop1, hop2, 150_000, 101),
		evmTransfer(3, evmFixtureTether, hop2, hop3, 120_000, 102),
		evmTransfer(4, evmFixtureTether, hop3, hop4, 110_000, 103), // hop 3 is the limit; hop4 is not followed
		evmTransfer(5, evmFixtureTether, evmFlowAddr(0x99), evmFlowAddr(0x98), 5_000_000, 103),
		evmTransfer(6, evmFixtureTether, seed, evmFlowAddr(0x16), 9_999, 400), // below the follow threshold
	})
	outflows := collector.ofKind("evm_flow_seed_outflow")
	if len(outflows) != 1 || outflows[0].Address != seed || outflows[0].Network != "ethereum-mainnet" || outflows[0].Severity != "high" {
		t.Fatalf("seed outflow findings = %+v", outflows)
	}
	evidence := outflows[0].Evidence
	if evidence["identity_or_intent_claim"] != false || evidence["koschei_can_reverse_transfer"] != false || evidence["follow_state"] != "in_memory_only" {
		t.Fatalf("truth-boundary evidence missing: %v", evidence)
	}
	options := evidence["response_options"].([]string)
	if strings.Join(options, ",") != "issuer_freeze_request:tether_usdt,receiving_party_notice" {
		t.Fatalf("response options = %v", options)
	}
	snapshot := radar.Snapshot()
	if snapshot.Seeds != 1 || snapshot.Followed != 4 || snapshot.Transfers != 6 {
		t.Fatalf("snapshot seeds=%d followed=%d transfers=%d", snapshot.Seeds, snapshot.Followed, snapshot.Transfers)
	}
	for _, row := range snapshot.TopFollowed {
		if row.Address == hop4 || row.Address == evmFlowAddr(0x16) {
			t.Fatalf("%s must not be followed", row.Address)
		}
		if row.Address == hop3 && (row.Hop != 3 || row.OriginSeed != seed || row.Received != "120000 USD (stablecoins)") {
			t.Fatalf("hop3 row = %+v", row)
		}
	}
}

func TestEVMFlowRadarNormalizes18DecimalTokensAndSkipsHolderChecksWithoutIssuerFreeze(t *testing.T) {
	seed, hop := evmFlowAddr(0x21), evmFlowAddr(0x22)
	checks := 0
	radar, _ := evmFlowRadarFixture(evmFlowConfig(seed), nil, func(context.Context, string, evm.FlowToken) EVMHolderObservation {
		checks++
		return EVMHolderObservation{Freeze: evm.FreezeClear, Balance: big.NewInt(0)}
	})
	radar.ingest(context.Background(), []evm.FlowTransfer{evmTransfer(1, evmFixturePeg, seed, hop, 2_000_000, 50)})
	snapshot := radar.Snapshot()
	if snapshot.Followed != 2 {
		t.Fatalf("an 18-decimal transfer of 2,000,000 must be followed; followed=%d", snapshot.Followed)
	}
	for _, row := range snapshot.TopFollowed {
		if row.Address == hop {
			if row.Received != "2000000 USD (stablecoins)" || len(row.Holdings) != 1 || row.Holdings[0].FreezeState != evm.FreezeUnsupported || row.Holdings[0].Received != "2000000 PEG" {
				t.Fatalf("hop row = %+v", row)
			}
		}
	}
	if checks != 0 {
		t.Fatal("a token without an issuer freeze function must not trigger holder checks")
	}
}

func TestEVMFlowRadarFanOutAndBridgeExit(t *testing.T) {
	seed, hop, bridge := evmFlowAddr(0x31), evmFlowAddr(0x32), evmFlowAddr(0x55)
	radar, collector := evmFlowRadarFixture(evmFlowConfig(seed), map[string]EVMBridge{bridge: {Label: "Fixture Bridge", Source: "https://example.invalid"}}, nil)
	radar.ingest(context.Background(), []evm.FlowTransfer{
		evmTransfer(1, evmFixtureTether, seed, hop, 900_000, 10),
		evmTransfer(2, evmFixtureTether, hop, evmFlowAddr(0x41), 20_000, 11),
		evmTransfer(3, evmFixtureTether, hop, evmFlowAddr(0x42), 20_000, 12),
		evmTransfer(4, evmFixtureTether, hop, evmFlowAddr(0x43), 20_000, 13),
		evmTransfer(5, evmFixtureTether, hop, bridge, 400_000, 14),
		evmTransfer(6, evmFixtureTether, evmFlowAddr(0x77), bridge, 3_000_000, 15),
	})
	fanOut := collector.ofKind("evm_flow_fan_out")
	if len(fanOut) != 1 || fanOut[0].Address != hop || fanOut[0].Evidence["window_blocks"] != uint64(50) {
		t.Fatalf("fan-out findings = %+v", fanOut)
	}
	exits := collector.ofKind("evm_flow_bridge_exit")
	if len(exits) != 1 || exits[0].Severity != "critical" || !strings.HasPrefix(exits[0].DedupeKey, "evm-flow:ethereum-mainnet:bridge-exit:") {
		t.Fatalf("bridge exits = %+v", exits)
	}
	if got := strings.Join(exits[0].Evidence["response_options"].([]string), ","); got != "bridge_operator_notice:Fixture Bridge,issuer_freeze_request:tether_usdt" {
		t.Fatalf("bridge exit options = %s", got)
	}
	inflows := collector.ofKind("evm_large_bridge_inflow")
	if len(inflows) != 1 || inflows[0].Severity != "info" || inflows[0].Address != evmFlowAddr(0x77) {
		t.Fatalf("unlinked bridge inflows = %+v", inflows)
	}
}

func TestEVMFlowRadarHolderCheckRequiresUnfrozenCurrentBalance(t *testing.T) {
	seed, frozen, unknown, holder := evmFlowAddr(0x51), evmFlowAddr(0x52), evmFlowAddr(0x53), evmFlowAddr(0x54)
	observations := map[string]EVMHolderObservation{
		frozen:  {Freeze: evm.FreezeFrozen},
		unknown: {Freeze: evm.FreezeUnknown},
		holder:  {Freeze: evm.FreezeClear, Balance: evmUnits(1_500_000, 6)},
	}
	radar, collector := evmFlowRadarFixture(evmFlowConfig(seed), nil, func(_ context.Context, address string, token evm.FlowToken) EVMHolderObservation {
		if token.Address != evmFixtureTether.Address {
			t.Errorf("holder check for unexpected token %s", token.Address)
		}
		return observations[address]
	})
	radar.ingest(context.Background(), []evm.FlowTransfer{
		evmTransfer(1, evmFixtureTether, seed, frozen, 1_200_000, 20),
		evmTransfer(2, evmFixtureTether, seed, unknown, 1_200_000, 20),
		evmTransfer(3, evmFixtureTether, seed, holder, 1_600_000, 20),
	})
	candidates := collector.ofKind("evm_flow_freeze_candidate")
	if len(candidates) != 1 || candidates[0].Address != holder {
		t.Fatalf("freeze candidates = %+v", candidates)
	}
	evidence := candidates[0].Evidence
	if evidence["issuer_freeze_state"] != evm.FreezeClear || evidence["current_balance"] != "1500000 USDT" || evidence["followed_received"] != "1600000 USDT" ||
		!strings.Contains(candidates[0].Message, "Only Tether can freeze it") || candidates[0].DedupeKey != "evm-flow:ethereum-mainnet:freeze-candidate:"+holder+":USDT" {
		t.Fatalf("freeze candidate = %+v", candidates[0])
	}
	radar.mu.Lock()
	retry, queued := radar.state.pending[unknown+"|"+evmFixtureTether.Address]
	radar.mu.Unlock()
	if !queued || retry.attempts != 1 {
		t.Fatalf("an unreadable freeze state must be queued for retry, got %+v queued=%v", retry, queued)
	}
	snapshot := radar.Snapshot()
	for _, row := range snapshot.TopFollowed {
		if row.Address == frozen && row.Holdings[0].FreezeState != evm.FreezeFrozen {
			t.Fatalf("frozen holder row = %+v", row)
		}
	}
}

func TestEVMFlowRadarCapsFindingsPerHour(t *testing.T) {
	cfg := evmFlowConfig()
	cfg.MaxFindings = 2
	bridge := evmFlowAddr(0x55)
	radar, collector := evmFlowRadarFixture(cfg, map[string]EVMBridge{bridge: {Label: "Fixture Bridge", Source: "https://example.invalid"}}, nil)
	transfers := []evm.FlowTransfer{}
	for i := byte(1); i <= 5; i++ {
		transfers = append(transfers, evmTransfer(i, evmFixtureTether, evmFlowAddr(0x60+i), bridge, 2_000_000, uint64(i)))
	}
	radar.ingest(context.Background(), transfers)
	if collector.count() != 2 || radar.Snapshot().Suppressed != 3 {
		t.Fatalf("delivered=%d suppressed=%d", collector.count(), radar.Snapshot().Suppressed)
	}
}

func TestEVMFlowRadarCycleHalvesRejectedRangesAndAdvancesCursor(t *testing.T) {
	seed, hop := evmFlowAddr(0x71), evmFlowAddr(0x72)
	var mu sync.Mutex
	ranges := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		_ = json.Unmarshal(body, &request)
		switch request.Method {
		case "eth_blockNumber":
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":"0x64"}`) // 100; confirmed head 97
		case "eth_getLogs":
			var filter struct{ FromBlock, ToBlock string }
			_ = json.Unmarshal(request.Params[0], &filter)
			mu.Lock()
			ranges = append(ranges, filter.FromBlock+"-"+filter.ToBlock)
			mu.Unlock()
			var from, to uint64
			fmt.Sscanf(filter.FromBlock, "0x%x", &from)
			fmt.Sscanf(filter.ToBlock, "0x%x", &to)
			if to-from+1 > 2 {
				_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"You can make eth_getLogs requests with up to a 2 block range"}}`)
				return
			}
			logs := "[]"
			if from == 87 {
				logs = fmt.Sprintf(`[{"address":"%s","topics":["%s","0x%024d%s","0x%024d%s"],"data":"0x%064x","blockNumber":"0x57","blockHash":"%s","transactionHash":"%s","logIndex":"0x0","removed":false}]`,
					evmFixtureTether.Address, evm.TransferTopic, 0, seed[2:], 0, hop[2:], evmUnits(50_000, 6), evmFlowTx(0xbb), evmFlowTx(0x01))
			}
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":`+logs+`}`)
		default:
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"no"}}`)
		}
	}))
	defer server.Close()
	cfg := evmFlowConfig(seed)
	cfg.Endpoint, cfg.MaxBlockRange, cfg.MaxRangesPerCycle, cfg.Backfill = server.URL, 8, 20, 10
	radar, collector := evmFlowRadarFixture(cfg, nil, nil)
	radar.client.HTTP = server.Client()

	processed, caughtUp, err := radar.cycle(context.Background())
	if err != nil || !caughtUp || processed != 11 { // backfill 10 starts at confirmed head 97 - 10 = 87
		t.Fatalf("processed=%d caughtUp=%v err=%v ranges=%v", processed, caughtUp, err, ranges)
	}
	snapshot := radar.Snapshot()
	if snapshot.Cursor != 97 || snapshot.Head != 97 || snapshot.LatestBlock != 100 || snapshot.BlockRange != 2 {
		t.Fatalf("cursor=%d head=%d latest=%d range=%d", snapshot.Cursor, snapshot.Head, snapshot.LatestBlock, snapshot.BlockRange)
	}
	mu.Lock()
	first := ranges[:3]
	mu.Unlock()
	if strings.Join(first, ",") != "0x57-0x5e,0x57-0x5a,0x57-0x58" {
		t.Fatalf("range must halve 8 -> 4 -> 2 from block 87: %v", first)
	}
	if len(collector.ofKind("evm_flow_seed_outflow")) != 1 {
		t.Fatal("the seed transfer in block 87 must be processed after the range shrank")
	}
}

func TestEVMFlowRadarEnvParsing(t *testing.T) {
	t.Setenv("KOSCHEI_EVM_FLOW_RADAR_NETWORKS", "")
	if supported, unsupported := EVMFlowRadarNetworks(); strings.Join(supported, ",") != "ethereum-mainnet" || len(unsupported) != 0 {
		t.Fatalf("default networks = %v / %v", supported, unsupported)
	}
	t.Setenv("KOSCHEI_EVM_FLOW_RADAR_NETWORKS", " Base-Mainnet, ethereum-mainnet, solana-mainnet, base-mainnet ")
	if supported, unsupported := EVMFlowRadarNetworks(); strings.Join(supported, ",") != "base-mainnet,ethereum-mainnet" || strings.Join(unsupported, ",") != "solana-mainnet" {
		t.Fatalf("networks = %v / %v", supported, unsupported)
	}
	t.Setenv("BASE_RPC_URL", "https://base.example.invalid/v2/fixture")
	t.Setenv("KOSCHEI_EVM_FLOW_RADAR_SEEDS", "0x"+strings.Repeat("ab", 20)+", not-an-address")
	t.Setenv("KOSCHEI_EVM_FLOW_RADAR_MIN_FOLLOW_USD", "25000")
	cfg, ok := LoadEVMFlowRadarConfigFromEnv("base-mainnet")
	if !ok || cfg.Endpoint != "https://base.example.invalid/v2/fixture" || cfg.EndpointEnv != "BASE_RPC_URL" || cfg.Confirmations != 10 ||
		len(cfg.Seeds) != 1 || cfg.MinFollowUSD6.Cmp(evmFlowUSD6(25_000)) != 0 || cfg.Tokens[0].Symbol != "USDC" {
		t.Fatalf("config = %+v", cfg)
	}
	if _, ok := LoadEVMFlowRadarConfigFromEnv("tron-mainnet"); ok {
		t.Fatal("non-EVM networks must not load")
	}
	t.Setenv("KOSCHEI_EVM_FLOW_RADAR_BRIDGES_JSON", `{
		"0x`+strings.Repeat("cd", 20)+`": {"label":"Everywhere Bridge","source":"https://example.invalid/a"},
		"0x`+strings.Repeat("ce", 20)+`": {"label":"Base Bridge","source":"https://example.invalid/b","networks":["base-mainnet"]},
		"0x`+strings.Repeat("cf", 20)+`": {"label":"No Source","source":"http://example.invalid"}}`)
	if bridges := EVMPinnedBridges("base-mainnet"); len(bridges) != 2 {
		t.Fatalf("base bridges = %v", bridges)
	}
	if bridges := EVMPinnedBridges("ethereum-mainnet"); len(bridges) != 1 || bridges["0x"+strings.Repeat("cd", 20)].Label != "Everywhere Bridge" {
		t.Fatalf("ethereum bridges = %v", bridges)
	}
}

func TestEVMFlowEvidencePackageIsSignedAndVerifiable(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 7)
	}
	t.Setenv("KOSCHEI_VERDICT_SIGNING_KEY_ID", "fixture-key-evm")
	t.Setenv("KOSCHEI_VERDICT_SIGNING_PRIVATE_KEY", hex.EncodeToString(seed))
	wallet, hop, bridge := evmFlowAddr(0x81), evmFlowAddr(0x82), evmFlowAddr(0x55)
	radar, collector := evmFlowRadarFixture(evmFlowConfig(wallet), map[string]EVMBridge{bridge: {Label: "Fixture Bridge", Source: "https://example.invalid"}}, nil)
	registerEVMFlowRadar(radar)
	radar.ingest(context.Background(), []evm.FlowTransfer{evmTransfer(1, evmFixtureTether, wallet, hop, 500_000, 1), evmTransfer(2, evmFixtureTether, hop, bridge, 400_000, 2)})
	exits := collector.ofKind("evm_flow_bridge_exit")
	if len(exits) != 1 {
		t.Fatal("fixture must produce one bridge exit")
	}
	finding, ok := FindEVMFlowFinding(exits[0].DedupeKey)
	if !ok {
		t.Fatal("finding must be retrievable from the active radar registry")
	}
	pkg, err := BuildEVMFlowEvidencePackage(finding, time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC))
	if err != nil || pkg.SignatureStatus != "signed" || pkg.Signature.KeyID != "fixture-key-evm" || !VerifyEVMFlowEvidencePackage(pkg) {
		t.Fatalf("package must be signed and verify: status=%q err=%v", pkg.SignatureStatus, err)
	}
	if strings.Join(pkg.Recipients, "|") != "Fixture Bridge (bridge operator): hold or block request|Tether (USDT issuer): freeze request" {
		t.Fatalf("recipients = %v", pkg.Recipients)
	}
	if pkg.PublicLinks["transaction"] != "https://etherscan.io/tx/"+evmFlowTx(2) || pkg.PublicLinks["to_address"] != "https://etherscan.io/address/"+bridge {
		t.Fatalf("links = %v", pkg.PublicLinks)
	}
	encoded, _ := json.Marshal(pkg)
	var received EVMFlowEvidencePackage
	if err := json.Unmarshal(encoded, &received); err != nil || !VerifyEVMFlowEvidencePackage(received) {
		t.Fatalf("package must verify after a JSON round trip: %v", err)
	}
	received.Finding.Message = "tampered"
	if VerifyEVMFlowEvidencePackage(received) {
		t.Fatal("a tampered package must not verify")
	}
	t.Setenv("KOSCHEI_VERDICT_SIGNING_PRIVATE_KEY", "")
	t.Setenv("KOSCHEI_VERDICT_SIGNING_KEY_ID", "")
	unsigned, err := BuildEVMFlowEvidencePackage(finding, time.Now())
	if err != nil || unsigned.SignatureStatus != "signer_not_configured" || unsigned.Signature != nil {
		t.Fatalf("without a signer the package must say so: %+v err=%v", unsigned.SignatureStatus, err)
	}
}
