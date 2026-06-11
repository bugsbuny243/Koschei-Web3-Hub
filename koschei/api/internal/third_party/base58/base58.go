package base58

import (
	"fmt"
	"math/big"
	"strings"
)

const bitcoinAlphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var bigRadix = big.NewInt(58)

// Decode decodes a Bitcoin/Solana alphabet base58 string into bytes.
func Decode(str string) ([]byte, error) {
	str = strings.TrimSpace(str)
	if str == "" {
		return []byte{}, nil
	}

	result := big.NewInt(0)
	for _, r := range str {
		idx := strings.IndexRune(bitcoinAlphabet, r)
		if idx < 0 {
			return nil, fmt.Errorf("invalid base58 character %q", r)
		}
		result.Mul(result, bigRadix)
		result.Add(result, big.NewInt(int64(idx)))
	}

	decoded := result.Bytes()
	leadingZeros := 0
	for leadingZeros < len(str) && str[leadingZeros] == bitcoinAlphabet[0] {
		leadingZeros++
	}
	if leadingZeros == 0 {
		return decoded, nil
	}
	out := make([]byte, leadingZeros+len(decoded))
	copy(out[leadingZeros:], decoded)
	return out, nil
}
