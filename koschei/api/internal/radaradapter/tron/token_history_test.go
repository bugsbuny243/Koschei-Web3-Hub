package tron

import (
	"encoding/json"
	r "koschei/api/internal/radaradapter"
	"strings"
	"testing"
)

func TestTokenHistoryBindsIdentityAndKeepsIntegerPrecision(t *testing.T) {
	self := "41" + strings.Repeat("0", 40)
	other := "41" + strings.Repeat("1", 40)
	a := &Adapter{hexAddress: self, subject: r.SubjectIdentity{Network: "tron-mainnet", NativeID: self, ChainIdentity: "genesis:" + MainnetGenesis}}
	for _, tc := range []struct {
		from, value, kind string
		want              int
	}{{self, "900719925474099312345", "Transfer", 1}, {other, "1", "Transfer", 0}, {self, "1.5", "Transfer", 0}, {self, "42", "Approval", 1}} {
		event := map[string]any{"transaction_id": strings.Repeat("a", 64), "from": tc.from, "to": other, "value": tc.value, "type": tc.kind, "block_timestamp": 1234567890, "token_info": map[string]any{"address": other, "decimals": 6}}
		raw, _ := json.Marshal(map[string]any{"success": true, "data": []any{event}})
		rows, _ := a.tokenHistoryRows(r.Response{Raw: raw, Host: "index.fixture.invalid"}, "100")
		if len(rows) != tc.want {
			t.Fatalf("rows=%d want=%d", len(rows), tc.want)
		}
		if len(rows) > 0 && (rows[0].Attributes["value_raw"] != tc.value || rows[0].Attributes["event_type"] != tc.kind || rows[0].Attributes["event_height_verified"] != false) {
			t.Fatal("precision or semantics lost")
		}
	}
}
