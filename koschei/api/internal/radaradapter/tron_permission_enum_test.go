package radaradapter

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestTronPermissionEnumWireForms(t *testing.T) {
	for _, tc := range []struct {
		name, slot, typ string
		id, want        int
		valid           bool
	}{
		{"owner name", "owner_permission", `"Owner"`, 0, 0, true},
		{"owner numeric", "owner_permission", `0`, 0, 0, true},
		{"witness name", "witness_permission", `"Witness"`, 1, 1, true},
		{"active name", "active_permission", `"Active"`, 2, 2, true},
		{"active numeric", "active_permission", `2`, 2, 2, true},
		{"unknown name", "owner_permission", `"Custodian"`, 0, 0, false},
		{"unknown numeric", "owner_permission", `3`, 0, 0, false},
		{"wrong container", "active_permission", `"Owner"`, 0, 0, false},
		{"fraction", "owner_permission", `0.5`, 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			permission := fmt.Sprintf(`{"id":%d,"type":%s,"threshold":1,"keys":[{"address":"41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","weight":1}]}`, tc.id, tc.typ)
			if tc.slot == "active_permission" {
				permission = "[" + permission + "]"
			}
			rows, err := ParseTronAccountPermissions([]byte(`{"` + tc.slot + `":` + permission + `}`))
			if !tc.valid {
				if err == nil {
					t.Fatal("malformed permission accepted")
				}
				return
			}
			if err != nil || len(rows) != 1 || rows[0].Type != tc.want {
				t.Fatalf("rows=%+v err=%v", rows, err)
			}
		})
	}
	// Proto3 may omit the zero-valued Owner enum and ID.
	if _, err := ParseTronAccountPermissions([]byte(`{"owner_permission":{"threshold":1,"keys":[{"address":"41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","weight":1}]}}`)); err != nil {
		t.Fatal(err)
	}
}

func TestTronEvidenceIDsSeparateObservationEvents(t *testing.T) {
	at := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	original := TronAccountObservation{Network: "tron-mainnet", Address: "41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Provider: "https://api.trongrid.io/wallet/getaccount", RawResponseSHA256: strings.Repeat("a", 64), Permissions: []TronPermissionFinding{{ID: "0", Threshold: 1, TotalWeight: 1, KeyCount: 1, ThresholdReachable: true}}}
	ids := map[string]bool{}
	for i := 0; i < 4; i++ {
		observation := original
		when := at
		switch i {
		case 1:
			observation.Address = "41bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		case 2:
			observation.RawResponseSHA256 = strings.Repeat("a", 63) + "b"
		case 3:
			when = at.Add(time.Nanosecond) // identical response, new observation
		}
		rows, err := observation.EvidenceRows(when, 123)
		if err != nil {
			t.Fatal(err)
		}
		if ids[rows[0].ID] {
			t.Fatalf("colliding observation %d: %s", i, rows[0].ID)
		}
		ids[rows[0].ID] = true
		repeated, err := observation.EvidenceRows(when, 123)
		if err != nil || repeated[0].ID != rows[0].ID {
			t.Fatal("same observation must be idempotent")
		}
	}
}
