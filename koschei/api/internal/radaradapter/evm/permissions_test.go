package evm

import (
	"context"
	"encoding/json"
	"fmt"
	r "koschei/api/internal/radaradapter"
	"strings"
	"testing"
)

func TestExtendedPermissionsReconcilePinnedState(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(fmt.Sprint(invalid), func(t *testing.T) {
			owner := "0x" + strings.Repeat("1", 40)
			token := "0x" + strings.Repeat("2", 40)
			spender := "0x" + strings.Repeat("3", 40)
			word := func(a string) string { return "0x" + strings.Repeat("0", 24) + a[2:] }
			anchor := map[string]any{"blockHash": "0x" + strings.Repeat("4", 64), "requireCanonical": true}
			calls := 0
			tr := r.TransportFunc(func(_ context.Context, q r.Request) (r.Response, error) {
				body := q.Body.(map[string]any)
				params := body["params"].([]any)
				var result any
				switch body["method"] {
				case "eth_getLogs":
					query := params[0].(map[string]any)
					topic := query["topics"].([]any)[0].(string)
					topics := []string{topic, word(owner), word(spender)}
					contract := token
					if address, ok := query["address"].(string); ok {
						contract = address
						topics = []string{topic, word(owner), word(token), word(spender)}
					}
					result = []any{map[string]any{"address": contract, "topics": topics, "transactionHash": "fixture"}}
				case "eth_call":
					calls++
					b, _ := json.Marshal(params[1])
					want, _ := json.Marshal(anchor)
					if string(b) != string(want) {
						t.Fatal("permission read not block-pinned")
					}
					data := params[0].(map[string]any)["data"].(string)
					if strings.HasPrefix(data, selector("isApprovedForAll(address,address)")) {
						result = fmt.Sprintf("0x%064x", 1)
						if invalid {
							result = fmt.Sprintf("0x%064x", 2)
						}
					} else {
						result = fmt.Sprintf("0x%064x%064x%064x", 42, 999, 7)
						if invalid {
							result = "0x" + strings.Repeat("f", 192)
						}
					}
				default:
					t.Fatalf("unexpected RPC %v", body)
				}
				raw, _ := json.Marshal(map[string]any{"result": result})
				return r.Response{Raw: raw, Host: "rpc.fixture.invalid"}, nil
			})
			adapter, _ := New(r.Target{Network: "ethereum-mainnet", Kind: "wallet", Raw: owner}, tr)
			a := adapter.(*Adapter)
			limits := []r.Limitation{}
			facts := map[string]map[string]any{}
			a.extraPermissions(context.Background(), 1, "0x200", anchor, func(relation string, _ r.Response, m map[string]any) { facts[relation] = m }, &limits)
			if calls != 2 {
				t.Fatalf("current state reads=%d", calls)
			}
			if invalid {
				if len(facts) != 0 {
					t.Fatal("noncanonical ABI promoted to evidence")
				}
				return
			}
			if facts["evm-nft_operator"]["approved_for_all"] != true || facts["evm-permit2"]["allowance"] != "42" || facts["evm-permit2"]["token"] != token {
				t.Fatalf("wrong reconciled facts: %#v", facts)
			}
		})
	}
}
