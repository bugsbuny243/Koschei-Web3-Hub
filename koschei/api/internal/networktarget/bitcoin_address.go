package networktarget

import (
	"bytes"
	"crypto/sha256"
	"math/big"
	"strings"
)

const (
	bech32ChecksumConst  = uint32(1)
	bech32mChecksumConst = uint32(0x2bc830a3)
)

var bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

func bitcoinMainnetAddress(value string) (canonical, classification string, ok bool) {
	if canonical, ok := bitcoinLegacyMainnetAddress(value); ok {
		return canonical, "bitcoin_base58check_syntax_only", true
	}
	if canonical, witnessVersion, ok := bitcoinSegWitMainnetAddress(value); ok {
		return canonical, bitcoinWitnessClassification(witnessVersion), true
	}
	return "", "", false
}

func bitcoinWitnessClassification(version byte) string {
	if version == 0 {
		return "bitcoin_segwit_v0_bech32_syntax_only"
	}
	if version == 1 {
		return "bitcoin_segwit_v1_bech32m_syntax_only"
	}
	return "bitcoin_segwit_bech32m_syntax_only"
}

func bitcoinLegacyMainnetAddress(value string) (string, bool) {
	decoded, ok := decodeBase58(value)
	if !ok || len(decoded) != 25 {
		return "", false
	}
	if decoded[0] != 0x00 && decoded[0] != 0x05 {
		return "", false
	}
	first := sha256.Sum256(decoded[:21])
	second := sha256.Sum256(first[:])
	if !bytes.Equal(decoded[21:], second[:4]) {
		return "", false
	}
	return value, true
}

func decodeBase58(value string) ([]byte, bool) {
	if value == "" || len(value) > 64 {
		return nil, false
	}
	n := new(big.Int)
	base := big.NewInt(58)
	for _, ch := range value {
		digit := strings.IndexRune(base58Alphabet, ch)
		if digit < 0 {
			return nil, false
		}
		n.Mul(n, base)
		n.Add(n, big.NewInt(int64(digit)))
	}
	leadingZeroes := len(value) - len(strings.TrimLeft(value, "1"))
	decoded := make([]byte, leadingZeroes+len(n.Bytes()))
	copy(decoded[leadingZeroes:], n.Bytes())
	return decoded, true
}

func bitcoinSegWitMainnetAddress(value string) (string, byte, bool) {
	if len(value) < 14 || len(value) > 90 {
		return "", 0, false
	}
	if value != strings.ToLower(value) && value != strings.ToUpper(value) {
		return "", 0, false
	}
	canonical := strings.ToLower(value)
	separator := strings.LastIndexByte(canonical, '1')
	if separator != 2 || canonical[:separator] != "bc" || separator+7 > len(canonical) {
		return "", 0, false
	}
	data, ok := decodeBech32Data(canonical[separator+1:])
	if !ok || len(data) < 7 {
		return "", 0, false
	}
	checksumType := bech32VerifyChecksum("bc", data)
	if checksumType == 0 {
		return "", 0, false
	}
	payload := data[:len(data)-6]
	if len(payload) < 2 || payload[0] > 16 {
		return "", 0, false
	}
	witnessVersion := payload[0]
	program, ok := convertBits(payload[1:], 5, 8, false)
	if !ok || len(program) < 2 || len(program) > 40 {
		return "", 0, false
	}
	if witnessVersion == 0 {
		if checksumType != bech32ChecksumConst || (len(program) != 20 && len(program) != 32) {
			return "", 0, false
		}
	} else if checksumType != bech32mChecksumConst {
		return "", 0, false
	}
	return canonical, witnessVersion, true
}

func decodeBech32Data(value string) ([]byte, bool) {
	out := make([]byte, len(value))
	for index, ch := range value {
		position := strings.IndexRune(bech32Charset, ch)
		if position < 0 {
			return nil, false
		}
		out[index] = byte(position)
	}
	return out, true
}

func bech32VerifyChecksum(hrp string, data []byte) uint32 {
	polymod := bech32Polymod(append(bech32HRPExpand(hrp), data...))
	switch polymod {
	case bech32ChecksumConst:
		return bech32ChecksumConst
	case bech32mChecksumConst:
		return bech32mChecksumConst
	default:
		return 0
	}
}

func bech32HRPExpand(hrp string) []byte {
	expanded := make([]byte, 0, len(hrp)*2+1)
	for _, ch := range hrp {
		expanded = append(expanded, byte(ch>>5))
	}
	expanded = append(expanded, 0)
	for _, ch := range hrp {
		expanded = append(expanded, byte(ch&31))
	}
	return expanded
}

func bech32Polymod(values []byte) uint32 {
	generators := [5]uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	checksum := uint32(1)
	for _, value := range values {
		top := checksum >> 25
		checksum = (checksum&0x1ffffff)<<5 ^ uint32(value)
		for index, generator := range generators {
			if (top>>index)&1 != 0 {
				checksum ^= generator
			}
		}
	}
	return checksum
}

func convertBits(data []byte, fromBits, toBits uint, pad bool) ([]byte, bool) {
	var accumulator uint32
	var bits uint
	maxValue := uint32((1 << toBits) - 1)
	maxAccumulator := uint32((1 << (fromBits + toBits - 1)) - 1)
	out := make([]byte, 0, len(data)*int(fromBits)/int(toBits)+1)
	for _, value := range data {
		if uint32(value)>>fromBits != 0 {
			return nil, false
		}
		accumulator = ((accumulator << fromBits) | uint32(value)) & maxAccumulator
		bits += fromBits
		for bits >= toBits {
			bits -= toBits
			out = append(out, byte((accumulator>>bits)&maxValue))
		}
	}
	if pad {
		if bits > 0 {
			out = append(out, byte((accumulator<<(toBits-bits))&maxValue))
		}
	} else if bits >= fromBits || ((accumulator<<(toBits-bits))&maxValue) != 0 {
		return nil, false
	}
	return out, true
}
