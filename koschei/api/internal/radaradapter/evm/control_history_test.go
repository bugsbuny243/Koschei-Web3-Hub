package evm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	r "koschei/api/internal/radaradapter"
	"strings"
	"testing"
)

func TestControlHistoryPinsBothSnapshotsAndFailsClosed(t *testing.T) {
	for _, scenario := range []string{"changed", "equal", "archive_missing", "bad_storage", "wrong_height", "bad_hash", "reorg_old", "reorg_head", "bad_code", "missing_current_admin"} {
		t.Run(scenario, func(t *testing.T) {
			head := blockIdentity{"0x300", "0x" + strings.Repeat("a", 64)}
			old := blockIdentity{"0x100", "0x" + strings.Repeat("b", 64)}
			subject := "0x" + strings.Repeat("1", 40)
			before := "0x" + strings.Repeat("2", 40)
			after := "0x" + strings.Repeat("3", 40)
			if scenario == "equal" {
				before = after
			}
			current := map[string]controlSnapshot{}
			for _, name := range []string{"implementation", "admin", "beacon"} {
				current[name] = controlSnapshot{after, r.Response{Raw: []byte("fixture-current-" + name), Host: "rpc.fixture.invalid"}}
			}
			if scenario == "missing_current_admin" {
				delete(current, "admin")
			}
			historicalHeaders := 0
			var requests []string
			tr := r.TransportFunc(func(_ context.Context, q r.Request) (r.Response, error) {
				body := q.Body.(map[string]any)
				method := body["method"].(string)
				params := body["params"].([]any)
				requests = append(requests, method)
				var result any
				switch method {
				case "eth_getBlockByNumber":
					number := params[0].(string)
					if number == old.Number {
						historicalHeaders++
						v := old
						if scenario == "wrong_height" {
							v.Number = "0x101"
						}
						if scenario == "bad_hash" {
							v.Hash = "0x" + strings.Repeat("z", 64)
						}
						if scenario == "reorg_old" && historicalHeaders > 1 {
							v.Hash = head.Hash
						}
						result = v
					} else if number == head.Number {
						v := head
						if scenario == "reorg_head" {
							v.Hash = old.Hash
						}
						result = v
					} else {
						t.Fatalf("unexpected/unpinned block query: %v", params)
					}
				case "eth_getStorageAt":
					if params[0] != subject {
						t.Fatal("wrong storage subject")
					}
					anchor := params[2].(map[string]any)
					if anchor["blockHash"] != old.Hash || anchor["requireCanonical"] != true {
						t.Fatalf("unpinned archive query: %v", params)
					}
					if scenario == "archive_missing" {
						return r.Response{}, errors.New("fixture pruned state")
					}
					result = "0x" + strings.Repeat("0", 24) + before[2:]
					if scenario == "bad_storage" {
						result = strings.TrimPrefix(result.(string), "0x")
					}
				case "eth_getCode":
					anchor := params[1].(map[string]any)
					if anchor["requireCanonical"] != true {
						t.Fatal("canonical code required")
					}
					switch anchor["blockHash"] {
					case old.Hash:
						if params[0] != before {
							t.Fatal("wrong previous implementation")
						}
						result = "0x6001"
					case head.Hash:
						if params[0] != after {
							t.Fatal("wrong current implementation")
						}
						result = "0x6002"
						if scenario == "equal" {
							result = "0x6001"
						}
					default:
						t.Fatal("unpinned code query")
					}
					if scenario == "bad_code" {
						result = "6001"
					}
				default:
					t.Fatalf("unexpected request: %s", method)
				}
				raw, _ := json.Marshal(map[string]any{"result": result})
				return r.Response{Raw: raw, Host: "rpc.fixture.invalid"}, nil
			})
			adapter, _ := New(r.Target{Network: "ethereum-mainnet", Kind: "contract", Raw: subject}, tr)
			a := adapter.(*Adapter)
			rows := map[string]map[string]any{}
			limits := []r.Limitation{}
			a.controlHistory(context.Background(), head, 768, current, func(rel string, _ r.Response, facts map[string]any) { rows[rel] = facts }, &limits)
			if len(limits) == 0 || limits[0].Code != "bounded_control_history" {
				t.Fatal("missing scope limitation")
			}
			switch scenario {
			case "archive_missing", "bad_storage", "wrong_height", "bad_hash", "reorg_old", "reorg_head":
				if len(rows) != 0 || len(limits) < 2 {
					t.Fatalf("failed snapshot promoted: %v %v", rows, limits)
				}
			default:
				want := 3
				if scenario == "missing_current_admin" {
					want = 2
				}
				if len(rows) != want {
					t.Fatalf("rows=%d want %d", len(rows), want)
				}
				facts := rows["evm-control-history-implementation"]
				if facts["changed_between_snapshots"] != (scenario != "equal") {
					t.Fatalf("wrong change flag: %v", facts)
				}
				if facts["from_block"] != old.Number || facts["from_block_hash"] != old.Hash || facts["intermediate_changes"] != "unknown" || facts["upgrade_transaction"] != "unknown" {
					t.Fatalf("incorrect scope: %v", facts)
				}
				if scenario == "bad_code" {
					if _, exists := facts["code_changed_between_snapshots"]; exists {
						t.Fatal("malformed code promoted")
					}
				} else if facts["code_changed_between_snapshots"] != (scenario != "equal") {
					t.Fatal("wrong code change flag")
				}
				checks, ok := facts["endpoint_recheck_response_sha256"].(map[string]string)
				if !ok || len(checks[old.Number]) != 64 || len(checks[head.Number]) != 64 {
					t.Fatal("endpoint provenance missing")
				}
			}
			if len(requests) > 8 {
				t.Fatalf("history exceeded bounded request budget: %d", len(requests))
			}
		})
	}
}

func TestHistoricalBlockIdentityRejectsAmbiguousEncodings(t *testing.T) {
	for _, number := range []string{"", "100", "0x01", "0x-1", "0x10000000000000000"} {
		raw := json.RawMessage(fmt.Sprintf(`{"number":%q,"hash":%q}`, number, "0x"+strings.Repeat("a", 64)))
		if _, _, err := parseBlockIdentity(raw); err == nil {
			t.Fatalf("accepted ambiguous height %q", number)
		}
	}
}

func TestControlHistoryGenesisDoesNotInventEarlierState(t *testing.T) {
	adapter, _ := New(r.Target{Network: "ethereum-mainnet", Kind: "contract", Raw: "0x" + strings.Repeat("1", 40)}, r.TransportFunc(func(context.Context, r.Request) (r.Response, error) {
		t.Fatal("genesis must not query earlier state")
		return r.Response{}, nil
	}))
	limits := []r.Limitation{}
	adapter.(*Adapter).controlHistory(context.Background(), blockIdentity{}, 0, map[string]controlSnapshot{"admin": {}}, func(string, r.Response, map[string]any) { t.Fatal("unexpected history") }, &limits)
}

// Exercises the public collector path on every registered EVM chain, including
// subject/provenance validation; helper-only tests cannot establish this wiring.
func TestCollectIncludesBoundControlHistoryAcrossEVMChains(t *testing.T) {
	for network, chainID := range Chains {
		t.Run(network, func(t *testing.T) {
			head := blockIdentity{"0x300", "0x" + strings.Repeat("a", 64)}
			old := blockIdentity{"0x100", "0x" + strings.Repeat("b", 64)}
			target := r.Target{Network: network, Kind: "contract", Raw: "0x" + strings.Repeat("1", 40)}
			tr := r.TransportFunc(func(_ context.Context, q r.Request) (r.Response, error) {
				body := q.Body.(map[string]any)
				params := body["params"].([]any)
				var result any
				switch body["method"] {
				case "eth_chainId":
					result = fmt.Sprintf("0x%x", chainID)
				case "eth_getBlockByNumber":
					switch params[0] {
					case "latest", head.Number:
						result = head
					case old.Number:
						result = old
					default:
						t.Fatalf("unexpected height %v", params[0])
					}
				case "eth_getCode":
					result = "0x6001"
				case "eth_getStorageAt":
					anchor := params[2].(map[string]any)
					address := "0x" + strings.Repeat("2", 40)
					if anchor["blockHash"] == old.Hash {
						address = "0x" + strings.Repeat("3", 40)
					}
					result = "0x" + strings.Repeat("0", 24) + address[2:]
				case "eth_call":
					return r.Response{}, errors.New("fixture has no optional interfaces")
				case "eth_getLogs":
					result = []any{}
				default:
					t.Fatalf("unexpected method %v", body["method"])
				}
				raw, _ := json.Marshal(map[string]any{"result": result})
				return r.Response{Raw: raw, Host: "rpc.fixture.invalid"}, nil
			})
			adapter, err := New(target, tr)
			if err != nil {
				t.Fatal(err)
			}
			rows, _, err := adapter.Collect(context.Background(), target)
			if err != nil {
				t.Fatal(err)
			}
			if err = r.ValidateEvidence(adapter.Subject(), rows); err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, row := range rows {
				if !strings.HasPrefix(row.Relation, "evm-control-history-") {
					continue
				}
				count++
				if row.SubjectID != target.Raw || row.Network != network || row.Height != "768" || row.Attributes["block_hash"] != head.Hash || row.Status != "observed" {
					t.Fatalf("unbound history: %+v", row)
				}
				if row.Attributes["changed_between_snapshots"] != true {
					t.Fatal("expected observed endpoint change")
				}
			}
			if count != 3 {
				t.Fatalf("collector omitted control history: %d", count)
			}
		})
	}
}
