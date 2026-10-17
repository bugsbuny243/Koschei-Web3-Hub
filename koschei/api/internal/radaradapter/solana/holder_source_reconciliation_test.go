package solana

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const holderFixtureMint = "11111111111111111111111111111111"
const holderFixtureTokenAccount = "So11111111111111111111111111111111111111112"
const holderFixtureAuthority = "Vote111111111111111111111111111111111111111"

func fixtureHolderJSON(t *testing.T, data any) []byte {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func fixtureHolderSnapshot(raw []byte) HolderRawRPCSnapshot {
	sum := sha256.Sum256(raw)
	return HolderRawRPCSnapshot{RawJSON: raw, CaptureSHA256: hex.EncodeToString(sum[:])}
}

func fixtureHolderSources(t *testing.T, slots [3]uint64) HolderRawSourceSet {
	t.Helper()
	rpc := func(slot uint64, value any) HolderRawRPCSnapshot {
		return fixtureHolderSnapshot(fixtureHolderJSON(t, map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"context": map[string]any{"slot": slot}, "value": value},
		}))
	}
	return HolderRawSourceSet{
		Mint: holderFixtureMint,
		CapturedAt: time.Date(2026, time.October, 17, 12, 0, 0, 0, time.UTC),
		Supply: rpc(slots[0], map[string]any{
			"amount": "1000", "decimals": 6, "private_secret": "never-expose-me",
		}),
		Largest: rpc(slots[1], []any{
			map[string]any{"address": holderFixtureTokenAccount, "amount": "900", "decimals": 6},
		}),
		Accounts: rpc(slots[2], []any{
			map[string]any{
				"owner": splTokenProgramID,
				"data": map[string]any{
					"program": "spl-token",
					"parsed": map[string]any{
						"type": "account",
						"info": map[string]any{
							"mint": holderFixtureMint, "owner": holderFixtureAuthority,
							"tokenAmount": map[string]any{"amount": "900", "decimals": 6},
						},
					},
				},
			},
		}),
	}
}

func fixtureHolderRewrite(t *testing.T, in *HolderRawRPCSnapshot, old, next string) {
	t.Helper()
	if !strings.Contains(string(in.RawJSON), old) {
		t.Fatalf("test token %q not in source %s", old, in.RawJSON)
	}
	*in = fixtureHolderSnapshot([]byte(strings.Replace(string(in.RawJSON), old, next, 1)))
}

func TestReconcileHolderSourcesPreservesDistinctSlotsAndOwnerAuthority(t *testing.T) {
	in := fixtureHolderSources(t, [3]uint64{101, 102, 103})
	got, err := ReconcileHolderRawSources(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != holderSourceStatus || got.RawSupplyAmount != "1000" || got.Decimals != 6 ||
		got.SupplySlot != 101 || got.LargestAccountsSlot != 102 || got.TokenAccountsSlot != 103 ||
		got.SameReportedSlot || got.AtomicSnapshotVerified || got.OwnerIdentityVerified ||
		got.ChainEvidenceVerified || got.VerdictBound {
		t.Fatalf("three-source observation was given unjustified authority: %#v", got)
	}
	if len(got.TokenAccounts) != 1 || got.TokenAccounts[0].TokenAccount != holderFixtureTokenAccount ||
		got.TokenAccounts[0].OwnerAuthority != holderFixtureAuthority ||
		got.TokenAccounts[0].TokenAccount == got.TokenAccounts[0].OwnerAuthority ||
		got.TokenAccounts[0].TokenProgram != splTokenProgramID || got.TokenAccounts[0].RawAmount != "900" {
		t.Fatalf("token account was confused with controlling authority: %#v", got.TokenAccounts)
	}
	if len(got.SourceWitnesses) != 3 || len(got.Limitations) < 3 {
		t.Fatalf("missing source witness and non-atomic boundaries: %#v", got)
	}
	for i, witness := range got.SourceWitnesses {
		if len(witness.ResponseSHA256) != 64 || len(witness.CommitmentSHA256) != 64 ||
			witness.ReportedHeight != []string{"101", "102", "103"}[i] ||
			witness.ChainEvidenceVerified || witness.VerdictBound ||
			witness.AttributesIndependentlyProven || witness.HeightIndependentlyVerified {
			t.Fatalf("source %d promoted chain authority: %#v", i, witness)
		}
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"never-expose-me", "private_secret", "jsonrpc", "caller_captured_solana_json_rpc"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("raw source material escaped reconciliation: %s", encoded)
		}
	}
	again, err := ReconcileHolderRawSources(in)
	if err != nil || again.SourceWitnesses[0].CommitmentSHA256 != got.SourceWitnesses[0].CommitmentSHA256 {
		t.Fatalf("source commitment not deterministic: %#v %v", again, err)
	}
	in = fixtureHolderSources(t, [3]uint64{101, 101, 101})
	equal, err := ReconcileHolderRawSources(in)
	if err != nil || !equal.SameReportedSlot || equal.AtomicSnapshotVerified {
		t.Fatalf("same reported slot was mistaken for atomicity: %#v %v", equal, err)
	}
}

func TestReconcileHolderSourcesFailsClosedOnConflicts(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*testing.T, *HolderRawSourceSet)
	}{
		{"captured bytes changed", func(t *testing.T, x *HolderRawSourceSet) {
			x.Supply.RawJSON = append(x.Supply.RawJSON, ' ')
		}},
		{"bad captured digest", func(t *testing.T, x *HolderRawSourceSet) {
			x.Largest.CaptureSHA256 = strings.Repeat("a", 64)
		}},
		{"wrong mint", func(_ *testing.T, x *HolderRawSourceSet) {
			x.Mint = holderFixtureTokenAccount
		}},
		{"zero capture timestamp", func(_ *testing.T, x *HolderRawSourceSet) {
			x.CapturedAt = time.Time{}
		}},
		{"duplicate slot key", func(t *testing.T, x *HolderRawSourceSet) {
			fixtureHolderRewrite(t, &x.Supply, `"slot":101`, `"slot":101,"slot":102`)
		}},
		{"stale slot", func(t *testing.T, x *HolderRawSourceSet) {
			fixtureHolderRewrite(t, &x.Supply, `"slot":101`, `"slot":0`)
		}},
		{"largest amount mismatch", func(t *testing.T, x *HolderRawSourceSet) {
			fixtureHolderRewrite(t, &x.Largest, `"amount":"900"`, `"amount":"901"`)
		}},
		{"non-string amount", func(t *testing.T, x *HolderRawSourceSet) {
			fixtureHolderRewrite(t, &x.Largest, `"amount":"900"`, `"amount":900`)
		}},
		{"token account wrong mint", func(t *testing.T, x *HolderRawSourceSet) {
			fixtureHolderRewrite(t, &x.Accounts, `"mint":"`+holderFixtureMint+`"`, `"mint":"`+holderFixtureTokenAccount+`"`)
		}},
		{"token program mismatch", func(t *testing.T, x *HolderRawSourceSet) {
			fixtureHolderRewrite(t, &x.Accounts, `"program":"spl-token"`, `"program":"spl-token-2022"`)
		}},
		{"unknown executable owner", func(t *testing.T, x *HolderRawSourceSet) {
			fixtureHolderRewrite(t, &x.Accounts, `"owner":"`+splTokenProgramID+`"`, `"owner":"`+holderFixtureMint+`"`)
		}},
		{"owner authority absent", func(t *testing.T, x *HolderRawSourceSet) {
			fixtureHolderRewrite(t, &x.Accounts, `"owner":"`+holderFixtureAuthority+`"`, `"owner":""`)
		}},
		{"duplicate amount", func(t *testing.T, x *HolderRawSourceSet) {
			fixtureHolderRewrite(t, &x.Largest, `"amount":"900"`, `"amount":"900","amount":"900"`)
		}},
		{"duplicate parsed mint", func(t *testing.T, x *HolderRawSourceSet) {
			fixtureHolderRewrite(t, &x.Accounts, `"mint":"`+holderFixtureMint+`"`, `"mint":"`+holderFixtureMint+`","mint":"`+holderFixtureMint+`"`)
		}},
		{"null account", func(t *testing.T, x *HolderRawSourceSet) {
			fixtureHolderRewrite(t, &x.Accounts, `"value":[{`, `"value":[null,{`)
		}},
		{"missing largest entries", func(t *testing.T, x *HolderRawSourceSet) {
			fixtureHolderRewrite(t, &x.Largest, `"value":[{`, `"value":[],"ignored":[{`)
		}},
		{"token account different decimals", func(t *testing.T, x *HolderRawSourceSet) {
			fixtureHolderRewrite(t, &x.Accounts, `"decimals":6`, `"decimals":5`)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := fixtureHolderSources(t, [3]uint64{101, 102, 103})
			tc.edit(t, &in)
			out, err := ReconcileHolderRawSources(in)
			if err == nil || out.Status != "" || len(out.TokenAccounts) != 0 || len(out.SourceWitnesses) != 0 {
				t.Fatalf("invalid holder source was not rejected: %#v error=%v", out, err)
			}
		})
	}
}
