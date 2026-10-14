package builtin

import (
	"context"
	r "koschei/api/internal/radaradapter"
	"koschei/api/internal/radaradapter/evm"
	"strings"
	"testing"
)

func TestEveryRegisteredNetworkPassesOfflineConformance(t *testing.T) {
	registry, failures := Registry()
	if len(failures) > 0 {
		t.Fatal(failures)
	}
	if len(registry.Capabilities()) != 24 {
		t.Fatalf("got %d capabilities", len(registry.Capabilities()))
	}
}
func TestEVMDelegationNeverPromotesGradeAndNoCodeDoesNotTrigger(t *testing.T) {
	for _, delegated := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "delegated"}[delegated], func(t *testing.T) {
			base := FixtureTransport("evm", "ethereum-mainnet")
			calls := 0
			transport := r.TransportFunc(func(c context.Context, q r.Request) (r.Response, error) {
				body, _ := q.Body.(map[string]any)
				if body["method"] == "eth_getCode" {
					calls++
					if delegated && calls == 1 {
						return r.Response{Raw: []byte(`{"result":"0xef0100` + strings.Repeat("a", 40) + `"}`), Host: "primary.fixture.invalid"}, nil
					}
				}
				return base.Do(c, q)
			})
			target := r.Target{Network: "ethereum-mainnet", Kind: "address", Raw: "0x" + strings.Repeat("0", 40)}
			a, e := evm.New(target, transport)
			if e != nil {
				t.Fatal(e)
			}
			rows, _, e := a.Collect(context.Background(), target)
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, row := range rows {
				if row.Status != "observed" {
					t.Fatal("EVM promotion")
				}
				if row.Relation == "evm-7702-delegation" {
					found = true
				}
			}
			if found != delegated {
				t.Fatal("incorrect delegation")
			}
		})
	}
}
