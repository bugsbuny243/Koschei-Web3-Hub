package solana

import (
	"math/big"
)

// EncodeSolanaPubkey returns the canonical base58 representation of a 32-byte key.
func EncodeSolanaPubkey(key [32]byte) string {
	zeros := 0
	for zeros < len(key) && key[zeros] == 0 {
		zeros++
	}
	value := new(big.Int).SetBytes(key[:])
	base := big.NewInt(58)
	mod := new(big.Int)
	digits := make([]byte, 0, 44)
	for value.Sign() > 0 {
		value.DivMod(value, base, mod)
		digits = append(digits, solanaBase58[int(mod.Int64())])
	}
	for i := 0; i < zeros; i++ {
		digits = append(digits, '1')
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
