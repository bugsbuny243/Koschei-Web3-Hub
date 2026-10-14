package cosmos

import "testing"

func TestBech32RejectsWrongChainAndChecksum(t *testing.T) {
	s := "cosmos1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqnrql8a"
	if e := Address(s, "cosmos"); e != nil {
		t.Fatal(e)
	}
	if Address(s, "osmo") == nil || Address(s[:len(s)-1]+"q", "cosmos") == nil {
		t.Fatal("invalid identity accepted")
	}
}
