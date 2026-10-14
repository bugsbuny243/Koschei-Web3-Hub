package tron

import (
	"context"
	r "koschei/api/internal/radaradapter"
	"os"
	"testing"
)

func TestAddressAndPermissions(t *testing.T) {
	self, e := Address("T9yD14Nj9j7xAB4dbGeiX9h8unkKHxuWwb")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Address("T9yD14Nj9j7xAB4dbGeiX9h8unkKHxuWwa"); e == nil {
		t.Fatal("checksum accepted")
	}
	tests := []struct {
		p              Permission
		external, self bool
	}{{Permission{ID: 0, Threshold: 1, Keys: []Key{{self, 1}}}, false, true}, {Permission{ID: 0, Threshold: 2, Keys: []Key{{self, 1}, {"410000000000000000000000000000000000000001", 1}}}, false, false}, {Permission{ID: 0, Threshold: 1, Keys: []Key{{"410000000000000000000000000000000000000001", 1}}}, true, false}}
	for _, tc := range tests {
		c, e := AnalyzePermission(tc.p, self)
		if e != nil || c.ExternalThreshold != tc.external || c.SelfThreshold != tc.self {
			t.Fatalf("control=%+v err=%v", c, e)
		}
	}
}
func TestCollectOfflineConformance(t *testing.T) {
	account, e := os.ReadFile("testdata/account.json")
	if e != nil {
		t.Fatal(e)
	}
	tr := r.TransportFunc(func(_ context.Context, q r.Request) (r.Response, error) {
		b := account
		switch q.Path {
		case "/wallet/getblockbynum":
			b = []byte(`{"blockID":"` + MainnetGenesis + `"}`)
		case "/walletsolidity/getnowblock":
			b = []byte(`{"blockID":"head","block_header":{"raw_data":{"number":100}}}`)
		}
		return r.Response{Raw: b, Host: q.Provider + ".fixture.example"}, nil
	})
	target := r.Target{Network: "tron-mainnet", Kind: "address", Raw: "T9yD14Nj9j7xAB4dbGeiX9h8unkKHxuWwb"}
	a, e := New(target, tr)
	if e != nil {
		t.Fatal(e)
	}
	if e = r.Conform(context.Background(), a, target); e != nil {
		t.Fatal(e)
	}
	rows, _, e := a.Collect(context.Background(), target)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, row := range rows {
		if row.Relation == "tron-external-permission-control" && row.Status == "verified" {
			found = true
		}
	}
	if !found {
		t.Fatal("external permission not confirmed")
	}
}
