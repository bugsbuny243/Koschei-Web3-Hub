package networktarget

import "testing"

func TestExpectedEVMChainIDExpandedMainnets(t *testing.T) {
	want := map[string]string{
		"ethereum-mainnet":  "0x1",
		"base-mainnet":      "0x2105",
		"arbitrum-mainnet":  "0xa4b1",
		"optimism-mainnet":  "0xa",
		"polygon-mainnet":   "0x89",
		"bnb-mainnet":       "0x38",
		"avalanche-mainnet": "0xa86a",
	}
	for network, expected := range want {
		got, ok := ExpectedEVMChainID(network)
		if !ok || got != expected {
			t.Fatalf("%s chain id=%q ok=%v want=%q", network, got, ok, expected)
		}
	}
	if _, ok := ExpectedEVMChainID("made-up-mainnet"); ok {
		t.Fatal("unknown EVM network unexpectedly received a chain id")
	}
}
