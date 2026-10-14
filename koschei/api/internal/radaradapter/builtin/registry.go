// Package builtin registers collectors only after replaying explicit synthetic
// protocol fixtures. These fixtures prove parser wiring, not live provider availability.
package builtin

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	r "koschei/api/internal/radaradapter"
	"koschei/api/internal/radaradapter/cosmos"
	"koschei/api/internal/radaradapter/evm"
	"koschei/api/internal/radaradapter/frontend"
	"koschei/api/internal/radaradapter/solana"
	"koschei/api/internal/radaradapter/stellar"
	"koschei/api/internal/radaradapter/ton"
	"koschei/api/internal/radaradapter/tron"
	"strings"
)

//go:embed fixtures/*.json
var Fixtures embed.FS

func FixtureTransport(family, network string) r.Transport {
	return r.TransportFunc(func(_ context.Context, q r.Request) (r.Response, error) {
		b, e := Fixtures.ReadFile("fixtures/" + family + ".json")
		if e != nil {
			return r.Response{}, e
		}
		chain := ""
		if family == "solana" {
			var fixtures map[string]json.RawMessage
			if err := json.Unmarshal(b, &fixtures); err != nil {
				return r.Response{}, err
			}
			body, _ := q.Body.(map[string]any)
			key, _ := body["method"].(string)
			value, ok := fixtures[key]
			if !ok {
				return r.Response{}, errors.New("fixture has no such method")
			}
			payload, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": json.RawMessage(value)})
			return r.Response{Raw: payload, Host: q.Provider + ".fixture.invalid"}, nil
		}
		if family == "evm" {
			chain = fmt.Sprintf("0x%x", evm.Chains[network])
		}
		if family == "cosmos" {
			chain = cosmos.Chains[network].ID
		}
		b = []byte(strings.ReplaceAll(string(b), "$CHAIN_ID", chain))
		var fixtures map[string]json.RawMessage
		if e = json.Unmarshal(b, &fixtures); e != nil {
			return r.Response{}, e
		}
		key := strings.Split(q.Path, "?")[0]
		rpc := q.Path == ""
		if rpc {
			body, _ := q.Body.(map[string]any)
			key, _ = body["method"].(string)
		}
		if strings.Contains(key, "/grants/granter/") {
			key = "$AUTHZ"
		}
		if strings.Contains(key, "/issued/") {
			key = "$FEEGRANT"
		}
		value, ok := fixtures[key]
		if !ok {
			return r.Response{}, errors.New("fixture has no such method")
		}
		if rpc {
			value, _ = json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": json.RawMessage(value)})
		}
		return r.Response{Raw: value, Host: q.Provider + ".fixture.invalid"}, nil
	})
}
func Registry() (*r.Registry, []string) {
	registry := r.NewRegistry()
	failures := []string{}
	register := func(family, network, id string, f r.Factory) {
		t := r.Target{Network: network, Kind: "address", Raw: id}
		if e := registry.Register(f, t, FixtureTransport(family, network)); e != nil {
			failures = append(failures, network+": "+e.Error())
		}
	}
	register("solana", "solana-mainnet", "11111111111111111111111111111111", solana.New)
	register("tron", "tron-mainnet", "T9yD14Nj9j7xAB4dbGeiX9h8unkKHxuWwb", tron.New)
	for network := range evm.Chains {
		register("evm", network, "0x"+strings.Repeat("0", 40), evm.New)
	}
	register("ton", "ton-mainnet", "0:"+strings.Repeat("0", 64), ton.New)
	for network, chain := range cosmos.Chains {
		register("cosmos", network, zeroBech32(chain.Prefix), cosmos.New)
	}
	register("stellar", "stellar-mainnet", stellar.EncodeAddress(make([]byte, 32)), stellar.New)
	ft := r.Target{Network: "offchain", Kind: "frontend", Raw: "https://dapp.fixture.invalid"}
	if e := registry.Register(frontend.New, ft, r.TransportFunc(func(context.Context, r.Request) (r.Response, error) {
		return r.Response{Raw: []byte("<html></html>"), Host: "dapp.fixture.invalid"}, nil
	})); e != nil {
		failures = append(failures, e.Error())
	}
	return registry, failures
}
func zeroBech32(hrp string) string {
	const alphabet = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
	values := []byte{}
	for _, c := range hrp {
		values = append(values, byte(c)>>5)
	}
	values = append(values, 0)
	for _, c := range hrp {
		values = append(values, byte(c)&31)
	}
	values = append(values, make([]byte, 38)...)
	check := uint32(1)
	g := []uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	for _, v := range values {
		top := check >> 25
		check = (check&0x1ffffff)<<5 ^ uint32(v)
		for k := 0; k < 5; k++ {
			if top>>k&1 != 0 {
				check ^= g[k]
			}
		}
	}
	check ^= 1
	out := hrp + "1" + strings.Repeat("q", 32)
	for i := 5; i >= 0; i-- {
		out += string(alphabet[(check>>uint(5*i))&31])
	}
	return out
}
