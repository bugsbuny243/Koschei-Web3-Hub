package builtin

import (
	"context"
	r "koschei/api/internal/radaradapter"
	"koschei/api/internal/radaradapter/cosmos"
	"koschei/api/internal/radaradapter/stellar"
	"sort"
	"strings"
)

// Catalog includes deferred networks explicitly; declaration never grants execution.
func Catalog() []r.Capability {
	registry, _ := Registry()
	out := registry.Capabilities()
	for _, legacy := range []struct {
		network, family, fixture string
		targets                  []string
	}{{"bitcoin-mainnet", "utxo", "internal/networktarget/bitcoin_probe_test.go", []string{"address", "transaction"}}, {"sui-mainnet", "move", "internal/networktarget/move_identity_probe_test.go", []string{"address"}}, {"aptos-mainnet", "move", "internal/networktarget/move_identity_probe_test.go", []string{"address"}}} {
		out = append(out, r.Capability{ID: legacy.family + "-legacy", Network: legacy.network, Family: legacy.family, Status: "existing_separate_route", TargetClasses: legacy.targets, Sources: []string{"Existing production collector retained; not claimed by the new adapter registry."}, Fixture: legacy.fixture, EvidenceOnly: legacy.family != "solana"})
	}
	for _, network := range []string{"hyperliquid-core", "xrpl-mainnet", "cardano-mainnet", "polkadot-mainnet", "near-mainnet", "litecoin-mainnet", "dogecoin-mainnet", "bitcoincash-mainnet", "starknet-mainnet"} {
		out = append(out, r.Capability{ID: network, Network: network, Status: "declared_not_executable", TargetClasses: []string{}, Sources: []string{}, ArchivalRequirements: []string{"Protocol-specific collector, fixtures and provider configuration required."}, EvidenceOnly: true})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Network < out[j].Network })
	return out
}
func SampleTarget(c r.Capability) r.Target {
	t := r.Target{Network: c.Network, Kind: "address"}
	switch c.Family {
	case "solana":
		t.Raw = "11111111111111111111111111111111"
	case "tron":
		t.Raw = "T9yD14Nj9j7xAB4dbGeiX9h8unkKHxuWwb"
	case "evm":
		t.Raw = "0x" + strings.Repeat("0", 40)
	case "ton":
		t.Raw = "0:" + strings.Repeat("0", 64)
	case "cosmos":
		t.Raw = zeroBech32(cosmos.Chains[c.Network].Prefix)
	case "stellar":
		t.Raw = stellar.EncodeAddress(make([]byte, 32))
	case "offchain":
		t.Kind = "frontend"
		t.Raw = "https://dapp.fixture.invalid"
	}
	return t
}

type countingTransport struct {
	r.Transport
	calls int
}

func (t *countingTransport) Do(ctx context.Context, q r.Request) (r.Response, error) {
	t.calls++
	return t.Transport.Do(ctx, q)
}

// FixtureRequestCounts measures the real collector calls on explicit synthetic
// fixtures, not real provider quotas or a claim of live network validation.
func FixtureRequestCounts() map[string]int {
	registry, _ := Registry()
	counts := map[string]int{}
	for _, c := range registry.Capabilities() {
		base := FixtureTransport(c.Family, c.Network)
		if c.Family == "offchain" {
			base = r.TransportFunc(func(context.Context, r.Request) (r.Response, error) {
				return r.Response{Raw: []byte("<html></html>"), Host: "dapp.fixture.invalid"}, nil
			})
		}
		counter := &countingTransport{Transport: base}
		registry.Run(context.Background(), SampleTarget(c), counter)
		counts[c.Network] = counter.calls
	}
	return counts
}
