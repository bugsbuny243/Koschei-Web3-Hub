package networktarget

import "strings"

// ExpectedBitcoinGenesisHash returns the trust anchor used to confirm that a
// configured read-only Bitcoin backend belongs to the requested network.
func ExpectedBitcoinGenesisHash(networkID string) (string, bool) {
	if strings.TrimSpace(networkID) != "bitcoin-mainnet" {
		return "", false
	}
	return bitcoinMainnetGenesisHash, true
}
