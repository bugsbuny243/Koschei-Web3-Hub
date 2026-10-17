package solana

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const (
	holderFixtureMint = "So11111111111111111111111111111111111111112"
	holderFixtureTokenAccount = "11111111111111111111111111111111"
	holderFixtureOwnerWallet = "ComputeBudget111111111111111111111111111111"
)

func holderRawFixture() HolderRawSourceSet {
	supply := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":{"context":{"slot":20},"value":{"amount":"100","decimals":0}},"private":"fixture-secret"}`)
	largest := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":{"context":{"slot":20},"value":[{"address":%q,"amount":"12","decimals":0}]}}`, holderFixtureTokenAccount)
	accounts := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":{"context":{"slot":20},"value":[{"owner":%q,"data":{"program":"spl-token","parsed":{"type":"account","info":{"mint":%q,"owner":%q,"tokenAmount":{"amount":"12","decimals":0}}}}}]}}`, holderSPLTokenProgramID, holderFixtureMint, holderFixtureOwnerWallet)
	return HolderRawSourceSet{Mint: holderFixtureMint, SupplyResponse: []byte(supply), LargestResponse: []byte(largest), AccountsResponse: []byte(accounts), RequestedTokenAccounts: []string{holderFixtureTokenAccount}}
}

func TestCheckHolderRawSourcesExactSameSlotObservedOnly(t *testing.T) {
	in := holderRawFixture()
	out, err := CheckHolderRawSources(in)
	if err != nil { t.Fatal(err) }
	if out.Status != "observed_same_reported_slot_consistent_not_independent_proof" ||
		out.SupplySlot != 20 || out.LargestSlot != 20 || out.AccountsSlot != 20 ||
		!out.SameReportedSlot || out.SupplyAmountRaw != "100" || out.LargestAccountsSumRaw != "12" ||
		len(out.Accounts) != 1 || out.Accounts[0].TokenAccount != holderFixtureTokenAccount ||
		out.Accounts[0].OwnerWallet != holderFixtureOwnerWallet ||
		out.Accounts[0].TokenProgram != holderSPLTokenProgramID ||
		out.ChainEvidenceVerified || out.VerdictBound || out.AtomicSnapshotProven {
		t.Fatalf("holder source authority or account role promoted: %#v", out)
	}
	if len(out.SupplyResponseSHA256) != 64 || len(out.LargestResponseSHA256) != 64 ||
		len(out.AccountsResponseSHA256) != 64 {
		t.Fatalf("raw source digests missing: %#v", out)
	}
	body, err := json.Marshal(out)
	if err != nil { t.Fatal(err) }
	if strings.Contains(string(body), "fixture-secret") || strings.Contains(string(body), "private") ||
		strings.Contains(string(body), "safe") || strings.Contains(string(body), "risk_grade") {
		t.Fatalf("source secret or safety verdict escaped: %s", body)
	}
	again, err := CheckHolderRawSources(in)
	if err != nil || again.SupplyResponseSHA256 != out.SupplyResponseSHA256 {
		t.Fatalf("raw source hash is not reproducible: %#v %v", again, err)
	}
}

func TestCheckHolderRawSourcesDifferentSlotsNeverReconciled(t *testing.T) {
	in := holderRawFixture()
	in.AccountsResponse = []byte(strings.ReplaceAll(string(in.AccountsResponse), `"slot":20`, `"slot":21`))
	in.AccountsResponse = []byte(strings.ReplaceAll(string(in.AccountsResponse), `"amount":"12"`, `"amount":"11"`))
	out, err := CheckHolderRawSources(in)
	if err != nil || out.Status != "observed_non_atomic_slot_set" || out.SameReportedSlot ||
		out.AtomicSnapshotProven || out.ChainEvidenceVerified ||
		len(out.Accounts) != 1 || out.Accounts[0].LargestAmountRaw != "12" ||
		out.Accounts[0].ResolvedAmountRaw != "11" {
		t.Fatalf("different-slot amounts falsely reconciled: %#v %v", out, err)
	}
}

func TestCheckHolderRawSourcesRejectsBrokenProvenance(t *testing.T) {
	for _, tc := range []struct {
		name string
		alter func(*HolderRawSourceSet)
	}{
		{"missing supply",func(s *HolderRawSourceSet){s.SupplyResponse=nil}},
		{"no RPC result",func(s *HolderRawSourceSet){s.SupplyResponse=[]byte(`{"jsonrpc":"2.0","id":1}`)}},
		{"RPC error with result",func(s *HolderRawSourceSet){s.SupplyResponse=[]byte(strings.Replace(string(s.SupplyResponse),`,"private":`,`,"error":{"code":-32000},"private":`,1))}},
		{"duplicate result",func(s *HolderRawSourceSet){s.SupplyResponse=[]byte(`{"result":{"context":{"slot":20},"value":{"amount":"100","decimals":0}},"result":{"context":{"slot":20},"value":{"amount":"101","decimals":0}}}`)}},
		{"zero context",func(s *HolderRawSourceSet){s.SupplyResponse=[]byte(strings.Replace(string(s.SupplyResponse),`"slot":20`,`"slot":0`,1))}},
		{"different mint",func(s *HolderRawSourceSet){s.AccountsResponse=[]byte(strings.ReplaceAll(string(s.AccountsResponse),holderFixtureMint,holderFixtureOwnerWallet))}},
		{"wrong account program",func(s *HolderRawSourceSet){s.AccountsResponse=[]byte(strings.ReplaceAll(string(s.AccountsResponse),holderSPLTokenProgramID,holderFixtureOwnerWallet))}},
		{"incorrect parser program",func(s *HolderRawSourceSet){s.AccountsResponse=[]byte(strings.ReplaceAll(string(s.AccountsResponse),`"program":"spl-token"`,`"program":"spl-token-2022"`))}},
		{"missing owner wallet",func(s *HolderRawSourceSet){s.AccountsResponse=[]byte(strings.ReplaceAll(string(s.AccountsResponse),holderFixtureOwnerWallet,""))}},
		{"noninteger balance",func(s *HolderRawSourceSet){s.LargestResponse=[]byte(strings.ReplaceAll(string(s.LargestResponse),`"amount":"12"`,`"amount":"12.5"`))}},
		{"uint64 overflow",func(s *HolderRawSourceSet){s.SupplyResponse=[]byte(strings.ReplaceAll(string(s.SupplyResponse),`"amount":"100"`,`"amount":"18446744073709551616"`))}},
		{"incorrect decimals",func(s *HolderRawSourceSet){s.AccountsResponse=[]byte(strings.ReplaceAll(string(s.AccountsResponse),`"decimals":0`,`"decimals":6`))}},
		{"stale resolved amount at same slot",func(s *HolderRawSourceSet){s.AccountsResponse=[]byte(strings.ReplaceAll(string(s.AccountsResponse),`"amount":"12"`,`"amount":"11"`))}},
		{"top sum exceeds same-slot supply",func(s *HolderRawSourceSet){s.SupplyResponse=[]byte(strings.ReplaceAll(string(s.SupplyResponse),`"amount":"100"`,`"amount":"10"`))}},
		{"wrong request order",func(s *HolderRawSourceSet){s.RequestedTokenAccounts=[]string{holderFixtureOwnerWallet}}},
		{"missing account",func(s *HolderRawSourceSet){s.AccountsResponse=[]byte(strings.ReplaceAll(string(s.AccountsResponse),`"value":[{"owner"`,`"value":[null,{"owner"`))}},
		{"empty request list",func(s *HolderRawSourceSet){s.RequestedTokenAccounts=nil}},
		{"duplicate checked field",func(s *HolderRawSourceSet){s.LargestResponse=[]byte(strings.ReplaceAll(string(s.LargestResponse),`"amount":"12"`,`"amount":"12","amount":"13"`))}},
	} {
		t.Run(tc.name,func(t *testing.T){
			in:=holderRawFixture()
			tc.alter(&in)
			out,err:=CheckHolderRawSources(in)
			if err==nil || out.Status!="" || len(out.Accounts)!=0 {t.Fatalf("broken holder proof accepted: %#v %v",out,err)}
		})
	}
}
