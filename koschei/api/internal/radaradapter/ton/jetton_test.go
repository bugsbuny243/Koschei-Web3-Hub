package ton

import (
	"context"
	"encoding/json"
	r "koschei/api/internal/radaradapter"
	"strings"
	"testing"
)

type jettonFixture struct {
	r.Transport
	allowed bool
}

func (f jettonFixture) CanonicalJetton(string) bool { return f.allowed }
func TestJettonDerivationBindsOwnerMasterAndWallet(t *testing.T) {
	owner := "0:" + strings.Repeat("1", 64)
	master := "0:" + strings.Repeat("2", 64)
	wallet := "0:" + strings.Repeat("3", 64)
	addressEntry := func(a string) []any {
		b, e := ownerSlice(a)
		if e != nil {
			t.Fatal(e)
		}
		return []any{"cell", map[string]any{"bytes": b}}
	}
	for _, bad := range []string{"none", "master", "derived", "unpinned"} {
		t.Run(bad, func(t *testing.T) {
			tr := jettonFixture{allowed: bad != "unpinned", Transport: r.TransportFunc(func(_ context.Context, q r.Request) (r.Response, error) {
				body := q.Body.(map[string]any)
				var stack []any
				if body["method"] == "get_wallet_address" {
					a := wallet
					if bad == "derived" {
						a = owner
					}
					stack = []any{addressEntry(a)}
				} else {
					m := master
					if bad == "master" {
						m = owner
					}
					stack = []any{[]string{"num", "0x2a"}, addressEntry(owner), addressEntry(m), addressEntry(owner)}
				}
				bytes, _ := json.Marshal(map[string]any{"ok": true, "result": map[string]any{"exit_code": 0, "stack": stack}})
				return r.Response{Raw: bytes, Host: "ton.fixture.invalid"}, nil
			})}
			target := r.Target{Network: "ton-mainnet", Kind: "jetton_wallet", Raw: owner + "|" + master + "|" + wallet}
			adapter, e := New(target, tr)
			if e != nil {
				t.Fatal(e)
			}
			a := adapter.(*Adapter)
			a.subject.ChainIdentity = "global_id:-239"
			rows, _, e := a.jettonWallet(context.Background(), nil, "123")
			if bad != "none" {
				if e == nil || len(rows) > 0 {
					t.Fatal("unbound balance accepted")
				}
				return
			}
			if e != nil || len(rows) != 1 || rows[0].Attributes["balance_raw"] != "42" {
				t.Fatalf("rows=%+v err=%v", rows, e)
			}
		})
	}
}
func TestCodeHashUsesCellGraph(t *testing.T) {
	boc, e := ownerSlice("0:" + strings.Repeat("1", 64))
	if e != nil {
		t.Fatal(e)
	}
	hash, e := CodeHash(boc)
	if e != nil || len(hash) != 64 || hash == r.Digest([]byte(boc)) {
		t.Fatal(hash, e)
	}
}
